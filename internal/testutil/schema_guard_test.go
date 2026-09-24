package testutil_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestQueriesStayInOwnSchema enforces the documented one-schema-per-module
// rule (audit A1-01): each domain's queries.sql may only reference tables in
// its own schema. Schema-qualified references are detected by the SQL
// keywords that introduce them (FROM/JOIN/INTO/UPDATE), which avoids false
// positives from table aliases.
//
// Allowed exceptions (documented in docs/adr/):
//   - internal/analytics is a read-model that denormalises from recipe.*,
//     mealplan.*, and identity.users (household fan-out) (ADR-001).
//   - internal/app/recipeimport owns the recipe_import tables that live in
//     the recipe schema (ADR-002).
var allowedSchemas = map[string][]string{
	"analytics":        {"analytics", "recipe", "mealplan", "identity"},
	"app/recipeimport": {"recipe"},
	"grocery":          {"grocery"},
	"household":        {"household"},
	"identity":         {"identity"},
	"inventory":        {"inventory"},
	"mealplan":         {"mealplan"},
	"recipe":           {"recipe"},
	"userprefs":        {"userprefs"},
	"wine":             {"wine"},
}

var schemaRef = regexp.MustCompile(`(?i)\b(?:FROM|JOIN|INTO|UPDATE)\s+([a-z_]+)\.`)

func TestQueriesStayInOwnSchema(t *testing.T) {
	const internalDir = ".."

	dirs, err := findQueriesDirs(internalDir)
	if err != nil {
		t.Fatalf("scan internal dirs: %v", err)
	}
	for _, dir := range dirs {
		queriesPath := filepath.Join(internalDir, dir, "queries.sql")
		raw, err := os.ReadFile(queriesPath) // #nosec G304 -- path is built from dirs discovered under internal/
		if err != nil {
			t.Fatalf("read %s: %v", queriesPath, err)
		}
		allowed, ok := allowedSchemas[dir]
		if !ok {
			t.Fatalf("%s: no schema ownership declared in allowedSchemas", dir)
		}
		allowedSet := make(map[string]bool, len(allowed))
		for _, s := range allowed {
			allowedSet[s] = true
		}
		for _, m := range schemaRef.FindAllStringSubmatch(string(raw), -1) {
			// sqlc.arg()/sqlc.narg() named parameters are codegen syntax, not
			// schema references (e.g. "IS NOT DISTINCT FROM sqlc.narg(x)").
			if m[1] == "sqlc" {
				continue
			}
			if !allowedSet[m[1]] {
				t.Errorf("%s references foreign schema %q (allowed: %s)", queriesPath, m[1], strings.Join(allowed, ", "))
			}
		}
	}
}

// findQueriesDirs returns the relative paths under internal that contain a
// queries.sql file, one level of nesting deep (e.g. "analytics",
// "app/recipeimport").
func findQueriesDirs(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "queries.sql" {
			return nil
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out, err
}
