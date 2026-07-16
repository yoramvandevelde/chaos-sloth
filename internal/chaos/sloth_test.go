package chaos

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/yoramvandevelde/chaos-sloth/internal/config"
)

// fakeClient implements VMClient for testing.
type fakeClient struct {
	mu       sync.Mutex
	calls    []string
	failOn   map[string]error // method name -> error to return
	recoverF func(node string, vmid int) error
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		calls:  make([]string, 0),
		failOn: make(map[string]error),
	}
}

func (f *fakeClient) record(method string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, method)
}

func (f *fakeClient) shouldFail(method string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failOn[method]
}

func (f *fakeClient) getCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *fakeClient) HibernateVM(_ context.Context, node string, vmid int) error {
	f.record(fmt.Sprintf("HibernateVM:%s/%d", node, vmid))
	return f.shouldFail("HibernateVM")
}

func (f *fakeClient) PauseVM(_ context.Context, node string, vmid int) error {
	f.record(fmt.Sprintf("PauseVM:%s/%d", node, vmid))
	return f.shouldFail("PauseVM")
}

func (f *fakeClient) ResumeVM(_ context.Context, node string, vmid int) error {
	f.record(fmt.Sprintf("ResumeVM:%s/%d", node, vmid))
	return f.shouldFail("ResumeVM")
}

func (f *fakeClient) StopVM(_ context.Context, node string, vmid int) error {
	f.record(fmt.Sprintf("StopVM:%s/%d", node, vmid))
	return f.shouldFail("StopVM")
}

func (f *fakeClient) StartVM(_ context.Context, node string, vmid int) error {
	f.record(fmt.Sprintf("StartVM:%s/%d", node, vmid))
	return f.shouldFail("StartVM")
}

func (f *fakeClient) ResetVM(_ context.Context, node string, vmid int) error {
	f.record(fmt.Sprintf("ResetVM:%s/%d", node, vmid))
	return f.shouldFail("ResetVM")
}

func defaultConfig(action config.Action) *config.Config {
	return &config.Config{
		Targets: []config.Target{
			{Node: "pve1", VMID: 101, Name: "web-1"},
		},
		Chaos: config.ChaosConfig{
			Action:      action,
			ResumeAfter: config.Duration{Duration: 5 * time.Minute},
			Interval:    config.Duration{Duration: 10 * time.Minute},
			Jitter:      0,
			DryRun:      false,
		},
	}
}

func TestPickAction_Fixed(t *testing.T) {
	actions := []config.Action{config.ActionHibernate, config.ActionPause, config.ActionStop, config.ActionReset}
	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			s := &Sloth{cfg: &config.Config{Chaos: config.ChaosConfig{Action: action}}, rng: newRng()}
			for i := 0; i < 10; i++ {
				got := s.pickAction()
				if got != action {
					t.Errorf("pickAction() = %q, want %q", got, action)
				}
			}
		})
	}
}

func TestPickAction_Random(t *testing.T) {
	s := &Sloth{cfg: &config.Config{Chaos: config.ChaosConfig{Action: config.ActionRandom}}, rng: newRng()}
	seen := make(map[config.Action]bool)
	for i := 0; i < 100; i++ {
		seen[s.pickAction()] = true
	}
	for _, a := range randomActions {
		if !seen[a] {
			t.Errorf("random action %q never picked in 100 iterations", a)
		}
	}
}

func TestPickAction_RandomExcludesRandom(t *testing.T) {
	s := &Sloth{cfg: &config.Config{Chaos: config.ChaosConfig{Action: config.ActionRandom}}, rng: newRng()}
	for i := 0; i < 100; i++ {
		got := s.pickAction()
		if got == config.ActionRandom {
			t.Fatal("pickAction() returned 'random' — should never happen")
		}
	}
}

func TestNextInterval_NoJitter(t *testing.T) {
	s := &Sloth{
		cfg: &config.Config{Chaos: config.ChaosConfig{Interval: config.Duration{Duration: 30 * time.Minute}, Jitter: 0}},
		rng: newRng(),
	}
	for i := 0; i < 50; i++ {
		got := s.nextInterval()
		if got != 30*time.Minute {
			t.Errorf("nextInterval() = %v, want 30m", got)
		}
	}
}

func TestNextInterval_WithJitter(t *testing.T) {
	base := 30 * time.Minute
	jitter := 20
	s := &Sloth{
		cfg: &config.Config{Chaos: config.ChaosConfig{Interval: config.Duration{Duration: base}, Jitter: jitter}},
		rng: newRng(),
	}
	min := time.Duration(float64(base) * 0.8)
	max := time.Duration(float64(base) * 1.2)
	for i := 0; i < 200; i++ {
		got := s.nextInterval()
		if got < min || got > max {
			t.Errorf("nextInterval() = %v, want between %v and %v", got, min, max)
		}
	}
}

func TestStrike_DryRun(t *testing.T) {
	client := newFakeClient()
	cfg := defaultConfig(config.ActionHibernate)
	cfg.Chaos.DryRun = true
	s := &Sloth{cfg: cfg, client: client, rng: newRng()}

	if err := s.strike(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.getCalls()) != 0 {
		t.Errorf("dry run should make no API calls, got %d", len(client.getCalls()))
	}
}

func TestStrike_Hibernate(t *testing.T) {
	client := newFakeClient()
	cfg := defaultConfig(config.ActionHibernate)
	cfg.Chaos.ResumeAfter = config.Duration{Duration: 10 * time.Millisecond}
	s := &Sloth{cfg: cfg, client: client, rng: newRng()}

	if err := s.strike(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls := client.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (disrupt + recover), got %d: %v", len(calls), calls)
	}
	if calls[0] != "HibernateVM:pve1/101" {
		t.Errorf("first call = %q, want HibernateVM:pve1/101", calls[0])
	}
	if calls[1] != "ResumeVM:pve1/101" {
		t.Errorf("second call = %q, want ResumeVM:pve1/101", calls[1])
	}
}

