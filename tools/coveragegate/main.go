// Command coveragegate checks per-module and per-surface coverage floors
// across the app's three coverage profiles (Go coverprofile, web lcov,
// mobile lcov).
//
// Usage:
//
//	go run ./tools/coveragegate [-config tools/coveragegate/floors.json]
//
// Floors and exclusions live in the config file so every exception to the
// coverage policy is auditable in one place. Per-module violations are
// reported; they fail the run only when "enforceModules" is true in the
// config (flipped once remediation lands). Per-surface totals always
// enforce their floor.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type surface struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"` // "gocover" or "lcov"
	Path    string   `json:"path"`
	Module  string   `json:"module"` // "package" (dir) or "file"
	Prefix  string   `json:"prefix"`
	Floor   float64  `json:"floor"`
	Exclude []string `json:"exclude"`
}

type config struct {
	ModuleFloor    float64   `json:"moduleFloor"`
	EnforceModules bool      `json:"enforceModules"`
	MinUnits       int       `json:"minUnits"`
	Surfaces       []surface `json:"surfaces"`
}

// moduleStat accumulates coverage units for one module (package or file).
type moduleStat struct {
	Units   int
	Covered int
}

func main() {
	cfgPath := flag.String("config", "tools/coveragegate/floors.json", "path to floors config")
	flag.Parse()

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "coveragegate:", err)
		os.Exit(2)
	}
	os.Exit(report(cfg))
}

func loadConfig(path string) (*config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

// report evaluates every configured surface, prints a per-module table, and
// returns 0 when all enforced floors hold.
func report(cfg *config) int {
	failed := false
	for _, s := range cfg.Surfaces {
		mods, err := measure(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "coveragegate: %s: %v\n", s.Name, err)
			failed = true
			continue
		}
		failed = printSurface(s, cfg, mods) || failed
	}
	if failed {
		fmt.Println("\nCOVERAGE GATE: FAIL")
		return 1
	}
	fmt.Println("\nCOVERAGE GATE: PASS")
	return 0
}

func printSurface(s surface, cfg *config, mods map[string]*moduleStat) bool {
	names := sortedKeys(mods)
	var totalUnits, totalCovered int
	var below []string

	fmt.Printf("\n=== %s (%s) ===\n", s.Name, s.Kind)
	for _, name := range names {
		m := mods[name]
		totalUnits += m.Units
		totalCovered += m.Covered
		pct := pct(m.Covered, m.Units)
		mark := ""
		switch {
		case m.Units < cfg.MinUnits:
			mark = "   (tiny: below min-units, exempt)"
		case pct < cfg.ModuleFloor:
			mark = " ** BELOW MODULE FLOOR"
			below = append(below, fmt.Sprintf("%s %.1f%%", name, pct))
		}
		fmt.Printf("  %-55s %6.1f%%  (%d/%d)%s\n", name, pct, m.Covered, m.Units, mark)
	}

	totalPct := pct(totalCovered, totalUnits)
	failed := totalPct < s.Floor
	status := "ok"
	if failed {
		status = "FAIL"
	}
	fmt.Printf("  %-55s %6.1f%%  (%d/%d) floor=%.0f%% [%s]\n", "TOTAL", totalPct, totalCovered, totalUnits, s.Floor, status)
	if len(below) > 0 {
		mode := "report-only"
		if cfg.EnforceModules {
			mode = "ENFORCED"
		}
		fmt.Printf("  modules below %.0f%% floor (%s): %s\n", cfg.ModuleFloor, mode, strings.Join(below, ", "))
	}
	return failed || (cfg.EnforceModules && len(below) > 0)
}

