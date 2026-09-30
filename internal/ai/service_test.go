package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/ai/tools"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

func testRegistry() *tools.Registry {
	reg := tools.New()
	reg.Register(llm.ToolSpec{
		Name:       "echo_scope",
		Parameters: map[string]any{"type": "object"},
	}, func(_ context.Context, scope tools.Scope, _ json.RawMessage) (any, error) {
		return map[string]any{"user": scope.UserID, "household": scope.HouseholdID}, nil
	})
	return reg
}

func TestAsk_ToolCallLoop(t *testing.T) {
	p := llm.NewMockProvider()
	p.EnqueueToolCalls(llm.ToolCall{ID: "call-0", Name: "echo_scope", Arguments: json.RawMessage(`{}`)})
	p.EnqueueText("You have flour.")

	svc := NewService(p, testRegistry(), Config{})
	ans, err := svc.Ask(context.Background(), 7, 9, "what's in my pantry?")
	require.NoError(t, err)
	assert.Equal(t, "You have flour.", ans.Text)
	require.Len(t, ans.Tools, 1)
	assert.Equal(t, "echo_scope", ans.Tools[0].Name)

	// Second request fed the tool result back with the caller's scope.
	require.Len(t, p.Requests, 2)
	toolMsg := p.Requests[1].Messages[len(p.Requests[1].Messages)-1]
	assert.Equal(t, llm.RoleTool, toolMsg.Role)
	assert.Equal(t, "call-0", toolMsg.ToolCallID)
	assert.JSONEq(t, `{"user":7,"household":9}`, toolMsg.Content)
}

func TestAsk_NoTools(t *testing.T) {
	p := llm.NewMockProvider()
	p.EnqueueText("Hello!")
	svc := NewService(p, testRegistry(), Config{})
	ans, err := svc.Ask(context.Background(), 1, 1, "hi")
	require.NoError(t, err)
	assert.Equal(t, "Hello!", ans.Text)
	assert.Empty(t, ans.Tools)
	assert.Equal(t, 1, p.CallCount())
}

func TestAsk_UnknownToolReportedToModel(t *testing.T) {
	p := llm.NewMockProvider()
	p.EnqueueToolCalls(llm.ToolCall{ID: "call-0", Name: "drop_table", Arguments: json.RawMessage(`{}`)})
	p.EnqueueText("I can't do that.")

	svc := NewService(p, testRegistry(), Config{})
	ans, err := svc.Ask(context.Background(), 1, 1, "drop the table")
	require.NoError(t, err)
	assert.Equal(t, "I can't do that.", ans.Text)

	toolMsg := p.Requests[1].Messages[len(p.Requests[1].Messages)-1]
	assert.Contains(t, toolMsg.Content, "unknown tool")
}

func TestAsk_ToolErrorReportedToModel(t *testing.T) {
	reg := tools.New()
	reg.Register(llm.ToolSpec{Name: "broken", Parameters: map[string]any{}},
		func(context.Context, tools.Scope, json.RawMessage) (any, error) {
			return nil, errors.New("db exploded")
		})
	p := llm.NewMockProvider()
	p.EnqueueToolCalls(llm.ToolCall{ID: "call-0", Name: "broken", Arguments: json.RawMessage(`{}`)})
	p.EnqueueText("Something went wrong reading data.")

	svc := NewService(p, reg, Config{})
	ans, err := svc.Ask(context.Background(), 1, 1, "q")
	require.NoError(t, err)
	toolMsg := p.Requests[1].Messages[len(p.Requests[1].Messages)-1]
	assert.Contains(t, toolMsg.Content, "db exploded")
	assert.Equal(t, "Something went wrong reading data.", ans.Text)
}

func TestAsk_RoundLimit(t *testing.T) {
	p := llm.NewMockProvider()
	// Every round requests another tool call forever.
	p.Handler = func(_ llm.Request) (llm.Response, error) {
		return llm.Response{Message: llm.Message{
			Role:      llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{{ID: "c", Name: "echo_scope", Arguments: json.RawMessage(`{}`)}},
		}}, nil
	}
	svc := NewService(p, testRegistry(), Config{MaxToolRounds: 2})
	_, err := svc.Ask(context.Background(), 1, 1, "loop")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool rounds")
	assert.Equal(t, 3, p.CallCount()) // rounds 0..2 inclusive
}

func TestAsk_Validation(t *testing.T) {
	p := llm.NewMockProvider()
	svc := NewService(p, testRegistry(), Config{})

	_, err := svc.Ask(context.Background(), 1, 1, "   ")
	assert.ErrorIs(t, err, ErrInvalidQuestion)

	_, err = svc.Ask(context.Background(), 1, 1, strings.Repeat("x", 2001))
	assert.ErrorIs(t, err, ErrInvalidQuestion)
	assert.Equal(t, 0, p.CallCount())
}

func TestAsk_Unavailable(t *testing.T) {
	svc := NewService(nil, testRegistry(), Config{})
	assert.False(t, svc.Available())
	_, err := svc.Ask(context.Background(), 1, 1, "hi")
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestAsk_ProviderError(t *testing.T) {
	p := llm.NewMockProvider()
	p.EnqueueError(errors.New("provider down"))
	svc := NewService(p, testRegistry(), Config{})
	_, err := svc.Ask(context.Background(), 1, 1, "hi")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider down")
}

func TestAsk_EmptyAnswer(t *testing.T) {
	p := llm.NewMockProvider()
	p.EnqueueText("   ")
	svc := NewService(p, testRegistry(), Config{})
	_, err := svc.Ask(context.Background(), 1, 1, "hi")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty answer")
}

func TestAsk_TruncatesToolResult(t *testing.T) {
	reg := tools.New()
	reg.Register(llm.ToolSpec{Name: "big", Parameters: map[string]any{}},
		func(context.Context, tools.Scope, json.RawMessage) (any, error) {
			return map[string]any{"blob": strings.Repeat("x", 5000)}, nil
		})
	p := llm.NewMockProvider()
	p.EnqueueToolCalls(llm.ToolCall{ID: "c", Name: "big", Arguments: json.RawMessage(`{}`)})
	p.EnqueueText("ok")

	svc := NewService(p, reg, Config{ToolResultMaxBytes: 100})
	_, err := svc.Ask(context.Background(), 1, 1, "q")
	require.NoError(t, err)
	toolMsg := p.Requests[1].Messages[len(p.Requests[1].Messages)-1]
	assert.LessOrEqual(t, len(toolMsg.Content), 120)
	assert.Contains(t, toolMsg.Content, "truncated")
}
