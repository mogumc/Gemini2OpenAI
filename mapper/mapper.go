package mapper

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/mogumc/gemini2openai/config"
	"github.com/mogumc/gemini2openai/types"
)

// schemaUnsupportedFields: JSON Schema fields Gemini function calling cannot process.
var schemaUnsupportedFields = map[string]bool{
	"additionalProperties": true,
	"allOf":                true,
	"oneOf":                true,
	"anyOf":                true,
	"not":                  true,
	"$ref":                 true,
	"$defs":                true,
	"if":                   true,
	"then":                 true,
	"else":                 true,
	"patternProperties":    true,
	"dependencies":         true,
	"propertyNames":        true,
}

// Default cache TTL: 30 minutes
const DefaultCacheTTL = 30 * time.Minute

// DummyThoughtSignature is an official placeholder to skip validation
// According to Gemini API docs: when injecting custom function calls without
// real thought_signature (e.g., transferring from other models), use this
// virtual signature to skip the validator
const DummyThoughtSignature = "skip_thought_signature_validator"

// cacheEntry represents a cache entry with expiration time
type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

// RequestCache holds per-request cache data
type RequestCache struct {
	thoughtSignatures map[string]*cacheEntry[string]
	toolCallNames     map[string]*cacheEntry[string]
	createdAt        time.Time
}

// NewRequestCache creates a new request cache with default TTL
func NewRequestCache() *RequestCache {
	return &RequestCache{
		thoughtSignatures: make(map[string]*cacheEntry[string]),
		toolCallNames:     make(map[string]*cacheEntry[string]),
		createdAt:        time.Now(),
	}
}

// IsExpired checks if the cache entry has expired
func (rc *RequestCache) IsExpired() bool {
	return time.Since(rc.createdAt) > DefaultCacheTTL
}

// RequestCacheManager manages request-level caches
type RequestCacheManager struct {
	mu       sync.RWMutex
	caches   map[string]*RequestCache
	stopChan chan struct{}
}

// Global request cache manager
var requestCacheManager = &RequestCacheManager{
	caches:   make(map[string]*RequestCache),
	stopChan: make(chan struct{}),
}

func init() {
	// Start background cleanup goroutine
	go requestCacheManager.cleanupExpiredCaches()
}

// GetOrCreateCache gets or creates a cache for the given request ID
func (m *RequestCacheManager) GetOrCreateCache(requestID string) *RequestCache {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cache, exists := m.caches[requestID]; exists {
		return cache
	}

	cache := NewRequestCache()
	m.caches[requestID] = cache
	return cache
}

// GetCache gets the cache for the given request ID
func (m *RequestCacheManager) GetCache(requestID string) *RequestCache {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.caches[requestID]
}

// RemoveCache removes the cache for the given request ID
func (m *RequestCacheManager) RemoveCache(requestID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.caches, requestID)
}

// cleanupExpiredCaches periodically removes expired caches
func (m *RequestCacheManager) cleanupExpiredCaches() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.removeExpiredCaches()
		case <-m.stopChan:
			return
		}
	}
}

// removeExpiredCaches removes all expired caches
func (m *RequestCacheManager) removeExpiredCaches() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for requestID, cache := range m.caches {
		if cache.IsExpired() {
			delete(m.caches, requestID)
		}
	}
}

// Stop stops the cleanup goroutine
func (m *RequestCacheManager) Stop() {
	close(m.stopChan)
}

// StoreThoughtSignature stores a thought signature for a specific request
func StoreThoughtSignature(requestID, toolCallID, thoughtSignature string) {
	if requestID == "" || toolCallID == "" || thoughtSignature == "" {
		return
	}

	cache := requestCacheManager.GetOrCreateCache(requestID)
	cache.thoughtSignatures[toolCallID] = &cacheEntry[string]{
		value:     thoughtSignature,
		expiresAt: time.Now().Add(DefaultCacheTTL),
	}
}

// GetThoughtSignature retrieves a thought signature for a specific request
func GetThoughtSignature(requestID, toolCallID string) string {
	if requestID == "" {
		return ""
	}

	cache := requestCacheManager.GetCache(requestID)
	if cache == nil {
		return ""
	}

	entry, exists := cache.thoughtSignatures[toolCallID]
	if !exists || time.Now().After(entry.expiresAt) {
		return ""
	}

	return entry.value
}

