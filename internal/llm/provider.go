package llm

import "context"

// Message roles.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message is a single chat turn.
type Message struct {
	Role       string      `json:"role"`
	Content    string      `json:"content,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"` // for role=tool results
}

// ToolCall represents an LLM-requested tool invocation.
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolSchema is the JSON schema definition for a tool the LLM can call.
type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema object
}

// Response is what the provider returns from a chat call.
type Response struct {
	Content   string
	ToolCalls []ToolCall
	Usage     Usage
}

// Usage tracks token consumption for cost accounting.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Provider is the single interface all LLM adapters must satisfy.
type Provider interface {
	// Chat sends a conversation and returns the assistant turn.
	Chat(ctx context.Context, messages []Message, tools []ToolSchema) (*Response, error)
	// Name returns a human-readable identifier (e.g. "anthropic/claude-opus-4-8").
	Name() string
}
