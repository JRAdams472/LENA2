// Command coveragefilter removes generated packages (sqlc query code and
// gomock mocks) from a Go coverage profile so the reported total reflects
// only hand-written code.
//
// Usage:
//
//	go run ./tools/coveragefilter < coverage.out > coverage-filtered.out
//
// It passes the mode header through unchanged and drops any cover block
// whose file path contains a /sqlc/ or /mock/ directory segment.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// isGenerated reports whether path refers to generated code: sqlc output or
// gomock mocks, identified by a /sqlc/ or /mock/ path segment.
func isGenerated(path string) bool {
	return strings.Contains(path, "/sqlc/") || strings.Contains(path, "/mock/")
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1024*1024), 1024*1024)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	for in.Scan() {
		line := in.Text()
		if strings.HasPrefix(line, "mode:") {
			fmt.Fprintln(out, line)
			continue
		}
		// Cover block lines start with "<file>:<start-line>.<col>,..." — the
		// file path is everything before the first colon.
		path, _, found := strings.Cut(line, ":")
		if !found || isGenerated(path) {
			continue
		}
		fmt.Fprintln(out, line)
	}
	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "coveragefilter:", err)
		os.Exit(1)
	}
}
