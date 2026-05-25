package mapper

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mogumc/gemini2openai/config"
	"github.com/mogumc/gemini2openai/types"
)

// MapRequest converts an OpenAI request to Gemini format.
func MapRequest(req *types.ChatCompletionRequest, cfg *config.Config) *types.GeminiRequest {
	geminiReq := &types.GeminiRequest{
		Contents: make([]types.GeminiContent, 0),
	}

	// Extract system message if present
	var systemParts []types.GeminiPart
	var nonSystemMessages []types.ChatMessage

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

	// Set system instruction
	if len(systemParts) > 0 {
		geminiReq.SystemInstruction = &types.GeminiContent{
			Role:  "user",
			Parts: systemParts,
		}
	}

	// Convert messages to contents
	for _, msg := range nonSystemMessages {
		content := convertMessage(msg)
		geminiReq.Contents = append(geminiReq.Contents, content)
	}

	// Convert tools
	if len(req.Tools) > 0 {
		geminiReq.Tools = convertTools(req.Tools)
	}

	// Build generation config
	genConfig := buildGenConfig(req, cfg)
	if genConfig != nil {
		geminiReq.GenerationConfig = genConfig
	}

	// Build safety settings
	geminiReq.SafetySettings = buildSafetySettings(cfg)

	return geminiReq
}

// extractText extracts text from message content.
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

// convertMessage converts a single OpenAI message to Gemini format.
func convertMessage(msg types.ChatMessage) types.GeminiContent {
	role := msg.Role
	if role == "assistant" {
		role = "model"
	}

	content := types.GeminiContent{
		Role:  role,
		Parts: make([]types.GeminiPart, 0),
	}

	// Handle text content
	text := extractText(msg.Content)
	if text != "" {
		content.Parts = append(content.Parts, types.GeminiPart{Text: text})
	}

	// Handle tool calls (for assistant messages)
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
		// Preserve thoughtSignature from OpenAI format (Gemini extension)
		if tc.Function.ThoughtSignature != "" {
			part.ThoughtSignature = tc.Function.ThoughtSignature
		}
		content.Parts = append(content.Parts, part)
	}

	// Handle tool role messages (function responses)
	if msg.Role == "tool" {
		var response interface{}
		if text != "" {
			json.Unmarshal([]byte(text), &response)
		}
		content.Parts = []types.GeminiPart{
			{
				FunctionResponse: &types.GeminiFunctionResponse{
					Name:     msg.ToolCallID,
					Response: response,
				},
			},
		}
	}

	return content
}

