package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/1rgs/claude-code-proxy-go/internal/config"
	"github.com/1rgs/claude-code-proxy-go/internal/handler"
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

	log.Printf("Starting server with config from: %s", *configPath)
	log.Printf("Configured providers: %d", len(cfg.Providers))

	// Create handler
	h, err := handler.NewHandler(cfg)
	if err != nil {
		log.Fatalf("Failed to create handler: %v", err)
	}

	// Register routes
	http.HandleFunc("/", h.HandleRoot)
	http.HandleFunc("/v1/messages", h.HandleMessages)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Server starting on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
