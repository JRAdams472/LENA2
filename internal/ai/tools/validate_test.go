package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

var testSpec = llm.ToolSpec{
	Name: "cooked",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"recipeIds": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
			"query":     map[string]any{"type": "string"},
			"limit":     map[string]any{"type": "integer", "minimum": 1, "maximum": 500},
			"flag":      map[string]any{"type": "boolean"},
		},
		"required": []string{"query"},
	},
}

func TestValidateArgs_Happy(t *testing.T) {
	require.NoError(t, ValidateArgs(testSpec, json.RawMessage(`{"query":"soup","limit":5,"recipeIds":[1,2],"flag":true}`)))
}

func TestValidateArgs_EmptyBecomesObject(t *testing.T) {
	openSpec := llm.ToolSpec{Name: "any", Parameters: map[string]any{"type": "object"}}
	require.NoError(t, ValidateArgs(openSpec, nil))
	require.NoError(t, ValidateArgs(openSpec, json.RawMessage(`{"anything":"goes"}`)))
	require.ErrorIs(t, ValidateArgs(openSpec, json.RawMessage(`"nope"`)), ErrInvalidArgs)
}

func TestValidateArgs_RequiredAndTypes(t *testing.T) {
	for name, args := range map[string]string{
		"missing required": `{}`,
		"wrong type":       `{"query":42}`,
		"non-integer":      `{"query":"x","limit":2.5}`,
		"array wrong item": `{"query":"x","recipeIds":["a"]}`,
		"undeclared prop":  `{"query":"x","householdId":9}`,
		"below minimum":    `{"query":"x","limit":0}`,
		"above maximum":    `{"query":"x","limit":9999}`,
		"malformed":        `{oops`,
		"non-object root":  `[1,2]`,
	} {
		assert.ErrorIs(t, ValidateArgs(testSpec, json.RawMessage(args)), ErrInvalidArgs, name)
	}
}

func TestValidateArgs_NoSchemaUnconstrained(t *testing.T) {
	spec := llm.ToolSpec{Name: "open"}
	require.NoError(t, ValidateArgs(spec, json.RawMessage(`{"any":1}`)))
	require.NoError(t, ValidateArgs(spec, nil))
}

func TestCall_ValidatesBeforeHandler(t *testing.T) {
	reg := New()
	called := false
	reg.Register(testSpec, func(context.Context, Scope, json.RawMessage) (any, error) {
		called = true
		return "ok", nil
	})

	_, err := reg.Call(context.Background(), Scope{}, "cooked", json.RawMessage(`{"limit":"lots"}`))
	require.ErrorIs(t, err, ErrInvalidArgs)
	assert.False(t, called, "handler must not run on invalid args")

	out, err := reg.Call(context.Background(), Scope{}, "cooked", json.RawMessage(`{"query":"x"}`))
	require.NoError(t, err)
	assert.Equal(t, "ok", out)
}

func TestCall_ArgsSizeCap(t *testing.T) {
	reg := New()
	reg.Register(llm.ToolSpec{Name: "open"}, func(context.Context, Scope, json.RawMessage) (any, error) {
		return "ok", nil
	})
	big := json.RawMessage(`{"blob":"` + string(make([]byte, MaxArgsBytes)) + `"}`)
	_, err := reg.Call(context.Background(), Scope{}, "open", big)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidArgs))
}
