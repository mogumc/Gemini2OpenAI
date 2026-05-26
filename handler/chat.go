package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mogumc/gemini2openai/client"
	"github.com/mogumc/gemini2openai/config"
	"github.com/mogumc/gemini2openai/mapper"
	"github.com/mogumc/gemini2openai/types"
)

const MaxRequestBodySize = 10 << 20

type ChatHandler struct {
	client *client.Client
	cfg    *config.Config
}

func NewChatHandler(client *client.Client, cfg *config.Config) *ChatHandler {
	return &ChatHandler{client: client, cfg: cfg}
}

func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

	var req types.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	model := req.Model
	if model == "" {
		model = h.cfg.GeminiModel
	} else if len(h.cfg.AllowedModels) > 0 {
		found := false
		for _, m := range h.cfg.AllowedModels {
			if m == model {
				found = true
				break
			}
		}
		if !found {
			model = h.cfg.GeminiModel
		}
	}

	requestID := fmt.Sprintf("req_%d_%s", time.Now().UnixNano(), mapper.GenerateRandomID())

	geminiReq := mapper.MapRequest(requestID, &req, h.cfg)

	if req.Stream {
		h.handleStream(w, r, requestID, geminiReq, model)
		return
	}

	h.handleSync(w, r, requestID, geminiReq, model)
}

func (h *ChatHandler) handleSync(w http.ResponseWriter, r *http.Request, requestID string, geminiReq *types.GeminiRequest, model string) {
	createdAt := time.Now().Unix()

	resp, err := h.client.GenerateContent(model, geminiReq)
	if err != nil {
		fmt.Printf("[ERROR] %s Gemini request failed - RequestID: %s, Error: %v\n", time.Now().Format("2006/01/02 15:04:05"), requestID, err)
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		mapper.CleanupRequestCache(requestID)
		return
	}

	openaiResp, hasToolCalls := mapper.MapResponse(requestID, resp, model, createdAt)
	if !hasToolCalls {
		mapper.CleanupRequestCache(requestID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(openaiResp)
}

func (h *ChatHandler) handleStream(w http.ResponseWriter, r *http.Request, requestID string, geminiReq *types.GeminiRequest, model string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		mapper.CleanupRequestCache(requestID)
		return
	}

	chunkCh, errCh := h.client.StreamGenerateContent(r.Context(), model, geminiReq)

	if chunkCh == nil {
		if errCh != nil {
			if err, ok := <-errCh; ok && err != nil {
				fmt.Printf("[ERROR] %s Stream request failed - RequestID: %s, Error: %v\n", time.Now().Format("2006/01/02 15:04:05"), requestID, err)
				writeSSEError(w, err.Error())
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				mapper.CleanupRequestCache(requestID)
				return
			}
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
		mapper.CleanupRequestCache(requestID)
		return
	}

	createdAt := time.Now().Unix()
	anyToolCalls := false

	headerChunk := &types.StreamChunk{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion.chunk",
		Created: createdAt,
		Model:   model,
		Choices: []types.StreamChoice{
			{
				Index: 0,
				Delta: types.StreamDelta{Role: "assistant"},
			},
		},
	}
	headerData, _ := json.Marshal(headerChunk)
	fmt.Fprintf(w, "data: %s\n\n", headerData)
	flusher.Flush()

	for {
		select {
		case chunk, ok := <-chunkCh:
			if !ok {
				if !anyToolCalls {
					mapper.CleanupRequestCache(requestID)
				}
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}

			streamChunk, hasToolCalls := mapper.MapStreamChunk(requestID, chunk, model, createdAt)
			if hasToolCalls {
				anyToolCalls = true
			}

			if isStreamChunkEmpty(streamChunk) {
				continue
			}

			data, _ := json.Marshal(streamChunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			for _, choice := range streamChunk.Choices {
				if choice.FinishReason != nil {
					if !anyToolCalls {
						mapper.CleanupRequestCache(requestID)
					}
					fmt.Fprintf(w, "data: [DONE]\n\n")
					flusher.Flush()
					return
				}
			}

		case err, ok := <-errCh:
			if ok && err != nil {
				fmt.Printf("[ERROR] %s Stream chunk error - RequestID: %s, Error: %v\n", time.Now().Format("2006/01/02 15:04:05"), requestID, err)
				writeSSEError(w, err.Error())
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				mapper.CleanupRequestCache(requestID)
				return
			}
		}
	}
}

func isStreamChunkEmpty(chunk *types.StreamChunk) bool {
	if len(chunk.Choices) == 0 {
		return true
	}
	for _, choice := range chunk.Choices {
		if choice.Delta.Content != "" || choice.Delta.ReasoningContent != "" || len(choice.Delta.ToolCalls) > 0 || choice.FinishReason != nil {
			return false
		}
	}
	return true
}

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"message": msg,
			"type":    "invalid_request_error",
		},
	})
}

func writeSSEError(w http.ResponseWriter, msg string) {
	displayMsg := "An error occurred while communicating with the AI provider."
	if !containsURL(msg) {
		displayMsg = msg
	}

	errResp := map[string]interface{}{
		"error": map[string]string{
			"message": displayMsg,
			"type":    "internal_error",
		},
	}
	data, _ := json.Marshal(errResp)
	fmt.Fprintf(w, "data: %s\n\n", data)
}

func containsURL(s string) bool {
	for _, prefix := range []string{"http://", "https://", "key="} {
		if len(s) > 0 {
			for i := 0; i <= len(s)-len(prefix); i++ {
				if s[i:i+len(prefix)] == prefix {
					return true
				}
			}
		}
	}
	return false
}
