package types

// Gemini API request/response types.

// GeminiRequest represents a Gemini API request.
type GeminiRequest struct {
	Contents          []GeminiContent    `json:"contents"`
	SystemInstruction *GeminiContent     `json:"system_instruction,omitempty"`
	Tools             []GeminiTool       `json:"tools,omitempty"`
	GenerationConfig  *GeminiGenConfig   `json:"generationConfig,omitempty"`
	SafetySettings    []GeminiSafety     `json:"safetySettings,omitempty"`
}

// GeminiContent represents content in Gemini format.
type GeminiContent struct {
	Role  string         `json:"role"`
	Parts []GeminiPart   `json:"parts"`
}

// GeminiPart represents a part of content.
type GeminiPart struct {
	Text             string                     `json:"text,omitempty"`
	Thought          bool                       `json:"thought,omitempty"`
	ThoughtSignature string                     `json:"thoughtSignature,omitempty"`
	InlineData       *GeminiBlob                `json:"inlineData,omitempty"`
	FileData         *GeminiFileData            `json:"fileData,omitempty"`
	FunctionCall     *GeminiFunctionCall        `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResponse    `json:"functionResponse,omitempty"`
}

// GeminiBlob represents inline data.
type GeminiBlob struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// GeminiFileData represents file data.
type GeminiFileData struct {
	MimeType string `json:"mimeType"`
	FileURI  string `json:"fileUri"`
}

// GeminiFunctionCall represents a function call.
type GeminiFunctionCall struct {
	Name string      `json:"name"`
	Args interface{} `json:"args"`
}

// GeminiFunctionResponse represents a function response.
type GeminiFunctionResponse struct {
	Name    string      `json:"name"`
	Response interface{} `json:"response"`
}

// GeminiTool represents a tool in Gemini format.
type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDecl `json:"functionDeclarations,omitempty"`
}

// GeminiFunctionDecl represents a function declaration.
type GeminiFunctionDecl struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// GeminiGenConfig represents generation configuration.
type GeminiGenConfig struct {
	Temperature     float64  `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	TopP            float64  `json:"topP,omitempty"`
	TopK            int      `json:"topK,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
	CandidateCount  int      `json:"candidateCount,omitempty"`
	ThinkingConfig  *GeminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

// GeminiThinkingConfig represents thinking configuration.
type GeminiThinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget,omitempty"` // -1=dynamic, 0=disabled, >0=token limit
}

// GeminiSafety represents a safety setting.
type GeminiSafety struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// GeminiResponse represents a Gemini API response.
type GeminiResponse struct {
	Candidates     []GeminiCandidate    `json:"candidates"`
	PromptFeedback *GeminiPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *GeminiUsage         `json:"usageMetadata,omitempty"`
}

// GeminiCandidate represents a response candidate.
type GeminiCandidate struct {
	Content       GeminiContent `json:"content"`
	FinishReason  string        `json:"finishReason,omitempty"`
	Index         int           `json:"index"`
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

// GeminiSafetyRating represents a safety rating.
type GeminiSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

// GeminiPromptFeedback represents prompt feedback.
type GeminiPromptFeedback struct {
	BlockReason string `json:"blockReason,omitempty"`
}

// GeminiUsage represents usage metadata.
type GeminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// GeminiStreamChunk represents a streaming response chunk.
type GeminiStreamChunk struct {
	Candidates    []GeminiCandidate `json:"candidates"`
	UsageMetadata *GeminiUsage      `json:"usageMetadata,omitempty"`
}
