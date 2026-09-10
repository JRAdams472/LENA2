package ocrimport

import (
	"regexp"
	"sort"
	"strings"
)

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9 ]+`)
var space = regexp.MustCompile(`\s+`)

// NormalizeName returns a lower-cased, punctuation-stripped, singularized
// string suitable for fuzzy catalog comparison.
func NormalizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphaNum.ReplaceAllString(s, " ")
	s = space.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return singularize(s)
}

// SortedWords normalizes the string, splits it into words, sorts them
// alphabetically and re-joins. This makes word-order independent comparison
// cheap.
func SortedWords(s string) string {
	words := strings.Fields(NormalizeName(s))
	sort.Strings(words)
	return strings.Join(words, " ")
}

func singularize(s string) string {
	// Very small English de-pluralization. Enough for the most common
	// ingredient and unit forms.
	for _, w := range strings.Fields(s) {
		switch {
		case strings.HasSuffix(w, "ies"):
			s = strings.ReplaceAll(s, w, w[:len(w)-3]+"y")
		case strings.HasSuffix(w, "ches"), strings.HasSuffix(w, "shes"), strings.HasSuffix(w, "xes"), strings.HasSuffix(w, "zes"), strings.HasSuffix(w, "oes"):
			s = strings.ReplaceAll(s, w, w[:len(w)-2])
		case strings.HasSuffix(w, "ss"):
			continue
		case strings.HasSuffix(w, "es"):
			s = strings.ReplaceAll(s, w, w[:len(w)-1])
		case strings.HasSuffix(w, "s"):
			s = strings.ReplaceAll(s, w, w[:len(w)-1])
		}
	}
	return s
}

// Similarity returns a score in [0, 1] between two strings. It uses
// Jaro-Winkler on both the original and word-sorted normalized forms and
// returns the higher score.
func Similarity(a, b string) float64 {
	aNorm := NormalizeName(a)
	bNorm := NormalizeName(b)
	s1 := jaroWinkler(aNorm, bNorm, 0.7, 0.1)
	aSorted := SortedWords(a)
	bSorted := SortedWords(b)
	s2 := jaroWinkler(aSorted, bSorted, 0.7, 0.1)
	if s2 > s1 {
		return s2
	}
	return s1
}

// jaroWinkler computes the Jaro-Winkler similarity of two strings.
// boostThreshold is the minimum Jaro score to apply the prefix scaling boost.
// prefixScale is the boost per matching prefix character (typically 0.1).
func jaroWinkler(s1, s2 string, boostThreshold, prefixScale float64) float64 {
	if s1 == s2 {
		return 1.0
	}
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	matchWindow := max(len(s1), len(s2))/2 - 1
	if matchWindow < 0 {
		matchWindow = 0
	}

	s1Matches := make([]bool, len(s1))
	s2Matches := make([]bool, len(s2))

	matches := 0
	transpositions := 0

	for i := range s1 {
		start := i - matchWindow
		if start < 0 {
			start = 0
		}
		end := i + matchWindow + 1
		if end > len(s2) {
			end = len(s2)
		}
		for j := start; j < end; j++ {
			if s2Matches[j] || s1[i] != s2[j] {
				continue
			}
			s1Matches[i] = true
			s2Matches[j] = true
			matches++
			break
		}
	}

	if matches == 0 {
		return 0.0
	}

	k := 0
	for i := range s1 {
		if !s1Matches[i] {
			continue
		}
		for !s2Matches[k] {
			k++
		}
		if s1[i] != s2[k] {
			transpositions++
		}
		k++
	}

	jaro := (float64(matches)/float64(len(s1)) +
		float64(matches)/float64(len(s2)) +
		(float64(matches)-float64(transpositions)/2.0)/float64(matches)) / 3.0

	prefix := 0
	for i := 0; i < min(4, len(s1), len(s2)); i++ {
		if s1[i] == s2[i] {
			prefix++
		} else {
			break
		}
	}

	var jw float64
	if jaro > boostThreshold {
		jw = jaro + float64(prefix)*prefixScale*(1.0-jaro)
	} else {
		jw = jaro
	}
	if jw > 1.0 {
		return 1.0
	}
	if jw < 0.0 {
		return 0.0
	}
	return jw
}
