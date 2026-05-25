package types

// OpenAI-compatible request/response types.

// ChatCompletionRequest represents an OpenAI chat completion request.
type ChatCompletionRequest struct {
	Model            string              `json:"model"`
	Messages         []ChatMessage       `json:"messages"`
	Temperature      *float64            `json:"temperature,omitempty"`
	MaxTokens        *int                `json:"max_tokens,omitempty"`
	TopP             *float64            `json:"top_p,omitempty"`
	Stop             []string            `json:"stop,omitempty"`
	Stream           bool                `json:"stream,omitempty"`
	Tools            []Tool              `json:"tools,omitempty"`
	ToolChoice       interface{}         `json:"tool_choice,omitempty"`
	Functions        []Function          `json:"functions,omitempty"`
	FunctionCall     interface{}         `json:"function_call,omitempty"`
	PresencePenalty  *float64            `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64            `json:"frequency_penalty,omitempty"`
	User             string              `json:"user,omitempty"`

	// Gemini extensions
	ThinkingBudget   *int   `json:"thinking_budget,omitempty"` // -1=dynamic, 0=disabled, >0=token limit
}

// ChatMessage represents a message in the conversation.
type ChatMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content,omitempty"`
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ContentPart represents a part of a multimodal content.
type ContentPart struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL *ImageURL   `json:"image_url,omitempty"`
}

// ImageURL represents an image URL.
type ImageURL struct {
	URL string `json:"url"`
}

// Tool represents a tool definition.
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function represents a function definition.
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// ToolCall represents a tool call in the response.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call.
type FunctionCall struct {
	Name           string `json:"name"`
	Arguments      string `json:"arguments"`
	ThoughtSignature string `json:"thought_signature,omitempty"` // Gemini extension: required for thinking models
}

// ChatCompletionResponse represents an OpenAI chat completion response.
type ChatCompletionResponse struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []Choice             `json:"choices"`
	Usage   *Usage               `json:"usage,omitempty"`
}

// Choice represents a completion choice.
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage represents token usage.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a streaming response chunk.
type StreamChunk struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// StreamChoice represents a choice in the streaming response.
type StreamChoice struct {
	Index        int            `json:"index"`
	Delta        StreamDelta    `json:"delta"`
	FinishReason *string        `json:"finish_reason"`
}

// StreamDelta represents the delta in a streaming response.
type StreamDelta struct {
	Role    string     `json:"role,omitempty"`
	Content string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}
