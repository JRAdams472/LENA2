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

func TestToolSpecs_ListsCatalog(t *testing.T) {
	svc := NewService(nil, testRegistry(), Config{})
	specs := svc.ToolSpecs()
	require.Len(t, specs, 1)
	assert.Equal(t, "echo_scope", specs[0].Name)

	// No service at all → no specs.
	var nilSvc *Service
	assert.Empty(t, nilSvc.ToolSpecs())
}

func TestCallTool_ScopedResult(t *testing.T) {
	svc := NewService(nil, testRegistry(), Config{})
	out, err := svc.CallTool(context.Background(), 7, 9, "echo_scope", nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"user":7,"household":9}`, out)
}

func TestCallTool_ErrorsBecomePayloads(t *testing.T) {
	svc := NewService(nil, testRegistry(), Config{})

	out, err := svc.CallTool(context.Background(), 7, 9, "does_not_exist", nil)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "unknown tool")

	// Invalid arguments are reported to the model the same way the
	// server loop reports them — as an error payload, not a Go error.
	out, err = svc.CallTool(context.Background(), 7, 9, "echo_scope", json.RawMessage(`{bad`))
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "invalid tool arguments")
}

func TestCallTool_HandlerErrorPayload(t *testing.T) {
	reg := tools.New()
	reg.Register(llm.ToolSpec{Name: "broken", Parameters: map[string]any{}},
		func(context.Context, tools.Scope, json.RawMessage) (any, error) {
			return nil, errors.New("db exploded")
		})
	svc := NewService(nil, reg, Config{})
	out, err := svc.CallTool(context.Background(), 1, 1, "broken", nil)
	require.NoError(t, err)
	assert.Contains(t, out, "db exploded")
}

func TestCallTool_Truncates(t *testing.T) {
	reg := tools.New()
	reg.Register(llm.ToolSpec{Name: "big", Parameters: map[string]any{}},
		func(context.Context, tools.Scope, json.RawMessage) (any, error) {
			return map[string]any{"blob": strings.Repeat("x", 5000)}, nil
		})
	svc := NewService(nil, reg, Config{ToolResultMaxBytes: 100})
	out, err := svc.CallTool(context.Background(), 1, 1, "big", nil)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), 120)
	assert.Contains(t, out, "truncated")
}

func TestCallTool_NilService(t *testing.T) {
	var svc *Service
	_, err := svc.CallTool(context.Background(), 1, 1, "x", nil)
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestPrompt_Catalog(t *testing.T) {
	svc := NewService(nil, testRegistry(), Config{})
	p, ok := svc.Prompt("ask")
	require.True(t, ok)
	assert.Contains(t, p, "Dot")

	_, ok = svc.Prompt("suggest-meals")
	assert.True(t, ok)

	_, ok = svc.Prompt("system-override")
	assert.False(t, ok)
	_, ok = svc.Prompt("")
	assert.False(t, ok)
}
