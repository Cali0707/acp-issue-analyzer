package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cmurray/acp-issue-analyzer/internal/config"
	ghclient "github.com/cmurray/acp-issue-analyzer/internal/github"
	"github.com/cmurray/acp-issue-analyzer/internal/store"
	"github.com/cmurray/acp-issue-analyzer/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Find and load config
	configPath := config.FindConfigPath()
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	if configPath == "" {
		configDir := config.DefaultConfigDir()
		if configDir != "" {
			return fmt.Errorf("no config file found. Create %s/config.toml or pass path as argument", configDir)
		}
		return fmt.Errorf("no config file found. Pass config path as argument")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Set up logging to file (avoid interfering with TUI)
	logPath := cfg.General.DataDir + "/acp-issue-analyzer.log"
	os.MkdirAll(cfg.General.DataDir, 0o755)
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer logFile.Close()
	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// GitHub client
	ctx := context.Background()
	gh, err := ghclient.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	// Session store
	sessionStore, err := store.New(cfg.SessionsDir())
	if err != nil {
		return fmt.Errorf("creating session store: %w", err)
	}

	// Launch TUI
	model := tui.NewModel(cfg, gh, sessionStore, logger)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}
