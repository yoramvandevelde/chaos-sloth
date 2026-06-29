package chaos

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/yoramvandevelde/chaos-sloth/internal/config"
	"github.com/yoramvandevelde/chaos-sloth/internal/proxmox"
)

var randomActions = []config.Action{config.ActionHibernate, config.ActionPause, config.ActionStop, config.ActionReset}

type Sloth struct {
	cfg    *config.Config
	client *proxmox.Client
	rng    *rand.Rand
}

func New(cfg *config.Config, client *proxmox.Client) *Sloth {
	return &Sloth{
		cfg:    cfg,
		client: client,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec
	}
}

func (s *Sloth) Run(ctx context.Context) error {
	slog.Info("chaos-sloth started",
		"targets", len(s.cfg.Targets),
		"action", s.cfg.Chaos.Action,
		"interval", s.cfg.Chaos.Interval.Duration,
		"jitter", fmt.Sprintf("%d%%", s.cfg.Chaos.Jitter),
		"dry_run", s.cfg.Chaos.DryRun,
	)

	for {
		wait := s.nextInterval()
		slog.Info("sleeping until next chaos event", "duration", wait)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}

		if err := s.strike(ctx); err != nil {
			slog.Error("chaos event failed", "err", err)
		}
	}
}

func (s *Sloth) nextInterval() time.Duration {
	base := s.cfg.Chaos.Interval.Duration
	if s.cfg.Chaos.Jitter == 0 {
		return base
	}
	maxDelta := float64(base) * float64(s.cfg.Chaos.Jitter) / 100.0
	delta := s.rng.Float64()*2*maxDelta - maxDelta
	return base + time.Duration(delta)
}

func (s *Sloth) pickAction() config.Action {
	if s.cfg.Chaos.Action != config.ActionRandom {
		return s.cfg.Chaos.Action
	}
	return randomActions[s.rng.Intn(len(randomActions))]
}

func (s *Sloth) strike(ctx context.Context) error {
	target := s.cfg.Targets[s.rng.Intn(len(s.cfg.Targets))]
	action := s.pickAction()

	label := target.Name
	if label == "" {
		label = fmt.Sprintf("%s/vm%d", target.Node, target.VMID)
	}

	slog.Info("striking target", "target", label, "action", action, "dry_run", s.cfg.Chaos.DryRun)

	if s.cfg.Chaos.DryRun {
		if action != config.ActionReset {
			slog.Info("[dry-run] would disrupt then recover", "target", label, "action", action, "resume_after", s.cfg.Chaos.ResumeAfter.Duration)
		} else {
			slog.Info("[dry-run] would reset (self-recovering)", "target", label)
		}
		return nil
	}

	switch action {
	case config.ActionHibernate:
		return s.disruptAndRecover(ctx, target, label, s.client.HibernateVM, s.client.ResumeVM)
	case config.ActionPause:
		return s.disruptAndRecover(ctx, target, label, s.client.PauseVM, s.client.ResumeVM)
	case config.ActionStop:
		return s.disruptAndRecover(ctx, target, label, s.client.StopVM, s.client.StartVM)
	case config.ActionReset:
		return s.client.ResetVM(ctx, target.Node, target.VMID)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

type vmAction func(ctx context.Context, node string, vmid int) error

func (s *Sloth) disruptAndRecover(ctx context.Context, target config.Target, label string, disrupt, recover vmAction) error {
	if err := disrupt(ctx, target.Node, target.VMID); err != nil {
		return fmt.Errorf("disrupt %s: %w", label, err)
	}

	resumeAfter := s.cfg.Chaos.ResumeAfter.Duration
	slog.Info("target disrupted", "target", label, "recovering_in", resumeAfter)

	select {
	case <-ctx.Done():
		slog.Warn("shutting down while target is disrupted, attempting recovery", "target", label)
		recoverCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := recover(recoverCtx, target.Node, target.VMID); err != nil {
			slog.Error("failed to recover on shutdown", "target", label, "err", err)
		}
		return ctx.Err()
	case <-time.After(resumeAfter):
	}

	if err := recover(ctx, target.Node, target.VMID); err != nil {
		return fmt.Errorf("recover %s: %w", label, err)
	}

	slog.Info("target recovered", "target", label)
	return nil
}
