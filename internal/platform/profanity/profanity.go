// Package profanity provides a simple word-boundary profanity detector.
package profanity

import (
	"regexp"
	"strings"
)

// defaultDenyList is a hardcoded English profanity baseline. It is not
// exhaustive; it is meant to catch obvious cases before and during OCR
// structuring.
var defaultDenyList = []string{
	"ass", "asshole", "bastard", "bitch", "bullshit", "cock", "crap",
	"cunt", "damn", "dick", "douche", "fag", "faggot", "fuck", "fucking",
	"goddamn", "hell", "jackass", "jerk", "nigger", "piss", "pussy",
	"shit", "slut", "tits", "twat", "whore",
}

// Detector checks text against a deny-list using word boundaries.
type Detector struct {
	re *regexp.Regexp
}

// New builds a detector from the default list plus optional extra terms.
// extraTerms should be comma-separated. Empty extra terms are ignored.
func New(extraTerms string) *Detector {
	terms := make([]string, len(defaultDenyList))
	copy(terms, defaultDenyList)
	for _, t := range strings.Split(extraTerms, ",") {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" {
			terms = append(terms, t)
		}
	}
	seen := make(map[string]bool)
	var pattern []string
	for _, t := range terms {
		if seen[t] {
			continue
		}
		seen[t] = true
		pattern = append(pattern, regexp.QuoteMeta(t))
	}
	if len(pattern) == 0 {
		return &Detector{re: nil}
	}
	re := regexp.MustCompile(`(?i)\b(?:` + strings.Join(pattern, "|") + `)\b`)
	return &Detector{re: re}
}

// Check returns true and the matched terms if any deny-list term is found.
func (d *Detector) Check(text string) (bool, []string) {
	if d.re == nil || text == "" {
		return false, nil
	}
	matches := d.re.FindAllString(text, -1)
	if len(matches) == 0 {
		return false, nil
	}
	seen := make(map[string]bool)
	var unique []string
	for _, m := range matches {
		lm := strings.ToLower(m)
		if !seen[lm] {
			seen[lm] = true
			unique = append(unique, lm)
		}
	}
	return true, unique
}