// StoreToolCallName stores a tool call name mapping for a specific request
func StoreToolCallName(requestID, toolCallID, functionName string) {
	if requestID == "" || toolCallID == "" || functionName == "" {
		return
	}

	cache := requestCacheManager.GetOrCreateCache(requestID)
	cache.toolCallNames[toolCallID] = &cacheEntry[string]{
		value:     functionName,
		expiresAt: time.Now().Add(DefaultCacheTTL),
	}
}

// GetToolCallName retrieves a tool call name for a specific request
func GetToolCallName(requestID, toolCallID string) string {
	if requestID == "" {
		return toolCallID
	}

	cache := requestCacheManager.GetCache(requestID)
	if cache == nil {
		return toolCallID
	}

	entry, exists := cache.toolCallNames[toolCallID]
	if !exists || time.Now().After(entry.expiresAt) {
		return toolCallID
	}

	return entry.value
}

// CleanupRequestCache clears the cache for a specific request
func CleanupRequestCache(requestID string) {
	if requestID == "" {
		return
	}
	requestCacheManager.RemoveCache(requestID)
}

// CleanupAllCaches clears all request caches (for testing or shutdown)
func CleanupAllCaches() {
	requestCacheManager.mu.Lock()
	defer requestCacheManager.mu.Unlock()

	requestCacheManager.caches = make(map[string]*RequestCache)
}

type extractedContent struct {
	texts        []string
	reasoning    []string
	toolCalls    []types.ToolCall
	hasToolCalls bool
}

func extractContentFromParts(requestID string, parts []types.GeminiPart) extractedContent {
	var ec extractedContent
	for _, part := range parts {
		if part.Thought {
			if part.Text != "" {
				ec.reasoning = append(ec.reasoning, part.Text)
			}
			continue
		}
		if part.Text != "" {
			ec.texts = append(ec.texts, part.Text)
		}
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			tc := types.ToolCall{
				ID:   fmt.Sprintf("call_%s", generateID()),
				Type: "function",
				Function: types.FunctionCall{
					Name:      part.FunctionCall.Name,
					Arguments: string(args),
				},
			}
			if part.ThoughtSignature != "" {
				tc.Function.ThoughtSignature = part.ThoughtSignature
				StoreThoughtSignature(requestID, tc.ID, part.ThoughtSignature)
			}
			ec.toolCalls = append(ec.toolCalls, tc)
			ec.hasToolCalls = true
		}
	}
	return ec
}

// MapRequest converts an OpenAI request to Gemini format.
func MapRequest(requestID string, req *types.ChatCompletionRequest, cfg *config.Config) *types.GeminiRequest {
	geminiReq := &types.GeminiRequest{
		Contents: make([]types.GeminiContent, 0),
	}

	var systemParts []types.GeminiPart
	// Explicitly clear nonSystemMessages for each request
	nonSystemMessages := make([]types.ChatMessage, 0)

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			text := extractText(msg.Content)
			if text != "" {
				systemParts = append(systemParts, types.GeminiPart{Text: text})
			}
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	if len(systemParts) > 0 {
		geminiReq.SystemInstruction = &types.GeminiContent{
			Role:  "user",
			Parts: systemParts,
		}
	}

	// Merge consecutive tool messages into a single content for parallel function calls
	// According to Gemini API: parallel function responses should be in the same content
	toolParts := make([]types.GeminiPart, 0)
	
	for _, msg := range nonSystemMessages {
		if msg.Role == "tool" {
			content := convertMessage(requestID, msg)
			toolParts = append(toolParts, content.Parts...)
		} else {
			if len(toolParts) > 0 {
				geminiReq.Contents = append(geminiReq.Contents, types.GeminiContent{
					Role:  "user",
					Parts: toolParts,
				})
				toolParts = nil
			}
			content := convertMessage(requestID, msg)
			geminiReq.Contents = append(geminiReq.Contents, content)
		}
	}
	// Flush any remaining tool parts
	if len(toolParts) > 0 {
		geminiReq.Contents = append(geminiReq.Contents, types.GeminiContent{
			Role:  "user",
			Parts: toolParts,
		})
	}

	if len(req.Tools) > 0 {
		geminiReq.Tools = convertTools(req.Tools)
	}

	genConfig := buildGenConfig(req, cfg)
	if genConfig != nil {
		geminiReq.GenerationConfig = genConfig
	}

	geminiReq.SafetySettings = buildSafetySettings(cfg)

	// Auto-handle thoughtSignature for thinking models
	// According to Gemini API docs: ALL function calls must have thoughtSignature
	// We now use DUMMY_SIGNATURE_FOR_HISTORICAL_MESSAGE for historical messages
	// so we don't need to disable thinking mode anymore

	return geminiReq
}

