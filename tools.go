//go:build tools

// tools.go pins the build-time dependencies invoked via `go run` from
// go:generate directives. Without this file `go mod tidy` prunes the
// tool-only modules (x/mod, x/tools) and codegen breaks.
package tools

import (
	_ "go.uber.org/mock/mockgen"
)
