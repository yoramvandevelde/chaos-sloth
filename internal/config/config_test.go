package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ValidYAML(t *testing.T) {
	cfg, err := Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Proxmox.URL != "https://proxmox.example.com:8006" {
		t.Errorf("URL = %q, want %q", cfg.Proxmox.URL, "https://proxmox.example.com:8006")
	}
	if cfg.Proxmox.TokenID != "user@pam!chaos-sloth" {
		t.Errorf("TokenID = %q, want %q", cfg.Proxmox.TokenID, "user@pam!chaos-sloth")
	}
	if cfg.Proxmox.TokenSecret != "secret123" {
		t.Errorf("TokenSecret = %q, want %q", cfg.Proxmox.TokenSecret, "secret123")
	}
	if cfg.Proxmox.InsecureTLS {
		t.Error("InsecureTLS should be false")
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("len(Targets) = %d, want 2", len(cfg.Targets))
	}
	if cfg.Targets[0].Node != "pve1" || cfg.Targets[0].VMID != 101 || cfg.Targets[0].Name != "web-1" {
		t.Errorf("Targets[0] = %+v, want {Node:pve1 VMID:101 Name:web-1}", cfg.Targets[0])
	}
	if cfg.Targets[1].Node != "pve2" || cfg.Targets[1].VMID != 201 {
		t.Errorf("Targets[1] = %+v, want {Node:pve2 VMID:201}", cfg.Targets[1])
	}
	if cfg.Chaos.Action != ActionHibernate {
		t.Errorf("Action = %q, want %q", cfg.Chaos.Action, ActionHibernate)
	}
	if cfg.Chaos.ResumeAfter.Duration != 10*time.Minute {
		t.Errorf("ResumeAfter = %v, want 10m", cfg.Chaos.ResumeAfter.Duration)
	}
	if cfg.Chaos.Interval.Duration != 30*time.Minute {
		t.Errorf("Interval = %v, want 30m", cfg.Chaos.Interval.Duration)
	}
	if cfg.Chaos.Jitter != 20 {
		t.Errorf("Jitter = %d, want 20", cfg.Chaos.Jitter)
	}
	if cfg.Chaos.DryRun {
		t.Error("DryRun should be false")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	_, err := Load("testdata/invalid.yaml")
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	_, err := Load("testdata/nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	t.Setenv("PROXMOX_URL", "https://test.example.com")
	t.Setenv("PROXMOX_TOKEN_ID", "test-id")
	t.Setenv("PROXMOX_TOKEN_SECRET", "test-secret")
	t.Setenv("CHAOS_TARGETS", `[{"node":"pve1","vmid":101}]`)
	t.Setenv("CHAOS_INTERVAL", "15m")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Proxmox.URL != "https://test.example.com" {
		t.Errorf("URL = %q, want %q", cfg.Proxmox.URL, "https://test.example.com")
	}
	if len(cfg.Targets) != 1 {
		t.Fatalf("len(Targets) = %d, want 1", len(cfg.Targets))
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("PROXMOX_URL", "https://env.example.com")
	t.Setenv("PROXMOX_TOKEN_ID", "env-id")
	t.Setenv("PROXMOX_TOKEN_SECRET", "env-secret")
	t.Setenv("CHAOS_TARGETS", `[{"node":"env-node","vmid":999,"name":"env-vm"}]`)
	t.Setenv("CHAOS_ACTION", "stop")
	t.Setenv("CHAOS_INTERVAL", "45m")
	t.Setenv("CHAOS_RESUME_AFTER", "3m")
	t.Setenv("CHAOS_JITTER", "10")
	t.Setenv("CHAOS_DRY_RUN", "true")

	cfg, err := Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Proxmox.URL != "https://env.example.com" {
		t.Errorf("URL = %q, want env override", cfg.Proxmox.URL)
	}
	if cfg.Proxmox.TokenID != "env-id" {
		t.Errorf("TokenID = %q, want env override", cfg.Proxmox.TokenID)
	}
	if cfg.Proxmox.TokenSecret != "env-secret" {
		t.Errorf("TokenSecret = %q, want env override", cfg.Proxmox.TokenSecret)
	}
	if cfg.Chaos.Action != ActionStop {
		t.Errorf("Action = %q, want %q", cfg.Chaos.Action, ActionStop)
	}
	if cfg.Chaos.Interval.Duration != 45*time.Minute {
		t.Errorf("Interval = %v, want 45m", cfg.Chaos.Interval.Duration)
	}
	if cfg.Chaos.ResumeAfter.Duration != 3*time.Minute {
		t.Errorf("ResumeAfter = %v, want 3m", cfg.Chaos.ResumeAfter.Duration)
	}
	if cfg.Chaos.Jitter != 10 {
		t.Errorf("Jitter = %d, want 10", cfg.Chaos.Jitter)
	}
	if !cfg.Chaos.DryRun {
		t.Error("DryRun should be true (env override)")
	}
	if len(cfg.Targets) != 1 || cfg.Targets[0].Name != "env-vm" {
		t.Errorf("Targets = %+v, want env-vm override", cfg.Targets)
	}
}

func TestApplyEnv_Booleans(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		expect bool
	}{
		{"true_string", "true", true},
		{"one_string", "1", true},
		{"false_string", "false", false},
		{"zero_string", "0", false},
		{"random_string", "yes", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PROXMOX_INSECURE_TLS", tt.value)
			t.Setenv("CHAOS_DRY_RUN", tt.value)
			var cfg Config
			if err := applyEnv(&cfg); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Proxmox.InsecureTLS != tt.expect {
				t.Errorf("InsecureTLS = %v for %q, want %v", cfg.Proxmox.InsecureTLS, tt.value, tt.expect)
			}
			if cfg.Chaos.DryRun != tt.expect {
				t.Errorf("DryRun = %v for %q, want %v", cfg.Chaos.DryRun, tt.value, tt.expect)
			}
		})
	}
}

func TestApplyEnv_InvalidDuration(t *testing.T) {
	t.Setenv("CHAOS_RESUME_AFTER", "not-a-duration")
	var cfg Config
	if err := applyEnv(&cfg); err == nil {
		t.Fatal("expected error for invalid CHAOS_RESUME_AFTER, got nil")
	}
}

func TestApplyEnv_InvalidJitter(t *testing.T) {
	t.Setenv("CHAOS_JITTER", "not-a-number")
	var cfg Config
	if err := applyEnv(&cfg); err == nil {
		t.Fatal("expected error for invalid CHAOS_JITTER, got nil")
	}
}

func TestApplyEnv_InvalidTargets(t *testing.T) {
	t.Setenv("CHAOS_TARGETS", "not-json")
	var cfg Config
	if err := applyEnv(&cfg); err == nil {
		t.Fatal("expected error for invalid CHAOS_TARGETS, got nil")
	}
}

func TestValidate_MissingURL(t *testing.T) {
	cfg := &Config{
		Proxmox: ProxmoxConfig{TokenID: "id", TokenSecret: "secret"},
		Targets: []Target{{Node: "pve", VMID: 1}},
		Chaos:   ChaosConfig{Action: ActionStop, Interval: Duration{time.Minute}},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for missing URL, got nil")
	}
}

func TestValidate_MissingToken(t *testing.T) {
	cfg := &Config{
		Proxmox: ProxmoxConfig{URL: "https://test.com"},
		Targets: []Target{{Node: "pve", VMID: 1}},
		Chaos:   ChaosConfig{Action: ActionStop, Interval: Duration{time.Minute}},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
}

func TestValidate_NoTargets(t *testing.T) {
	cfg := &Config{
		Proxmox: ProxmoxConfig{URL: "https://test.com", TokenID: "id", TokenSecret: "secret"},
		Targets: []Target{},
		Chaos:   ChaosConfig{Action: ActionStop, Interval: Duration{time.Minute}},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for no targets, got nil")
	}
}

func TestValidate_TargetMissingNode(t *testing.T) {
	cfg := &Config{
		Proxmox: ProxmoxConfig{URL: "https://test.com", TokenID: "id", TokenSecret: "secret"},
		Targets: []Target{{VMID: 1}},
		Chaos:   ChaosConfig{Action: ActionStop, Interval: Duration{time.Minute}},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for target missing node, got nil")
	}
}

func TestValidate_TargetInvalidVMID(t *testing.T) {
	cfg := &Config{
		Proxmox: ProxmoxConfig{URL: "https://test.com", TokenID: "id", TokenSecret: "secret"},
		Targets: []Target{{Node: "pve", VMID: 0}},
		Chaos:   ChaosConfig{Action: ActionStop, Interval: Duration{time.Minute}},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for invalid VMID, got nil")
	}
}

func TestValidate_InvalidAction(t *testing.T) {
	cfg := &Config{
		Proxmox: ProxmoxConfig{URL: "https://test.com", TokenID: "id", TokenSecret: "secret"},
		Targets: []Target{{Node: "pve", VMID: 1}},
		Chaos:   ChaosConfig{Action: "invalid", Interval: Duration{time.Minute}},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for invalid action, got nil")
	}
}

func TestValidate_AllActions(t *testing.T) {
	actions := []Action{ActionHibernate, ActionPause, ActionStop, ActionReset, ActionRandom}
	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			cfg := &Config{
				Proxmox: ProxmoxConfig{URL: "https://test.com", TokenID: "id", TokenSecret: "secret"},
				Targets: []Target{{Node: "pve", VMID: 1}},
				Chaos:   ChaosConfig{Action: action, Interval: Duration{time.Minute}},
			}
			if err := cfg.validate(); err != nil {
				t.Errorf("action %q should be valid, got error: %v", action, err)
			}
		})
	}
}