func extractText(content interface{}) string {
	if content == nil {
		return ""
	}

	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var texts []string
		for _, part := range v {
			if p, ok := part.(map[string]interface{}); ok {
				if t, ok := p["text"].(string); ok {
					texts = append(texts, t)
				}
			}
		}
		return strings.Join(texts, "\n")
	default:
		return fmt.Sprintf("%v", content)
	}
}

func convertMessage(requestID string, msg types.ChatMessage) types.GeminiContent {
	role := msg.Role
	if role == "assistant" {
		role = "model"
	}

	content := types.GeminiContent{
		Role:  role,
		Parts: make([]types.GeminiPart, 0),
	}

	if msg.Role == "tool" {
		text := extractText(msg.Content)
		var response interface{}
		if text != "" {
			// Try to parse as JSON first
			if err := json.Unmarshal([]byte(text), &response); err != nil {
				// If not valid JSON, use the text directly as a string value
				response = text
			}
		} else {
			response = ""
		}
		// Gemini requires function_response.response to be a JSON object, not array/primitive
		if _, ok := response.(map[string]interface{}); !ok {
			response = map[string]interface{}{"result": response}
		}
		// Look up the function name from tool_call_id mapping
		funcName := GetToolCallName(requestID, msg.ToolCallID)

		content.Parts = []types.GeminiPart{
			{
				FunctionResponse: &types.GeminiFunctionResponse{
					Name:     funcName,
					Response: response,
				},
			},
		}
		return content
	}

	text := extractText(msg.Content)
	if text != "" {
		content.Parts = append(content.Parts, types.GeminiPart{Text: text})
	}

	for _, tc := range msg.ToolCalls {
		var args interface{}
		if tc.Function.Arguments != "" {
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
		}
		part := types.GeminiPart{
			FunctionCall: &types.GeminiFunctionCall{
				Name: tc.Function.Name,
				Args: args,
			},
		}
		// Auto-handle thoughtSignature:
		// 1. Use client-provided value if available
		// 2. Fall back to cache (from previous Gemini response)
		// 3. If still empty, use DUMMY signature (required for thinking models)
		if tc.Function.ThoughtSignature != "" {
			part.ThoughtSignature = tc.Function.ThoughtSignature
		} else {
			part.ThoughtSignature = GetThoughtSignature(requestID, tc.ID)
		}

		// According to Gemini API docs: ALL function calls must have thoughtSignature
		// For historical messages without real signature, use DUMMY placeholder
		if part.ThoughtSignature == "" {
			part.ThoughtSignature = DummyThoughtSignature
		}

		// Store tool_call_id → function_name mapping for later use in tool responses
		StoreToolCallName(requestID, tc.ID, tc.Function.Name)
		content.Parts = append(content.Parts, part)
	}

	return content
}

func convertTools(tools []types.Tool) []types.GeminiTool {
	geminiTools := make([]types.GeminiTool, 0, len(tools))

	for _, tool := range tools {
		if tool.Type == "function" {
			cleanParams := cleanSchema(tool.Function.Parameters)
			decl := types.GeminiFunctionDecl{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  cleanParams,
			}
			geminiTools = append(geminiTools, types.GeminiTool{
				FunctionDeclarations: []types.GeminiFunctionDecl{decl},
			})
		}
	}

	return geminiTools
}

// cleanSchema removes JSON Schema fields unsupported by Gemini function calling.
func cleanSchema(schema interface{}) interface{} {
	switch v := schema.(type) {
	case map[string]interface{}:
		cleaned := make(map[string]interface{})
		for key, val := range v {
			if schemaUnsupportedFields[key] {
				continue
			}
			cleaned[key] = cleanSchema(val)
		}
		return cleaned
	case []interface{}:
		cleaned := make([]interface{}, len(v))
		for i, val := range v {
			cleaned[i] = cleanSchema(val)
		}
		return cleaned
	default:
		return schema
	}
}

