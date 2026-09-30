package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

func TestRegistry_SpecsOrderAndCall(t *testing.T) {
	reg := New()
	reg.Register(llm.ToolSpec{Name: "b_tool", Parameters: map[string]any{}},
		func(_ context.Context, s Scope, _ json.RawMessage) (any, error) {
			return s.HouseholdID * 2, nil
		})
	reg.Register(llm.ToolSpec{Name: "a_tool", Parameters: map[string]any{}},
		func(_ context.Context, _ Scope, args json.RawMessage) (any, error) {
			return string(args), nil
		})

	specs := reg.Specs()
	require.Len(t, specs, 2)
	assert.Equal(t, "b_tool", specs[0].Name)
	assert.Equal(t, "a_tool", specs[1].Name)

	out, err := reg.Call(context.Background(), Scope{UserID: 1, HouseholdID: 21}, "b_tool", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(42), out)

	out, err = reg.Call(context.Background(), Scope{}, "a_tool", json.RawMessage(`{"x":1}`))
	require.NoError(t, err)
	assert.Equal(t, `{"x":1}`, out)
}

func TestRegistry_UnknownTool(t *testing.T) {
	_, err := New().Call(context.Background(), Scope{}, "nope", nil)
	assert.ErrorIs(t, err, ErrUnknownTool)
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	reg := New()
	h := func(context.Context, Scope, json.RawMessage) (any, error) { return nil, nil }
	reg.Register(llm.ToolSpec{Name: "t"}, h)
	assert.Panics(t, func() {
		reg.Register(llm.ToolSpec{Name: "t"}, h)
	})
}
