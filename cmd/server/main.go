package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/hy-shine/claude-proxy-go/internal/config"
	"github.com/hy-shine/claude-proxy-go/internal/handler"
	"github.com/hy-shine/claude-proxy-go/internal/logger"
	"github.com/hy-shine/claude-proxy-go/internal/version"
)

var (
	configPath = flag.String("f", "configs/config.json", "Path to config file")
	showVer    = flag.Bool("v", false, "Show version and exit")
)

func main() {
	flag.Usage = func() {
		fmt.Println("Usage: server [options]")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVer {
		fmt.Println(version.String())
		return
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := logger.Init(logger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
	}); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	logger.Infof("Starting server with config from: %s", *configPath)
	logger.Infof("Configured providers: %d, models: %d, log.level=%s", len(cfg.Providers), len(cfg.EnabledModels()), cfg.Log.Level)

	// Create handler
	h, err := handler.NewHandler(cfg)
	if err != nil {
		logger.Errorf("Failed to create handler: %v", err)
		os.Exit(1)
	}

	// Register routes
	http.HandleFunc("/health", h.HandleHealth)
	http.HandleFunc("/v1/messages", h.HandleMessages)
	http.HandleFunc("/v1/messages/count_tokens", h.HandleCountTokens)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Infof("Server starting on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Errorf("Server stopped with error: %v", err)
		os.Exit(1)
	}
}
