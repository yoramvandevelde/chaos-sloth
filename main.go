package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/yoramvandevelde/chaos-sloth/internal/chaos"
	"github.com/yoramvandevelde/chaos-sloth/internal/config"
	"github.com/yoramvandevelde/chaos-sloth/internal/proxmox"
)

func main() {
	configPath := flag.String("config", "", "path to config file (optional, all settings can also be set via env vars)")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	client := proxmox.New(cfg.Proxmox.URL, cfg.Proxmox.TokenID, cfg.Proxmox.TokenSecret, cfg.Proxmox.InsecureTLS)
	sloth := chaos.New(cfg, client)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := sloth.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
