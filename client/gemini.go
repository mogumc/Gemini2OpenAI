package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mogumc/gemini2openai/types"
)

// Client is the Gemini API client.
type Client struct {
	apiKey       string
	baseURL      string
	httpClient   *http.Client
	streamClient *http.Client
}

// New creates a new Gemini client.
func New(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // generous timeout for non-streaming (thinking models may take long)
		},
		streamClient: &http.Client{
			Timeout: 0, // no timeout for streaming
		},
	}
}

// GenerateContent sends a non-streaming request to Gemini.
func (c *Client) GenerateContent(model string, req *types.GeminiRequest) (*types.GeminiResponse, error) {
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, model, c.apiKey)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		
		// Parse error response for structured error message
		var errorResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errorResp); err == nil && errorResp.Error.Message != "" {
			// Return structured error without sensitive info
			return nil, fmt.Errorf("gemini api error (%d): %s", resp.StatusCode, errorResp.Error.Message)
		}
		
		return nil, fmt.Errorf("gemini api error (%d)", resp.StatusCode)
	}

	var geminiResp types.GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Debug output for successful response
	respJSON, _ := json.Marshal(geminiResp)
	fmt.Printf("[DEBUG] Gemini Response Length: %d bytes\n", len(respJSON))

	// Check for candidates with errors
	for i, candidate := range geminiResp.Candidates {
		if candidate.FinishReason != "" {
			fmt.Printf("[DEBUG] Candidate %d - FinishReason: %s\n", i, candidate.FinishReason)
		}
		if len(candidate.SafetyRatings) > 0 {
			fmt.Printf("[DEBUG] Candidate %d - SafetyRatings count: %d\n", i, len(candidate.SafetyRatings))
		}
	}

	return &geminiResp, nil
}

// StreamGenerateContent sends a streaming request to Gemini.
// The caller should pass the request context so the goroutine can detect client disconnection.
func (c *Client) StreamGenerateContent(ctx context.Context, model string, req *types.GeminiRequest) (<-chan *types.GeminiStreamChunk, <-chan error) {
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", c.baseURL, model, c.apiKey)

	body, err := json.Marshal(req)
	if err != nil {
		errCh := make(chan error, 1)
		errCh <- fmt.Errorf("marshal request: %w", err)
		close(errCh)
		return nil, errCh
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		errCh := make(chan error, 1)
		errCh <- fmt.Errorf("create request: %w", err)
		close(errCh)
		return nil, errCh
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.streamClient.Do(httpReq)
	if err != nil {
		errCh := make(chan error, 1)
		errCh <- fmt.Errorf("request gemini: %w", err)
		close(errCh)
		return nil, errCh
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		errCh := make(chan error, 1)
		errCh <- fmt.Errorf("gemini api error (%d): %s", resp.StatusCode, string(respBody))
		close(errCh)
		return nil, errCh
	}

	chunkCh := make(chan *types.GeminiStreamChunk, 100)
	errCh := make(chan error, 10)

	go func() {
		defer resp.Body.Close()
		defer close(chunkCh)
		defer close(errCh)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			if data == "" || data == "[DONE]" {
				if data == "[DONE]" {
					break
				}
				continue
			}

			var chunk types.GeminiStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				select {
				case errCh <- fmt.Errorf("decode chunk: %w", err):
				default:
				}
				continue
			}

			select {
			case chunkCh <- &chunk:
			case <-ctx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil {
			select {
			case errCh <- fmt.Errorf("read stream: %w", err):
			default:
			}
		}
	}()

	return chunkCh, errCh
}
