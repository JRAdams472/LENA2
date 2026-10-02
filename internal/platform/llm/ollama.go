package llm

import (
	"context"
	"fmt"
	"strconv"

	"github.com/JRAdams472/LENA2/internal/platform/ollamaclient"
)

// ollamaProvider adapts *ollamaclient.Client to Provider. Ollama's /api/chat
// returns tool calls as message.tool_calls entries without IDs, so IDs are
// synthesized by position for correlating tool results back to the model.
type ollamaProvider struct {
	client *ollamaclient.Client
}

// NewOllamaProvider wraps an Ollama HTTP endpoint as a Provider.
func NewOllamaProvider(p Params) Provider {
	c := ollamaclient.New(p.URL, p.Model, p.Temperature, p.NumCtx)
	if p.Timeout > 0 {
		c = ollamaclient.NewWithTimeout(p.URL, p.Model, p.Temperature, p.NumCtx, p.Timeout)
	}
	return &ollamaProvider{client: c}
}

func (p *ollamaProvider) Name() string { return "ollama" }

func (p *ollamaProvider) Chat(ctx context.Context, req Request) (Response, error) {
	raw := ollamaclient.ChatRequest{
		Messages: toOllamaMessages(req.Messages),
		Tools:    toOllamaTools(req.Tools),
	}
	// Ollama cannot combine format:"json" with tool calling — when tools are
	// advertised the model answers with structured tool_calls instead.
	if len(req.Tools) == 0 {
		if req.JSONSchema != nil {
			raw.Format = req.JSONSchema
		} else if req.JSONMode {
			raw.Format = "json"
		}
	}
	resp, err := p.client.ChatRaw(ctx, raw)
	if err != nil {
		return Response{}, fmt.Errorf("ollama chat: %w", err)
	}
	return Response{Message: fromOllamaMessage(resp.Message)}, nil
}

func toOllamaMessages(ms []Message) []ollamaclient.Message {
	out := make([]ollamaclient.Message, len(ms))
	for i, m := range ms {
		out[i] = ollamaclient.Message{
			Role:      m.Role,
			Content:   m.Content,
			Name:      m.Name,
			ToolCalls: toOllamaToolCalls(m.ToolCalls),
		}
	}
	return out
}

func toOllamaToolCalls(cs []ToolCall) []ollamaclient.ToolCall {
	if len(cs) == 0 {
		return nil
	}
	out := make([]ollamaclient.ToolCall, len(cs))
	for i, c := range cs {
		out[i] = ollamaclient.ToolCall{
			Function: ollamaclient.ToolCallFunction{Name: c.Name, Arguments: c.Arguments},
		}
	}
	return out
}

func toOllamaTools(ts []ToolSpec) []ollamaclient.ToolSpec {
	if len(ts) == 0 {
		return nil
	}
	out := make([]ollamaclient.ToolSpec, len(ts))
	for i, t := range ts {
		out[i] = ollamaclient.ToolSpec{
			Type: "function",
			Function: ollamaclient.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}
	return out
}

func fromOllamaMessage(m ollamaclient.Message) Message {
	out := Message{Role: m.Role, Content: m.Content}
	if len(m.ToolCalls) > 0 {
		out.ToolCalls = make([]ToolCall, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			out.ToolCalls[i] = ToolCall{
				ID:        "call-" + strconv.Itoa(i),
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
	}
	return out
}
