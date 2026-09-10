// Package nutritionparse converts raw Nutrition Facts OCR text into a list
// of canonical nutrients. It is deterministic and dependency-free.
package nutritionparse

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Nutrient is a single parsed nutrient value.
type Nutrient struct {
	Label  string
	Amount float64
	Unit   string
}

type aliasEntry struct {
	Aliases   []string
	Canonical string
	Unit      string
}

var aliases = []aliasEntry{
	{Aliases: []string{"Calories", "Cal"}, Canonical: "Calories", Unit: "kcal"},
	{Aliases: []string{"Total Fat", "Fat"}, Canonical: "Total Fat", Unit: "g"},
	{Aliases: []string{"Saturated Fat", "Sat Fat"}, Canonical: "Saturated Fat", Unit: "g"},
	{Aliases: []string{"Trans Fat"}, Canonical: "Trans Fat", Unit: "g"},
	{Aliases: []string{"Cholesterol"}, Canonical: "Cholesterol", Unit: "mg"},
	{Aliases: []string{"Sodium"}, Canonical: "Sodium", Unit: "mg"},
	{Aliases: []string{"Total Carbohydrate", "Total Carb", "Carbohydrate", "Carb"}, Canonical: "Total Carbohydrate", Unit: "g"},
	{Aliases: []string{"Dietary Fiber", "Fiber"}, Canonical: "Dietary Fiber", Unit: "g"},
	{Aliases: []string{"Total Sugars", "Sugars", "Sugar"}, Canonical: "Total Sugars", Unit: "g"},
	{Aliases: []string{"Added Sugars", "Added Sugar"}, Canonical: "Added Sugars", Unit: "g"},
	{Aliases: []string{"Protein"}, Canonical: "Protein", Unit: "g"},
	{Aliases: []string{"Calcium"}, Canonical: "Calcium", Unit: "mg"},
	{Aliases: []string{"Iron"}, Canonical: "Iron", Unit: "mg"},
	{Aliases: []string{"Potassium"}, Canonical: "Potassium", Unit: "mg"},
	{Aliases: []string{"Vitamin D"}, Canonical: "Vitamin D", Unit: "mcg"},
}

var (
	noiseRE  = regexp.MustCompile(`[^a-zA-Z0-9\s.%µ]`)
	spaceRE  = regexp.MustCompile(`\s+`)
	numberRE = regexp.MustCompile(`(?i)(?:^|\s)(-?\d+(?:\.\d+)?)\s*(mcg|µg|mg|kcal|g|%)?(?:\s|$)`)
)

func init() {
	// Sort aliases so longer matches are checked first, which prevents
	// "Fat" from shadowing "Total Fat".
	sort.Slice(aliases, func(i, j int) bool {
		maxI, maxJ := 0, 0
		for _, a := range aliases[i].Aliases {
			if len(a) > maxI {
				maxI = len(a)
			}
		}
		for _, a := range aliases[j].Aliases {
			if len(a) > maxJ {
				maxJ = len(a)
			}
		}
		return maxI > maxJ
	})
	for i := range aliases {
		sort.Slice(aliases[i].Aliases, func(a, b int) bool {
			return len(aliases[i].Aliases[a]) > len(aliases[i].Aliases[b])
		})
	}
}

// Parse converts OCR text into a slice of Nutrient values. Lines that do not
// contain a number are dropped. When a known alias is matched the canonical
// label and unit are used; otherwise the OCR text is title-cased and a
// recognized unit token is required.
func Parse(text string) []Nutrient {
	var out []Nutrient
	for _, raw := range strings.Split(text, "\n") {
		line := normalize(raw)
		if line == "" {
			continue
		}
		matches := numberRE.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			continue
		}

		for _, m := range matches {
			label, amount, unit := extract(line, m)
			if label == "" {
				continue
			}

			canonical, canonicalUnit, matched := matchAlias(line)
			if matched {
				label = canonical
				if canonicalUnit != "" {
					unit = canonicalUnit
				}
			} else if unit == "" {
				// Unknown nutrients must carry a recognized unit token.
				continue
			}

			out = append(out, Nutrient{Label: label, Amount: amount, Unit: unit})
			break
		}
	}
	return out
}

func normalize(s string) string {
	s = strings.ReplaceAll(s, "\u00b5g", "mcg") // µg -> mcg
	s = noiseRE.ReplaceAllString(s, " ")
	s = spaceRE.ReplaceAllString(strings.TrimSpace(s), " ")
	return strings.ToLower(s)
}

func extract(line string, m []int) (string, float64, string) {
	fullStart, fullEnd := m[0], m[1]
	numStart, numEnd := m[2], m[3]
	unitStart, unitEnd := -1, -1
	if len(m) >= 6 {
		unitStart, unitEnd = m[4], m[5]
	}

	labelPart := strings.TrimSpace(line[:fullStart])
	// If the number starts the line, try to use the text after it as the
	// label (e.g. "120 Calories").
	if labelPart == "" {
		after := strings.TrimSpace(line[fullEnd:])
		if after != "" {
			labelPart = after
		}
	}

	numStr := line[numStart:numEnd]
	amount, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return "", 0, ""
	}

	unit := ""
	if unitStart >= 0 && unitEnd > unitStart {
		unit = strings.ToLower(line[unitStart:unitEnd])
		if unit == "µg" {
			unit = "mcg"
		}
	}

	return titleCase(labelPart), amount, unit
}

func matchAlias(line string) (string, string, bool) {
	words := strings.Fields(line)
	for _, e := range aliases {
		for _, a := range e.Aliases {
			if containsWords(words, strings.Fields(normalize(a))) {
				return e.Canonical, e.Unit, true
			}
		}
	}
	return "", "", false
}

func containsWords(haystack, needle []string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if w == "" {
			continue
		}
		r := []rune(w)
		r[0] = unicodeToTitle(r[0])
		for j := 1; j < len(r); j++ {
			r[j] = unicodeToLower(r[j])
		}
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

func unicodeToTitle(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	return r
}

func unicodeToLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r - 'A' + 'a'
	}
	return r
}
