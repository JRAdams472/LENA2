package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"**/sqlc/**", "internal/session/sqlc/query.sql.go", true},
		{"**/sqlc/**", "internal/session/resolver.go", false},
		{"**/mock/**", "internal/bff/mock/mock_resolver.go", true},
		{"cmd/testissuer/**", "cmd/testissuer/main.go", true},
		{"cmd/testissuer/**", "cmd/lena/main.go", false},
		{"lib/ai/engines/**", "lib/ai/engines/nano.ts", true},
		{"lib/ai/gemma_binding.dart", "lib/ai/gemma_binding.dart", true},
		{"lib/ai/gemma_binding.dart", "lib/ai/gemma_engine.dart", false},
		{"internal/platform/dbtx/dbtxtest/**", "internal/platform/dbtx/dbtxtest/helper.go", true},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, globMatch(c.pattern, c.path), "%s vs %s", c.pattern, c.path)
	}
}

func TestParseGoCover(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "coverage.out")
	content := `mode: atomic
github.com/JRAdams472/LENA2/internal/bff/resolver.go:10.20,15.3 2 1
github.com/JRAdams472/LENA2/internal/bff/resolver.go:20.1,25.2 3 0
github.com/JRAdams472/LENA2/internal/bff/resolver.go:10.20,15.3 2 0
github.com/JRAdams472/LENA2/internal/session/sqlc/q.sql.go:5.1,8.2 4 0
github.com/JRAdams472/LENA2/cmd/lena/main.go:1.1,4.2 1 1
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	mods, err := parseGoCover(f, surface{
		Kind:    "gocover",
		Module:  "package",
		Prefix:  "github.com/JRAdams472/LENA2/",
		Exclude: []string{"**/sqlc/**"},
	})
	require.NoError(t, err)

	// Duplicate block merged (covered wins), sqlc excluded.
	bff := mods["internal/bff"]
	require.NotNil(t, bff)
	assert.Equal(t, 5, bff.Units)
	assert.Equal(t, 2, bff.Covered)
	assert.Nil(t, mods["internal/session/sqlc"])
	assert.Equal(t, 1, mods["cmd/lena"].Covered)
}

func TestParseLCOV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lcov.info")
	content := `TN:
SF:lib\screens\foo.dart
DA:10,1
DA:11,0
DA:12,1
end_of_record
SF:lib\ai\gemma_binding.dart
DA:5,0
DA:6,0
end_of_record
SF:lib/screens/bar.dart
DA:1,3
DA:2,0
end_of_record
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	mods, err := parseLCOV(f, surface{
		Kind:    "lcov",
		Module:  "file",
		Exclude: []string{"lib/ai/gemma_binding.dart"},
	})
	require.NoError(t, err)

	foo := mods["lib/screens/foo.dart"]
	require.NotNil(t, foo)
	assert.Equal(t, 3, foo.Units)
	assert.Equal(t, 2, foo.Covered)
	assert.Nil(t, mods["lib/ai/gemma_binding.dart"])
	require.NotNil(t, mods["lib/screens/bar.dart"])
}

func TestReportFloors(t *testing.T) {
	dir := t.TempDir()
	lcov := filepath.Join(dir, "lcov.info")
	require.NoError(t, os.WriteFile(lcov, []byte(
		"SF:a.dart\nDA:1,1\nDA:2,1\nDA:3,0\nend_of_record\n"), 0o644))

	// 66.7% < 70 floor -> surface fails even though enforceModules is off.
	cfg := &config{
		ModuleFloor: 70, EnforceModules: false, MinUnits: 1,
		Surfaces: []surface{{Name: "m", Kind: "lcov", Path: lcov, Module: "file", Floor: 70}},
	}
	assert.Equal(t, 1, report(cfg))

	cfg.Surfaces[0].Floor = 50
	assert.Equal(t, 0, report(cfg))
}