func buildGenConfig(req *types.ChatCompletionRequest, cfg *config.Config) *types.GeminiGenConfig {
	genConfig := &types.GeminiGenConfig{}

	thinkingBudget := 0
	if req.ThinkingBudget != nil {
		thinkingBudget = *req.ThinkingBudget
	} else if cfg.DefaultThinkingBudget != 0 {
		thinkingBudget = cfg.DefaultThinkingBudget
	}

	if req.Temperature != nil {
		genConfig.Temperature = *req.Temperature
	} else {
		genConfig.Temperature = cfg.DefaultTemperature
	}

	// For thinking models, maxOutputTokens includes both thinking and output tokens
	if req.MaxTokens != nil {
		genConfig.MaxOutputTokens = *req.MaxTokens
	} else {
		genConfig.MaxOutputTokens = cfg.DefaultMaxTokens
	}
	if thinkingBudget != 0 && genConfig.MaxOutputTokens < 16384 {
		genConfig.MaxOutputTokens = 16384
	}

	if req.TopP != nil {
		genConfig.TopP = *req.TopP
	} else {
		genConfig.TopP = cfg.DefaultTopP
	}

	genConfig.TopK = cfg.DefaultTopK

	if len(req.Stop) > 0 {
		genConfig.StopSequences = req.Stop
	}

	if thinkingBudget != 0 {
		genConfig.ThinkingConfig = &types.GeminiThinkingConfig{
			ThinkingBudget: thinkingBudget,
		}
	}

	return genConfig
}

func buildSafetySettings(cfg *config.Config) []types.GeminiSafety {
	threshold := cfg.DefaultSafetySettings
	if threshold == "" {
		threshold = "BLOCK_NONE"
	}

	return []types.GeminiSafety{
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: threshold},
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: threshold},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: threshold},
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: threshold},
	}
}

// MapResponse converts a Gemini response to OpenAI format.
// Returns (response, hasToolCalls) — caller uses hasToolCalls to decide cache cleanup.
func MapResponse(requestID string, resp *types.GeminiResponse, model string, createdAt int64) (*types.ChatCompletionResponse, bool) {
	openaiResp := &types.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%s", generateID()),
		Object:  "chat.completion",
		Created: createdAt,
		Model:   model,
		Choices: make([]types.Choice, 0),
	}

	anyToolCalls := false

	for i, candidate := range resp.Candidates {
		ec := extractContentFromParts(requestID, candidate.Content.Parts)
		if ec.hasToolCalls {
			anyToolCalls = true
		}

		choice := types.Choice{
			Index: i,
			Message: types.ChatMessage{
				Role: "assistant",
			},
		}

		if candidate.FinishReason != "" {
			reason := mapFinishReason(candidate.FinishReason)
			choice.FinishReason = &reason
		}

		if len(ec.texts) > 0 {
			choice.Message.Content = strings.Join(ec.texts, "\n")
		}
		if len(ec.reasoning) > 0 {
			choice.Message.ReasoningContent = strings.Join(ec.reasoning, "\n")
		}
		if len(ec.toolCalls) > 0 {
			choice.Message.ToolCalls = ec.toolCalls
		}

		openaiResp.Choices = append(openaiResp.Choices, choice)
	}

	if resp.UsageMetadata != nil {
		openaiResp.Usage = &types.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}

	return openaiResp, anyToolCalls
}

// MapStreamChunk converts a Gemini stream chunk to OpenAI format.
func MapStreamChunk(requestID string, chunk *types.GeminiStreamChunk, model string, createdAt int64) (*types.StreamChunk, bool) {
	streamChunk := &types.StreamChunk{
		ID:      fmt.Sprintf("chatcmpl-%s", generateID()),
		Object:  "chat.completion.chunk",
		Created: createdAt,
		Model:   model,
		Choices: make([]types.StreamChoice, 0),
	}

	anyToolCalls := false

	for i, candidate := range chunk.Candidates {
		ec := extractContentFromParts(requestID, candidate.Content.Parts)
		if ec.hasToolCalls {
			anyToolCalls = true
		}

		choice := types.StreamChoice{
			Index: i,
			Delta: types.StreamDelta{},
		}

		if len(ec.reasoning) > 0 {
			choice.Delta.ReasoningContent = strings.Join(ec.reasoning, "\n")
		}
		if len(ec.texts) > 0 {
			choice.Delta.Content = strings.Join(ec.texts, "\n")
		}
		if len(ec.toolCalls) > 0 {
			choice.Delta.ToolCalls = ec.toolCalls
		}

		if candidate.FinishReason != "" {
			reason := mapFinishReason(candidate.FinishReason)
			choice.FinishReason = &reason
		}

		streamChunk.Choices = append(streamChunk.Choices, choice)
	}

	return streamChunk, anyToolCalls
}

func mapFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	default:
		return "stop"
	}
}

func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(100000))
}

// GenerateRandomID generates a random ID for request identification
func GenerateRandomID() string {
	return fmt.Sprintf("%d", rand.Intn(1000000))
}
