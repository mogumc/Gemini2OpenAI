package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mogumc/gemini2openai/types"
)

// Client is the Gemini API client.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// New creates a new Gemini client.
func New(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
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
		return nil, fmt.Errorf("gemini api error (%d): %s", resp.StatusCode, string(respBody))
	}

	var geminiResp types.GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &geminiResp, nil
}

// StreamGenerateContent sends a streaming request to Gemini.
func (c *Client) StreamGenerateContent(model string, req *types.GeminiRequest) (<-chan *types.GeminiStreamChunk, <-chan error) {
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", c.baseURL, model, c.apiKey)

	body, err := json.Marshal(req)
	if err != nil {
		errCh := make(chan error, 1)
		errCh <- fmt.Errorf("marshal request: %w", err)
		close(errCh)
		return nil, errCh
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
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
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line buffer

		for scanner.Scan() {
			line := scanner.Text()

			// SSE format: "data: {...}"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk types.GeminiStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				select {
				case errCh <- fmt.Errorf("decode chunk: %w", err):
				default:
				}
				continue
			}

			// Non-blocking write: if chunkCh is full, consumer stopped reading → exit goroutine
			select {
			case chunkCh <- &chunk:
			default:
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
