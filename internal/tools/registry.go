package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ddtfch/codeaudit/internal/llm"
)

// Handler is a function that executes a tool given its JSON-encoded arguments.
type Handler func(ctx context.Context, args json.RawMessage) (string, error)

// Registry holds all available tools and dispatches LLM tool-call requests.
type Registry struct {
	schemas  []llm.ToolSchema
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(schema llm.ToolSchema, h Handler) {
	r.schemas = append(r.schemas, schema)
	r.handlers[schema.Name] = h
}

// Schemas returns all tool schemas for inclusion in LLM calls.
func (r *Registry) Schemas() []llm.ToolSchema {
	return r.schemas
}

// Dispatch executes a tool call by name and returns the result string.
func (r *Registry) Dispatch(ctx context.Context, call llm.ToolCall) (string, error) {
	h, ok := r.handlers[call.Name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", call.Name)
	}
	return h(ctx, json.RawMessage(call.Arguments))
}
