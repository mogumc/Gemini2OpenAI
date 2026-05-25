package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/mogumc/gemini2openai/auth"
	"github.com/mogumc/gemini2openai/client"
	"github.com/mogumc/gemini2openai/config"
	"github.com/mogumc/gemini2openai/handler"
)

func main() {
	cfg := config.Load()

	// Validate required config
	if cfg.GeminiAPIKey == "" {
		log.Fatal("GEMINI_API_KEY is required")
	}

	// Create Gemini client
	geminiClient := client.New(cfg.GeminiAPIKey, cfg.GeminiBaseURL)

	// Create chat handler
	chatHandler := handler.NewChatHandler(geminiClient, cfg)

	// Setup routes with auth middleware
	mux := http.NewServeMux()

	// OpenAI-compatible endpoints
	mux.Handle("/v1/chat/completions", auth.Middleware(cfg.ProxyAPIKey)(chatHandler))

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// Models endpoint
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"object":"list","data":[{"id":"%s","object":"model","created":0,"owned_by":"google"}]}`, cfg.GeminiModel)
	})

	addr := ":" + cfg.Port
	log.Printf("Gemini2OpenAI proxy starting on %s", addr)
	log.Printf("Gemini base URL: %s", cfg.GeminiBaseURL)
	log.Printf("Gemini model: %s", cfg.GeminiModel)
	if cfg.ProxyAPIKey != "" {
		log.Printf("Proxy auth: enabled")
	} else {
		log.Printf("Proxy auth: disabled (no PROXY_API_KEY set)")
	}

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
