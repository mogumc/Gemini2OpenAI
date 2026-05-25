package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mogumc/gemini2openai/client"
	"github.com/mogumc/gemini2openai/config"
	"github.com/mogumc/gemini2openai/mapper"
	"github.com/mogumc/gemini2openai/types"
)

// ChatHandler handles chat completion requests.
type ChatHandler struct {
	client *client.Client
	cfg    *config.Config
}

// NewChatHandler creates a new chat handler.
func NewChatHandler(client *client.Client, cfg *config.Config) *ChatHandler {
	return &ChatHandler{
		client: client,
		cfg:    cfg,
	}
}

// ServeHTTP handles POST /v1/chat/completions.
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":{"message":"Method not allowed","type":"invalid_request_error"}}`, http.StatusMethodNotAllowed)
		return
	}

	var req types.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"message":"Invalid request body","type":"invalid_request_error"}}`, http.StatusBadRequest)
		return
	}

	// Determine model
	model := req.Model
	if model == "" {
		model = h.cfg.GeminiModel
	}
	model = mapModelName(model)

	// Map request
	geminiReq := mapper.MapRequest(&req, h.cfg)

	// Handle streaming
	if req.Stream {
		h.handleStream(w, r, geminiReq, model)
		return
	}

	// Non-streaming
	h.handleSync(w, r, geminiReq, model)
}

// mapModelName maps OpenAI model names to Gemini equivalents.
func mapModelName(model string) string {
	if strings.HasPrefix(model, "gemini") {
		return model
	}

	mappings := map[string]string{
		"gpt-4":                  "gemini-2.5-pro",
		"gpt-4-turbo":            "gemini-2.5-pro",
		"gpt-4-turbo-2024-04-09": "gemini-2.5-pro",
		"gpt-4o":                 "gemini-2.5-flash",
		"gpt-4o-mini":            "gemini-2.0-flash",
		"gpt-4o-2024-05-13":      "gemini-2.5-flash",
		"gpt-4o-2024-08-06":      "gemini-2.5-flash",
		"gpt-4o-2024-11-20":      "gemini-2.5-flash",
		"gpt-4o-mini-2024-07-18": "gemini-2.0-flash",
		"gpt-3.5-turbo":          "gemini-2.0-flash",
		"gpt-3.5-turbo-0125":     "gemini-2.0-flash",
		"gpt-3.5-turbo-16k":      "gemini-2.0-flash",
		"o1":                     "gemini-2.5-pro",
		"o1-mini":                "gemini-2.0-flash",
		"o1-preview":             "gemini-2.5-pro",
		"claude-3-5-sonnet":      "gemini-2.5-flash",
		"claude-3-5-haiku":       "gemini-2.0-flash",
		"claude-3-opus":          "gemini-2.5-pro",
		"claude-3-sonnet":        "gemini-2.5-flash",
		"claude-3-haiku":         "gemini-2.0-flash",
	}

	if mapped, ok := mappings[model]; ok {
		return mapped
	}
	return model
}

func (h *ChatHandler) handleSync(w http.ResponseWriter, r *http.Request, geminiReq *types.GeminiRequest, model string) {
	resp, err := h.client.GenerateContent(model, geminiReq)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":{"message":"%s","type":"internal_error"}}`, err.Error()), http.StatusInternalServerError)
		return
	}

	openaiResp := mapper.MapResponse(resp, model)
	openaiResp.Created = time.Now().Unix()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(openaiResp)
}

func (h *ChatHandler) handleStream(w http.ResponseWriter, r *http.Request, geminiReq *types.GeminiRequest, model string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	chunkCh, errCh := h.client.StreamGenerateContent(model, geminiReq)

	// Write stream header
	headerChunk := &types.StreamChunk{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []types.StreamChoice{
			{
				Index: 0,
				Delta: types.StreamDelta{
					Role: "assistant",
				},
			},
		},
	}
	headerData, _ := json.Marshal(headerChunk)
	fmt.Fprintf(w, "data: %s\n\n", headerData)
	flusher.Flush()

	isFirst := true
	for {
		select {
		case chunk, ok := <-chunkCh:
			if !ok {
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}

			streamChunk := mapper.MapStreamChunk(chunk, model, isFirst)
			isFirst = false

			data, _ := json.Marshal(streamChunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			for _, choice := range streamChunk.Choices {
				if choice.FinishReason != nil {
					fmt.Fprintf(w, "data: [DONE]\n\n")
					flusher.Flush()
					return
				}
			}

		case err, ok := <-errCh:
			if ok && err != nil {
				fmt.Fprintf(w, "data: {\"error\":{\"message\":\"%s\"}}\n\n", err.Error())
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}
		}
	}
}