// measure parses a profile and returns module -> stats, after exclusions.
func measure(s surface) (map[string]*moduleStat, error) {
	f, err := os.Open(s.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var raw map[string]*moduleStat
	switch s.Kind {
	case "gocover":
		raw, err = parseGoCover(f, s)
	case "lcov":
		raw, err = parseLCOV(f, s)
	default:
		return nil, fmt.Errorf("unknown kind %q", s.Kind)
	}
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// parseGoCover reads a Go coverage profile and groups statements by package
// directory. -coverpkg duplicates blocks across test binaries, so identical
// file+range keys are merged (covered if hit anywhere).
func parseGoCover(f *os.File, s surface) (map[string]*moduleStat, error) {
	type block struct {
		stmts   int
		covered bool
	}
	blocks := map[string]*block{}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}
		// Format: <file>:<sl>.<sc>,<el>.<ec> <numStmts> <count>
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("malformed cover line: %q", line)
		}
		key := fields[0]
		stmts, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("malformed stmt count in %q", line)
		}
		cnt, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("malformed count in %q", line)
		}
		if b, ok := blocks[key]; ok {
			b.covered = b.covered || cnt > 0
		} else {
			blocks[key] = &block{stmts: stmts, covered: cnt > 0}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	mods := map[string]*moduleStat{}
	for key, b := range blocks {
		file, _, _ := strings.Cut(key, ":")
		file = strings.TrimPrefix(file, s.Prefix)
		if excluded(s.Exclude, file) {
			continue
		}
		mod := file
		if s.Module == "package" {
			mod = dirOf(file)
		}
		m := mods[mod]
		if m == nil {
			m = &moduleStat{}
			mods[mod] = m
		}
		m.Units += b.stmts
		if b.covered {
			m.Covered += b.stmts
		}
	}
	return mods, nil
}

// parseLCOV reads an lcov tracefile and groups hit lines per file.
// Duplicate SF sections are merged (a line counts covered if hit anywhere).
func parseLCOV(f *os.File, s surface) (map[string]*moduleStat, error) {
	// file -> line -> hit?
	lines := map[string]map[int]bool{}
	var cur string

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "SF:"):
			cur = normPath(strings.TrimPrefix(line, "SF:"))
			cur = strings.TrimPrefix(cur, s.Prefix)
		case strings.HasPrefix(line, "DA:"):
			if cur == "" {
				continue
			}
			parts := strings.SplitN(strings.TrimPrefix(line, "DA:"), ",", 2)
			if len(parts) != 2 {
				continue
			}
			ln, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			hit, _ := strconv.ParseFloat(strings.Split(parts[1], ",")[0], 64)
			m := lines[cur]
			if m == nil {
				m = map[int]bool{}
				lines[cur] = m
			}
			m[ln] = m[ln] || hit > 0
		case line == "end_of_record":
			cur = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	mods := map[string]*moduleStat{}
	for file, fileLines := range lines {
		if excluded(s.Exclude, file) {
			continue
		}
		mod := file
		if s.Module == "package" {
			mod = dirOf(file)
		}
		m := mods[mod]
		if m == nil {
			m = &moduleStat{}
			mods[mod] = m
		}
		m.Units += len(fileLines)
		for _, hit := range fileLines {
			if hit {
				m.Covered++
			}
		}
	}
	return mods, nil
}

func normPath(p string) string {
	return strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
}

func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return "."
}

func pct(covered, units int) float64 {
	if units == 0 {
		return 100
	}
	return 100 * float64(covered) / float64(units)
}

func sortedKeys(m map[string]*moduleStat) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		pi, pj := pct(m[keys[i]].Covered, m[keys[i]].Units), pct(m[keys[j]].Covered, m[keys[j]].Units)
		if pi != pj {
			return pi < pj
		}
		return keys[i] < keys[j]
	})
	return keys
}

// excluded reports whether path matches any glob in patterns. Supported
// syntax: `*` within a segment, `**` spanning segments, `?` single char.
func excluded(patterns []string, path string) bool {
	for _, p := range patterns {
		if globMatch(p, path) {
			return true
		}
	}
	return false
}

var globCache = map[string]*regexp.Regexp{}

func globMatch(pattern, path string) bool {
	re, ok := globCache[pattern]
	if !ok {
		re = regexp.MustCompile("^" + globToRegex(pattern) + "$")
		globCache[pattern] = re
	}
	return re.MatchString(path)
}

func globToRegex(pattern string) string {
	var b strings.Builder
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return b.String()
}
