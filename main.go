package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jimohabdol/git-pr-watcher/internal/config"
	"github.com/jimohabdol/git-pr-watcher/internal/github"
	"github.com/jimohabdol/git-pr-watcher/internal/logger"
	"github.com/jimohabdol/git-pr-watcher/internal/notifier"
	"github.com/jimohabdol/git-pr-watcher/internal/watcher"
)

func main() {
	var (
		configFile = flag.String("config", "config.yaml", "Path to configuration file")
		watch      = flag.Bool("watch", false, "Run in watch mode (continuous monitoring)")
		interval   = flag.Duration("interval", 1*time.Hour, "Check interval when in watch mode")
		debug      = flag.Bool("debug", false, "Enable debug logging")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
		skipEmails = flag.Bool("skip-emails", false, "Skip sending emails (for testing)")
	)
	flag.Parse()

	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if *debug {
		cfg.Debug.Enabled = true
	}
	if *verbose {
		cfg.Debug.Verbose = true
	}
	if *skipEmails {
		cfg.Debug.SkipEmails = true
	}

	var logLevel logger.LogLevel
	if cfg.Debug.Verbose {
		logLevel = logger.VERBOSE
	} else if cfg.Debug.Enabled {
		logLevel = logger.DEBUG
	} else {
		logLevel = logger.INFO
	}
	logger.Init(logLevel)

	logger.Info("Starting GitHub PR Age Watcher")
	logger.Debug("Configuration loaded from: %s", *configFile)
	if cfg.Debug.SkipEmails {
		logger.Info("Email sending is DISABLED (testing mode)")
	}

	githubClient, err := github.NewClient(cfg.GitHub.Token)
	if err != nil {
		logger.Error("Failed to create GitHub client: %v", err)
		return
	}
	logger.Debug("GitHub client initialized")

	emailNotifier, err := notifier.NewEmailNotifier(cfg.Email, cfg.Debug.SkipEmails)
	if err != nil {
		logger.Error("Failed to create email notifier: %v", err)
		return
	}
	logger.Debug("Email notifier initialized")

	prWatcher := watcher.NewPRWatcher(githubClient, emailNotifier, cfg)
	defer prWatcher.Close()

	if *watch {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		logger.Info("Starting PR watcher in watch mode (interval: %v)", *interval)
		logger.Info("Press Ctrl+C to stop gracefully")

		ticker := time.NewTicker(*interval)
		defer ticker.Stop()

		if err := prWatcher.CheckPRs(); err != nil {
			logger.Error("Error checking PRs: %v", err)
		}

		for {
			select {
			case <-sigChan:
				logger.Info("Received shutdown signal, stopping gracefully...")
				return
			case <-ticker.C:
				if err := prWatcher.CheckPRs(); err != nil {
					logger.Error("Error checking PRs: %v", err)
				}
			}
		}
	} else {
		logger.Info("Running PR watcher once...")
		if err := prWatcher.CheckPRs(); err != nil {
			logger.Error("Error checking PRs: %v", err)
			return
		}
		logger.Info("PR check completed successfully")
	}
}
