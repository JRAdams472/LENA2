package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

// ErrInvalidArgs wraps every argument-validation failure so the caller can
// report a bad tool call to the model without leaking internals.
var ErrInvalidArgs = errors.New("invalid tool arguments")

// MaxArgsBytes bounds a single tool-call argument payload. Arguments are
// model-generated data — nothing registered needs more than this.
const MaxArgsBytes = 4096

// ValidateArgs checks args against the tool spec's JSON-Schema parameters
// before a handler sees them. The registry calls this on every Call so both
// the server-side assistant loop and client-driven tool calls get the same
// guardrails. It supports the schema subset the registered tools use:
// object/properties/required, the primitive types, arrays with item
// schemas, and numeric bounds. Missing required fields and undeclared
// properties are rejected — schemas are closed.
func ValidateArgs(spec llm.ToolSpec, args json.RawMessage) error {
	var v any
	if len(bytes.TrimSpace(args)) == 0 {
		v = map[string]any{}
	} else if err := json.Unmarshal(args, &v); err != nil {
		return fmt.Errorf("%w: malformed JSON: %w", ErrInvalidArgs, err)
	}
	if len(spec.Parameters) == 0 {
		return nil
	}
	return validateValue(spec.Parameters, v, "arguments")
}

func validateValue(schema map[string]any, v any, path string) error {
	typ, _ := schema["type"].(string)
	switch typ {
	case "":
		// No declared type — nothing to check.
		return nil
	case "object":
		return validateObject(schema, v, path)
	case "string":
		return validatePrimitive[string](v, path, "string")
	case "integer":
		return validateNumber(schema, v, path, true)
	case "number":
		return validateNumber(schema, v, path, false)
	case "boolean":
		return validatePrimitive[bool](v, path, "boolean")
	case "array":
		return validateArray(schema, v, path)
	default:
		return fmt.Errorf("%w: %s has unsupported schema type %q", ErrInvalidArgs, path, typ)
	}
}

// validateObject checks required fields and recurses into declared
// properties. A declared property list is treated as closed —
// model-invented arguments get a clear error instead of reaching the
// handler silently ignored.
func validateObject(schema map[string]any, v any, path string) error {
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: %s must be an object", ErrInvalidArgs, path)
	}
	props, _ := schema["properties"].(map[string]any)
	for _, req := range stringList(schema["required"]) {
		if _, ok := m[req]; !ok {
			return fmt.Errorf("%w: %s.%s is required", ErrInvalidArgs, path, req)
		}
	}
	for key, val := range m {
		ps, ok := props[key]
		if !ok {
			if len(props) > 0 {
				return fmt.Errorf("%w: %s.%s is not a declared property", ErrInvalidArgs, path, key)
			}
			continue
		}
		sub, ok := ps.(map[string]any)
		if !ok {
			continue
		}
		if err := validateValue(sub, val, path+"."+key); err != nil {
			return err
		}
	}
	return nil
}

// validatePrimitive enforces a scalar JSON type.
func validatePrimitive[T string | bool](v any, path, name string) error {
	if _, ok := v.(T); !ok {
		return fmt.Errorf("%w: %s must be a %s", ErrInvalidArgs, path, name)
	}
	return nil
}

// validateNumber checks the float64 decode and, for integer schemas, that
// the value is integral, then applies numeric bounds.
func validateNumber(schema map[string]any, v any, path string, integer bool) error {
	f, ok := v.(float64)
	if !ok || (integer && f != math.Trunc(f)) {
		name := "number"
		if integer {
			name = "integer"
		}
		return fmt.Errorf("%w: %s must be a %s", ErrInvalidArgs, path, name)
	}
	return checkBounds(schema, f, path)
}

// validateArray recurses into each element against the items schema.
func validateArray(schema map[string]any, v any, path string) error {
	a, ok := v.([]any)
	if !ok {
		return fmt.Errorf("%w: %s must be an array", ErrInvalidArgs, path)
	}
	items, ok := schema["items"].(map[string]any)
	if !ok {
		return nil
	}
	for i, item := range a {
		if err := validateValue(items, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

// checkBounds applies minimum/maximum when the schema declares them.
func checkBounds(schema map[string]any, f float64, path string) error {
	if lo, ok := numOf(schema["minimum"]); ok && f < lo {
		return fmt.Errorf("%w: %s must be >= %v", ErrInvalidArgs, path, lo)
	}
	if hi, ok := numOf(schema["maximum"]); ok && f > hi {
		return fmt.Errorf("%w: %s must be <= %v", ErrInvalidArgs, path, hi)
	}
	return nil
}

// numOf reads a numeric literal from a decoded schema value (Go int,
// float64, or json.Number depending on how the map was built).
func numOf(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// stringList reads a required-style list that may be []string (hand-built
// schema maps) or []any (decoded JSON).
func stringList(v any) []string {
	switch l := v.(type) {
	case []string:
		return l
	case []any:
		out := make([]string, 0, len(l))
		for _, e := range l {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
