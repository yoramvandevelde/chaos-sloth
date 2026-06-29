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
		if action == config.ActionHibernate || action == config.ActionPause {
			slog.Info("[dry-run] would pause/hibernate then resume", "target", label, "resume_after", s.cfg.Chaos.ResumeAfter.Duration)
		}
		return nil
	}

	switch action {
	case config.ActionHibernate:
		return s.pauseAndResume(ctx, target, label, s.client.HibernateVM)
	case config.ActionPause:
		return s.pauseAndResume(ctx, target, label, s.client.PauseVM)
	case config.ActionStop:
		return s.client.StopVM(ctx, target.Node, target.VMID)
	case config.ActionReset:
		return s.client.ResetVM(ctx, target.Node, target.VMID)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

type vmAction func(ctx context.Context, node string, vmid int) error

func (s *Sloth) pauseAndResume(ctx context.Context, target config.Target, label string, pause vmAction) error {
	if err := pause(ctx, target.Node, target.VMID); err != nil {
		return fmt.Errorf("pause %s: %w", label, err)
	}

	resumeAfter := s.cfg.Chaos.ResumeAfter.Duration
	slog.Info("target paused", "target", label, "resuming_in", resumeAfter)

	select {
	case <-ctx.Done():
		slog.Warn("shutting down while target is paused, attempting resume", "target", label)
		if err := s.client.ResumeVM(context.Background(), target.Node, target.VMID); err != nil {
			slog.Error("failed to resume on shutdown", "target", label, "err", err)
		}
		return ctx.Err()
	case <-time.After(resumeAfter):
	}

	if err := s.client.ResumeVM(ctx, target.Node, target.VMID); err != nil {
		return fmt.Errorf("resume %s: %w", label, err)
	}

	slog.Info("target resumed", "target", label)
	return nil
}