func TestStrike_Pause(t *testing.T) {
	client := newFakeClient()
	cfg := defaultConfig(config.ActionPause)
	cfg.Chaos.ResumeAfter = config.Duration{Duration: 10 * time.Millisecond}
	s := &Sloth{cfg: cfg, client: client, rng: newRng()}

	if err := s.strike(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls := client.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %v", len(calls), calls)
	}
	if calls[0] != "PauseVM:pve1/101" {
		t.Errorf("first call = %q, want PauseVM:pve1/101", calls[0])
	}
	if calls[1] != "ResumeVM:pve1/101" {
		t.Errorf("second call = %q, want ResumeVM:pve1/101", calls[1])
	}
}

func TestStrike_Stop(t *testing.T) {
	client := newFakeClient()
	cfg := defaultConfig(config.ActionStop)
	cfg.Chaos.ResumeAfter = config.Duration{Duration: 10 * time.Millisecond}
	s := &Sloth{cfg: cfg, client: client, rng: newRng()}

	if err := s.strike(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls := client.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %v", len(calls), calls)
	}
	if calls[0] != "StopVM:pve1/101" {
		t.Errorf("first call = %q, want StopVM:pve1/101", calls[0])
	}
	if calls[1] != "StartVM:pve1/101" {
		t.Errorf("second call = %q, want StartVM:pve1/101", calls[1])
	}
}

func TestStrike_Reset(t *testing.T) {
	client := newFakeClient()
	cfg := defaultConfig(config.ActionReset)
	s := &Sloth{cfg: cfg, client: client, rng: newRng()}

	if err := s.strike(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls := client.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call (reset only), got %d: %v", len(calls), calls)
	}
	if calls[0] != "ResetVM:pve1/101" {
		t.Errorf("call = %q, want ResetVM:pve1/101", calls[0])
	}
}

func TestStrike_DisruptFails(t *testing.T) {
	client := newFakeClient()
	client.failOn["HibernateVM"] = fmt.Errorf("API down")
	cfg := defaultConfig(config.ActionHibernate)
	s := &Sloth{cfg: cfg, client: client, rng: newRng()}

	err := s.strike(context.Background())
	if err == nil {
		t.Fatal("expected error when disrupt fails, got nil")
	}
	calls := client.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call (failed disrupt only), got %d: %v", len(calls), calls)
	}
}

func TestStrike_RecoverFails(t *testing.T) {
	client := newFakeClient()
	client.failOn["ResumeVM"] = fmt.Errorf("resume broken")
	cfg := defaultConfig(config.ActionHibernate)
	cfg.Chaos.ResumeAfter = config.Duration{Duration: 10 * time.Millisecond}
	s := &Sloth{cfg: cfg, client: client, rng: newRng()}

	err := s.strike(context.Background())
	if err == nil {
		t.Fatal("expected error when recover fails, got nil")
	}
	calls := client.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (disrupt + failed recover), got %d: %v", len(calls), calls)
	}
}

func TestStrike_ContextCancelledDuringDisrupt(t *testing.T) {
	client := newFakeClient()
	cfg := defaultConfig(config.ActionHibernate)
	cfg.Chaos.ResumeAfter = config.Duration{Duration: 1 * time.Hour} // long wait
	s := &Sloth{cfg: cfg, client: client, rng: newRng()}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a brief moment to simulate shutdown during wait
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := s.strike(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	calls := client.getCalls()
	// Should have: HibernateVM + ResumeVM (recovery attempt on shutdown)
	if len(calls) < 2 {
		t.Fatalf("expected at least 2 calls (disrupt + recovery), got %d: %v", len(calls), calls)
	}
	// First call should be the disrupt
	if calls[0] != "HibernateVM:pve1/101" {
		t.Errorf("first call = %q, want HibernateVM:pve1/101", calls[0])
	}
	// Last call should be the recovery attempt
	last := calls[len(calls)-1]
	if last != "ResumeVM:pve1/101" {
		t.Errorf("last call = %q, want ResumeVM:pve1/101", last)
	}
}

func TestRun_ImmediateCancellation(t *testing.T) {
	client := newFakeClient()
	cfg := defaultConfig(config.ActionHibernate)
	cfg.Chaos.Interval = config.Duration{Duration: 10 * time.Millisecond}
	s := New(cfg, client)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := s.Run(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestNew(t *testing.T) {
	client := newFakeClient()
	cfg := defaultConfig(config.ActionStop)
	sloth := New(cfg, client)

	if sloth.cfg != cfg {
		t.Error("cfg not set correctly")
	}
	if sloth.client != client {
		t.Error("client not set correctly")
	}
	if sloth.rng == nil {
		t.Error("rng should not be nil")
	}
}

func TestDisruptAndRecover_NormalFlow(t *testing.T) {
	client := newFakeClient()
	cfg := defaultConfig(config.ActionHibernate)
	cfg.Chaos.ResumeAfter = config.Duration{Duration: 50 * time.Millisecond}
	s := &Sloth{cfg: cfg, client: client, rng: newRng()}

	start := time.Now()
	err := s.disruptAndRecover(
		context.Background(),
		cfg.Targets[0],
		"test-vm",
		client.HibernateVM,
		client.ResumeVM,
	)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("disruptAndRecover returned too fast: %v, expected ~50ms wait", elapsed)
	}
}

// newRng creates a deterministic RNG for testing.
func newRng() *rand.Rand {
	return rand.New(rand.NewSource(42))
}
