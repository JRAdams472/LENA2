// Command coveragefilter removes gomock mock packages from a Go coverage
// profile so the reported total reflects production and generated sqlc code.
//
// Usage:
//
//	go run ./tools/coveragefilter < coverage.out > coverage-filtered.out
//
// It passes the mode header through unchanged and drops any cover block
// whose file path contains a /mock/ directory segment.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// isGenerated reports whether path refers to a gomock mock package,
// identified by a /mock/ path segment.
func isGenerated(path string) bool {
	return strings.Contains(path, "/mock/")
}

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "coveragefilter:", err)
		os.Exit(1)
	}
}

func run(in *os.File, outFile *os.File) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	out := bufio.NewWriter(outFile)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "mode:") {
			// Cover block lines start with "<file>:<start-line>.<col>,..." —
			// the file path is everything before the first colon.
			path, _, found := strings.Cut(line, ":")
			if !found || isGenerated(path) {
				continue
			}
		}
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return out.Flush()
}
