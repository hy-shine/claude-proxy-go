package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/1rgs/claude-code-proxy-go/internal/config"
	"github.com/1rgs/claude-code-proxy-go/internal/handler"
	"github.com/1rgs/claude-code-proxy-go/internal/logger"
)

var (
	configPath = flag.String("f", "configs/config.json", "Path to config file")
	help       = flag.Bool("help", false, "Show help message")
)

func main() {
	flag.Parse()

	if *help {
		fmt.Println("Usage: ./server -f <config-file>")
		fmt.Println("")
		fmt.Println("Options:")
		flag.PrintDefaults()
		return
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := logger.Init(cfg.Log.Level); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	logger.Infof("Starting server with config from: %s", *configPath)
	logger.Infof("Configured providers: %d, models: %d, log.level=%s", len(cfg.Providers), len(cfg.EnabledModels()), cfg.Log.Level)

	// Create handler
	h, err := handler.NewHandler(cfg)
	if err != nil {
		logger.Errorf("Failed to create handler: %v", err)
		log.Fatal(err)
	}

	// Register routes
	http.HandleFunc("/", h.HandleRoot)
	http.HandleFunc("/v1/messages", h.HandleMessages)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Infof("Server starting on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Errorf("Server stopped with error: %v", err)
		log.Fatal(err)
	}
}