// convertTools converts OpenAI tools to Gemini format.
func convertTools(tools []types.Tool) []types.GeminiTool {
	geminiTools := make([]types.GeminiTool, 0, len(tools))

	for _, tool := range tools {
		if tool.Type == "function" {
			// Clean parameters: Gemini doesn't support additionalProperties
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

// cleanSchema recursively removes fields unsupported by Gemini API's function calling Schema.
// Gemini function calling only supports: type, properties, required, description, enum, items, nullable.
// Fields like additionalProperties, allOf/oneOf/anyOf, format, $ref etc. must be stripped.
// Semantic impact is minimal: required still enforces mandatory properties, and LLMs don't
// generate extra properties beyond the schema anyway.
func cleanSchema(schema interface{}) interface{} {
	// Fields that Gemini function calling Schema does not support
	unsupported := map[string]bool{
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
		"format":               true,
		"minimum":              true,
		"maximum":              true,
		"exclusiveMinimum":     true,
		"exclusiveMaximum":     true,
		"minLength":            true,
		"maxLength":            true,
		"pattern":              true,
		"minItems":             true,
		"maxItems":             true,
		"uniqueItems":          true,
		"minProperties":        true,
		"maxProperties":        true,
		"multipleOf":           true,
		"patternProperties":    true,
		"dependencies":         true,
		"propertyNames":        true,
	}

	switch v := schema.(type) {
	case map[string]interface{}:
		cleaned := make(map[string]interface{})
		for key, val := range v {
			if unsupported[key] {
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

// buildGenConfig builds Gemini generation config from OpenAI request.
func buildGenConfig(req *types.ChatCompletionRequest, cfg *config.Config) *types.GeminiGenConfig {
	genConfig := &types.GeminiGenConfig{}

	// Temperature
	if req.Temperature != nil {
		genConfig.Temperature = *req.Temperature
	} else {
		genConfig.Temperature = cfg.DefaultTemperature
	}

	// Max tokens
	if req.MaxTokens != nil {
		genConfig.MaxOutputTokens = *req.MaxTokens
	} else {
		genConfig.MaxOutputTokens = cfg.DefaultMaxTokens
	}

	// TopP
	if req.TopP != nil {
		genConfig.TopP = *req.TopP
	} else {
		genConfig.TopP = cfg.DefaultTopP
	}

	// TopK (OpenAI doesn't have this, use default)
	genConfig.TopK = cfg.DefaultTopK

	// Stop sequences
	if len(req.Stop) > 0 {
		genConfig.StopSequences = req.Stop
	}

	// Thinking budget: request-level overrides config-level
	if req.ThinkingBudget != nil {
		genConfig.ThinkingConfig = &types.GeminiThinkingConfig{
			ThinkingBudget: *req.ThinkingBudget,
		}
	} else if cfg.DefaultThinkingBudget != 0 {
		genConfig.ThinkingConfig = &types.GeminiThinkingConfig{
			ThinkingBudget: cfg.DefaultThinkingBudget,
		}
	}

	return genConfig
}

// buildSafetySettings builds safety settings from config.
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
func MapResponse(resp *types.GeminiResponse, model string) *types.ChatCompletionResponse {
	openaiResp := &types.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%s", generateID()),
		Object:  "chat.completion",
		Created: int64(1000000000), // placeholder
		Model:   model,
		Choices: make([]types.Choice, 0),
	}

	// Map candidates to choices
	for i, candidate := range resp.Candidates {
		choice := types.Choice{
			Index: i,
			Message: types.ChatMessage{
				Role: "assistant",
			},
			FinishReason: mapFinishReason(candidate.FinishReason),
		}

		// Extract text from parts
		var texts []string
		var toolCalls []types.ToolCall

		for _, part := range candidate.Content.Parts {
			// Skip thought parts - they are internal reasoning, not user-visible content
			if part.Thought {
				continue
			}
			if part.Text != "" {
				texts = append(texts, part.Text)
			}
			if part.FunctionCall != nil {
				args, _ := json.Marshal(part.FunctionCall.Args)
				toolCall := types.ToolCall{
					ID:   fmt.Sprintf("call_%s", generateID()),
					Type: "function",
					Function: types.FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(args),
					},
				}
				// Preserve thoughtSignature from Gemini response (required for thinking models)
				if part.ThoughtSignature != "" {
					toolCall.Function.ThoughtSignature = part.ThoughtSignature
				}
				toolCalls = append(toolCalls, toolCall)
			}
		}

		if len(texts) > 0 {
			choice.Message.Content = strings.Join(texts, "\n")
		}
		if len(toolCalls) > 0 {
			choice.Message.ToolCalls = toolCalls
		}

		openaiResp.Choices = append(openaiResp.Choices, choice)
	}

	// Map usage
	if resp.UsageMetadata != nil {
		openaiResp.Usage = &types.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}

	return openaiResp
}

// MapStreamChunk converts a Gemini stream chunk to OpenAI format.
func MapStreamChunk(chunk *types.GeminiStreamChunk, model string, isFirst bool) *types.StreamChunk {
	streamChunk := &types.StreamChunk{
		ID:      fmt.Sprintf("chatcmpl-%s", generateID()),
		Object:  "chat.completion.chunk",
		Created: int64(1000000000),
		Model:   model,
		Choices: make([]types.StreamChoice, 0),
	}

	for _, candidate := range chunk.Candidates {
		choice := types.StreamChoice{
			Index: 0,
			Delta: types.StreamDelta{},
		}

		// First chunk includes role
		if isFirst {
			choice.Delta.Role = "assistant"
		}

		// Extract content
		for _, part := range candidate.Content.Parts {
			// Skip thought parts
			if part.Thought {
				continue
			}
			if part.Text != "" {
				choice.Delta.Content = part.Text
			}
			if part.FunctionCall != nil {
				args, _ := json.Marshal(part.FunctionCall.Args)
				toolCall := types.ToolCall{
					ID:   fmt.Sprintf("call_%s", generateID()),
					Type: "function",
					Function: types.FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(args),
					},
				}
				// Preserve thoughtSignature from Gemini stream (required for thinking models)
				if part.ThoughtSignature != "" {
					toolCall.Function.ThoughtSignature = part.ThoughtSignature
				}
				choice.Delta.ToolCalls = append(choice.Delta.ToolCalls, toolCall)
			}
		}

		// Finish reason
		if candidate.FinishReason != "" {
			reason := mapFinishReason(candidate.FinishReason)
			choice.FinishReason = &reason
		}

		streamChunk.Choices = append(streamChunk.Choices, choice)
	}

	return streamChunk
}

// mapFinishReason maps Gemini finish reason to OpenAI format.
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

// generateID generates a simple unique ID.
func generateID() string {
	return strconv.FormatInt(int64(1000000000+len("test")), 10)
}
