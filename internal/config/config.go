package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Action string

const (
	ActionHibernate Action = "hibernate" // save state to disk, stop VM (Proxmox UI: Hibernate)
	ActionPause     Action = "pause"     // freeze in RAM, fast (Proxmox UI: Pause)
	ActionStop      Action = "stop"      // graceful shutdown
	ActionReset     Action = "reset"     // hard reset
	ActionRandom    Action = "random"    // randomly picks one each time
)

// Duration wraps time.Duration to support YAML parsing of strings like "5m", "1h".
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	dur, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value.Value, err)
	}
	d.Duration = dur
	return nil
}

type Target struct {
	Node string `yaml:"node" json:"node"`
	VMID int    `yaml:"vmid" json:"vmid"`
	Name string `yaml:"name" json:"name"` // optional label for logging
}

type ProxmoxConfig struct {
	URL         string `yaml:"url"`
	TokenID     string `yaml:"token_id"`
	TokenSecret string `yaml:"token_secret"`
	InsecureTLS bool   `yaml:"insecure_tls"`
}

type ChaosConfig struct {
	Action      Action   `yaml:"action"`
	ResumeAfter Duration `yaml:"resume_after"`
	Interval    Duration `yaml:"interval"`
	Jitter      int      `yaml:"jitter"` // percentage 0–100
	DryRun      bool     `yaml:"dry_run"`
}

type Config struct {
	Proxmox ProxmoxConfig `yaml:"proxmox"`
	Targets []Target      `yaml:"targets"`
	Chaos   ChaosConfig   `yaml:"chaos"`
}

// Load reads config from an optional YAML file, then applies env var overrides.
// If path is empty, only env vars are used.
func Load(path string) (*Config, error) {
	var cfg Config

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
		data = []byte(os.ExpandEnv(string(data)))
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	if err := applyEnv(&cfg); err != nil {
		return nil, err
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// applyEnv overrides config fields from environment variables.
// Env vars always take precedence over the config file.
//
// Supported variables:
//
//	PROXMOX_URL, PROXMOX_TOKEN_ID, PROXMOX_TOKEN_SECRET, PROXMOX_INSECURE_TLS
//	CHAOS_ACTION, CHAOS_RESUME_AFTER, CHAOS_INTERVAL, CHAOS_JITTER, CHAOS_DRY_RUN
//	CHAOS_TARGETS  (JSON array: [{"node":"pve1","vmid":101,"name":"web-1"}])
func applyEnv(cfg *Config) error {
	if v := os.Getenv("PROXMOX_URL"); v != "" {
		cfg.Proxmox.URL = v
	}
	if v := os.Getenv("PROXMOX_TOKEN_ID"); v != "" {
		cfg.Proxmox.TokenID = v
	}
	if v := os.Getenv("PROXMOX_TOKEN_SECRET"); v != "" {
		cfg.Proxmox.TokenSecret = v
	}
	if v := os.Getenv("PROXMOX_INSECURE_TLS"); v != "" {
		cfg.Proxmox.InsecureTLS = v == "true" || v == "1"
	}

	if v := os.Getenv("CHAOS_ACTION"); v != "" {
		cfg.Chaos.Action = Action(v)
	}
	if v := os.Getenv("CHAOS_RESUME_AFTER"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("CHAOS_RESUME_AFTER: %w", err)
		}
		cfg.Chaos.ResumeAfter = Duration{d}
	}
	if v := os.Getenv("CHAOS_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("CHAOS_INTERVAL: %w", err)
		}
		cfg.Chaos.Interval = Duration{d}
	}
	if v := os.Getenv("CHAOS_JITTER"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("CHAOS_JITTER: %w", err)
		}
		cfg.Chaos.Jitter = n
	}
	if v := os.Getenv("CHAOS_DRY_RUN"); v != "" {
		cfg.Chaos.DryRun = v == "true" || v == "1"
	}
	if v := os.Getenv("CHAOS_TARGETS"); v != "" {
		var targets []Target
		if err := json.Unmarshal([]byte(v), &targets); err != nil {
			return fmt.Errorf("CHAOS_TARGETS: %w", err)
		}
		cfg.Targets = targets
	}

	return nil
}

func (c *Config) validate() error {
	if c.Proxmox.URL == "" {
		return fmt.Errorf("proxmox url is required (PROXMOX_URL or proxmox.url)")
	}
	if c.Proxmox.TokenID == "" || c.Proxmox.TokenSecret == "" {
		return fmt.Errorf("proxmox token is required (PROXMOX_TOKEN_ID / PROXMOX_TOKEN_SECRET)")
	}
	if len(c.Targets) == 0 {
		return fmt.Errorf("at least one target is required (CHAOS_TARGETS or targets)")
	}
	for i, t := range c.Targets {
		if t.Node == "" {
			return fmt.Errorf("targets[%d].node is required", i)
		}
		if t.VMID <= 0 {
			return fmt.Errorf("targets[%d].vmid must be positive", i)
		}
	}

	if c.Chaos.Action == "" {
		c.Chaos.Action = ActionHibernate
	}
	switch c.Chaos.Action {
	case ActionHibernate, ActionPause, ActionStop, ActionReset, ActionRandom:
	default:
		return fmt.Errorf("chaos.action must be one of: hibernate, pause, stop, reset, random")
	}

	if c.Chaos.Interval.Duration <= 0 {
		return fmt.Errorf("chaos interval must be positive (CHAOS_INTERVAL or chaos.interval)")
	}
	if (c.Chaos.Action == ActionHibernate || c.Chaos.Action == ActionPause) && c.Chaos.ResumeAfter.Duration <= 0 {
		c.Chaos.ResumeAfter = Duration{5 * time.Minute}
	}
	if c.Chaos.Jitter < 0 || c.Chaos.Jitter > 100 {
		return fmt.Errorf("chaos.jitter must be between 0 and 100")
	}

	return nil
}
