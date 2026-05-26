package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mogumc/gemini2openai/auth"
	"github.com/mogumc/gemini2openai/client"
	"github.com/mogumc/gemini2openai/config"
	"github.com/mogumc/gemini2openai/handler"
)

func main() {
	cfg := config.Load()

	if cfg.GeminiAPIKey == "" {
		log.Fatal("GEMINI_API_KEY is required")
	}

	geminiClient := client.New(cfg.GeminiAPIKey, cfg.GeminiBaseURL)
	chatHandler := handler.NewChatHandler(geminiClient, cfg)

	mux := http.NewServeMux()
	mux.Handle("/v1/chat/completions", auth.Middleware(cfg.ProxyAPIKey)(chatHandler))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"id":        cfg.GeminiModel,
					"object":    "model",
					"created":   0,
					"owned_by":  "google",
				},
			},
		})
	})

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Gemini2OpenAI proxy starting on %s", addr)
		log.Printf("Gemini base URL: %s", cfg.GeminiBaseURL)
		log.Printf("Gemini model: %s", cfg.GeminiModel)
		if cfg.ProxyAPIKey != "" {
			log.Printf("Proxy auth: enabled")
		} else {
			log.Printf("Proxy auth: disabled (no PROXY_API_KEY set)")
		}

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, shutting down gracefully...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