func TestValidate_DefaultAction(t *testing.T) {
	cfg := &Config{
		Proxmox: ProxmoxConfig{URL: "https://test.com", TokenID: "id", TokenSecret: "secret"},
		Targets: []Target{{Node: "pve", VMID: 1}},
		Chaos:   ChaosConfig{Interval: Duration{time.Minute}},
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Chaos.Action != ActionHibernate {
		t.Errorf("default Action = %q, want %q", cfg.Chaos.Action, ActionHibernate)
	}
}

func TestValidate_NegativeInterval(t *testing.T) {
	cfg := &Config{
		Proxmox: ProxmoxConfig{URL: "https://test.com", TokenID: "id", TokenSecret: "secret"},
		Targets: []Target{{Node: "pve", VMID: 1}},
		Chaos:   ChaosConfig{Action: ActionStop, Interval: Duration{-time.Minute}},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for negative interval, got nil")
	}
}

func TestValidate_ZeroInterval(t *testing.T) {
	cfg := &Config{
		Proxmox: ProxmoxConfig{URL: "https://test.com", TokenID: "id", TokenSecret: "secret"},
		Targets: []Target{{Node: "pve", VMID: 1}},
		Chaos:   ChaosConfig{Action: ActionStop},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for zero interval, got nil")
	}
}

func TestValidate_DefaultResumeAfter(t *testing.T) {
	actions := []Action{ActionHibernate, ActionPause}
	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			cfg := &Config{
				Proxmox: ProxmoxConfig{URL: "https://test.com", TokenID: "id", TokenSecret: "secret"},
				Targets: []Target{{Node: "pve", VMID: 1}},
				Chaos:   ChaosConfig{Action: action, Interval: Duration{time.Minute}},
			}
			if err := cfg.validate(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Chaos.ResumeAfter.Duration != 5*time.Minute {
				t.Errorf("ResumeAfter = %v, want 5m default", cfg.Chaos.ResumeAfter.Duration)
			}
		})
	}
}

func TestValidate_JitterBounds(t *testing.T) {
	tests := []struct {
		name    string
		jitter  int
		wantErr bool
	}{
		{"zero", 0, false},
		{"fifty", 50, false},
		{"hundred", 100, false},
		{"negative", -1, true},
		{"over_hundred", 101, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Proxmox: ProxmoxConfig{URL: "https://test.com", TokenID: "id", TokenSecret: "secret"},
				Targets: []Target{{Node: "pve", VMID: 1}},
				Chaos:   ChaosConfig{Action: ActionStop, Interval: Duration{time.Minute}, Jitter: tt.jitter},
			}
			err := cfg.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("jitter=%d, wantErr=%v, got err=%v", tt.jitter, tt.wantErr, err)
			}
		})
	}
}

func TestLoad_YAMLExpansion(t *testing.T) {
	content := `
proxmox:
  url: "https://${TEST_CHAOS_HOST}:8006"
  token_id: "user@pam!test"
  token_secret: "s3cret"
targets:
  - node: "pve1"
    vmid: 101
chaos:
  action: stop
  interval: "10m"
`
	path := filepath.Join(t.TempDir(), "expand.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	t.Setenv("TEST_CHAOS_HOST", "my-proxmox.local")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Proxmox.URL != "https://my-proxmox.local:8006" {
		t.Errorf("URL = %q, expected env expansion", cfg.Proxmox.URL)
	}
}

func TestDuration_UnmarshalYAML(t *testing.T) {
	// Test via full config parse since Duration is embedded
	content := `
proxmox:
  url: "https://test.com"
  token_id: "id"
  token_secret: "secret"
targets:
  - node: "pve"
    vmid: 1
chaos:
  action: stop
  interval: "1h30m"
  resume_after: "45s"
`
	path := filepath.Join(t.TempDir(), "duration.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Chaos.Interval.Duration != 90*time.Minute {
		t.Errorf("Interval = %v, want 1h30m", cfg.Chaos.Interval.Duration)
	}
	if cfg.Chaos.ResumeAfter.Duration != 45*time.Second {
		t.Errorf("ResumeAfter = %v, want 45s", cfg.Chaos.ResumeAfter.Duration)
	}
}

func TestLoad_MissingFields(t *testing.T) {
	_, err := Load("testdata/missing_fields.yaml")
	if err == nil {
		t.Fatal("expected error for missing fields, got nil")
	}
}
