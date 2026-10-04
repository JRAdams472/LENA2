package ocrimport

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Unicode vulgar fractions OCR may emit instead of "1/2".
var vulgarFractions = map[rune]string{
	'¼': "1/4", '½': "1/2", '¾': "3/4",
	'⅐': "1/7", '⅑': "1/9", '⅒': "1/10",
	'⅓': "1/3", '⅔': "2/3",
	'⅕': "1/5", '⅖': "2/5", '⅗': "3/5", '⅘': "4/5",
	'⅙': "1/6", '⅚': "5/6",
	'⅛': "1/8", '⅜': "3/8", '⅝': "5/8", '⅞': "7/8",
}

// Unit words that can follow a leading quantity. Singulars and common plurals
// plus the printed abbreviations ResolveUnit already aliases.
var unitWords = map[string]bool{
	"cup": true, "cups": true,
	"teaspoon": true, "teaspoons": true, "tsp": true,
	"tablespoon": true, "tablespoons": true, "tbsp": true, "tbs": true,
	"ounce": true, "ounces": true, "oz": true,
	"pound": true, "pounds": true, "lb": true, "lbs": true,
	"gram": true, "grams": true, "g": true,
	"kilogram": true, "kilograms": true, "kg": true,
	"milligram": true, "milligrams": true, "mg": true,
	"milliliter": true, "milliliters": true, "millilitre": true, "millilitres": true, "ml": true,
	"liter": true, "liters": true, "litre": true, "litres": true, "l": true,
	"pint": true, "pints": true, "pt": true,
	"quart": true, "quarts": true, "qt": true,
	"gallon": true, "gallons": true, "gal": true,
	"fluid": true, // consumed only as part of "fluid ounce" / "fl oz"
	"can":   true, "cans": true, "jar": true, "jars": true,
	"package": true, "packages": true, "pkg": true, "packet": true, "packets": true,
	"box": true, "boxes": true, "bag": true, "bags": true,
	"bottle": true, "bottles": true, "carton": true, "cartons": true,
	"tube": true, "tubes": true, "tin": true, "tins": true,
	"clove": true, "cloves": true, "bunch": true, "bunches": true,
	"slice": true, "slices": true, "pinch": true, "pinches": true,
	"dash": true, "dashes": true, "stick": true, "sticks": true,
	"sprig": true, "sprigs": true, "head": true, "heads": true,
	"stalk": true, "stalks": true, "piece": true, "pieces": true,
	"dozen": true, "handful": true, "handfuls": true, "sheet": true, "sheets": true,
	"drop": true, "drops": true, "strip": true, "strips": true,
	"cube": true, "cubes": true, "fillet": true, "fillets": true,
	"scoop": true, "scoops": true, "wedge": true, "wedges": true,
	"shot": true, "shots": true, "each": true, "ea": true,
}

// vagueQtyWords keep "a few carrots" from being read as quantity 1.
var vagueQtyWords = map[string]bool{
	"few": true, "couple": true, "several": true,
	"bit": true, "little": true, "lot": true,
}

// containerWords accept a preceding size qualifier: "1 28-oz can tomatoes".
var containerWords = map[string]bool{
	"can": true, "cans": true, "jar": true, "jars": true,
	"bottle": true, "bottles": true, "package": true, "packages": true,
	"pkg": true, "box": true, "boxes": true, "bag": true, "bags": true,
	"carton": true, "cartons": true, "tube": true, "tubes": true, "tin": true, "tins": true,
}

var (
	// "2-3", "2 – 3" → range, quantity is the lower value.
	rangeRe = regexp.MustCompile(`^(\d+(?:[.,]\d+)?)\s*[-–—]\s*(\d+(?:[.,]\d+)?)\b`)
	// "1 1/2" → mixed number.
	mixedRe = regexp.MustCompile(`^(\d+)\s+(\d+)/(\d+)\b`)
	// "1/2" → fraction.
	fractionRe = regexp.MustCompile(`^(\d+)/(\d+)\b`)
	// "3.5", "2" → number.
	numberRe = regexp.MustCompile(`^(\d+(?:[.,]\d+)?)\b`)
	// "a"/"an"/"one" → 1, unless a vague quantifier follows ("a few").
	wordQtyRe = regexp.MustCompile(`(?i)^(?:a|an|one)\s+`)
	// "28-oz", "14.5 ounce", "750ml" → package-size qualifier.
	sizeRe = regexp.MustCompile(`(?i)^(\d+(?:[.,]\d+)?)\s*-?\s*(fl\.?\s*oz|fluid\s+ounces?|ounces?|oz|pounds?|lbs?|grams?|kilograms?|kg|g|milliliters?|millilitres?|ml|liters?|litres?|l)\b`)
)

// FillMissingQuantity is a deterministic safety net for draft items whose
// ingredient still carries a leading quantity the model failed to split out
// (e.g. "2-3 dried ancho chiles" or "1 28-oz can whole tomatoes"). It only
// runs when the model produced no quantity, and keeps whatever unit the
// model did emit. Extracted ranges and package sizes move to notes so no
// source text is silently dropped.
func FillMissingQuantity(d *DraftItem) {
	if d.Quantity != nil {
		return
	}
	qty, unit, extras, rest := splitLeadingQuantity(d.Ingredient)
	if qty == nil {
		return
	}
	d.Quantity = qty
	if unit != "" && d.Unit == nil {
		u := unit
		d.Unit = &u
	}
	if trimmed := strings.TrimSpace(rest); trimmed != "" {
		d.Ingredient = trimmed
	}
	for _, ex := range extras {
		if d.Notes == nil || *d.Notes == "" {
			n := ex
			d.Notes = &n
		} else {
			*d.Notes += "; " + ex
		}
	}
}

// splitLeadingQuantity parses a leading quantity ("2-3", "1 1/2", "½", "a")
// then an optional package size and unit word. Returns the quantity, unit
// (may be ""), note-worthy extras (ranges, sizes), and the remaining
// ingredient text.
func splitLeadingQuantity(line string) (*float64, string, []string, string) {
	s := normalizeFractions(line)
	s = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '–' || r == '—' {
			return '-'
		}
		return r
	}, s))

	qty, extras, s, ok := leadingQty(s)
	if !ok {
		return nil, "", nil, line
	}

	s = strings.TrimSpace(s)

	// Optional package-size qualifier: "28-oz can", "750ml bottle".
	var unit string
	if m := sizeRe.FindStringSubmatch(s); m != nil {
		rest := strings.TrimSpace(s[len(m[0]):])
		if tok, n := nextToken(rest); containerWords[tok] {
			extras = append(extras, strings.TrimSpace(m[0]))
			unit = tok
			s = rest[n:]
		}
	}

	// Optional unit word ("fluid ounce" counts as one unit). May already be
	// set by the package-size branch.
	s = strings.TrimSpace(s)
	if unit == "" {
		unit, s = unitWord(s)
	}

	return &qty, unit, extras, stripLeadingOf(s)
}

// leadingQty parses the quantity prefix — range, mixed number, fraction,
// decimal, or quantity word. ok=false means no quantity was present (the
// caller returns the original line as the ingredient text).
func leadingQty(s string) (qty float64, extras []string, rest string, ok bool) {
	if m := rangeRe.FindStringSubmatch(s); m != nil {
		return parseDecimal(m[1]), []string{fmt.Sprintf("range %s-%s", m[1], m[2])}, s[len(m[0]):], true
	}
	if m := mixedRe.FindStringSubmatch(s); m != nil {
		whole := parseDecimal(m[1])
		num := parseDecimal(m[2])
		den := parseDecimal(m[3])
		if den == 0 {
			return 0, nil, "", false
		}
		return whole + num/den, nil, s[len(m[0]):], true
	}
	if m := fractionRe.FindStringSubmatch(s); m != nil {
		num := parseDecimal(m[1])
		den := parseDecimal(m[2])
		if den == 0 {
			return 0, nil, "", false
		}
		return num / den, nil, s[len(m[0]):], true
	}
	if m := numberRe.FindStringSubmatch(s); m != nil {
		return parseDecimal(m[1]), nil, s[len(m[0]):], true
	}
	if m := wordQtyRe.FindStringSubmatch(s); m != nil {
		rest := strings.TrimSpace(s[len(m[0]):])
		if tok, _ := nextToken(rest); vagueQtyWords[tok] {
			return 0, nil, "", false // "a few carrots" — not a quantity
		}
		return 1, nil, s[len(m[0]):], true
	}
	return 0, nil, "", false
}

// unitWord consumes a unit token ("fl oz"/"fluid ounce" count as one
// unit), returning the unit and the remaining text.
func unitWord(s string) (unit, rest string) {
	tok, n := nextToken(s)
	switch {
	case tok == "fl":
		if r := strings.TrimSpace(s[n:]); len(r) > 0 {
			if t2, n2 := nextToken(r); t2 == "oz" || t2 == "ounce" || t2 == "ounces" {
				return "fl oz", r[n2:]
			}
		}
	case tok == "fluid":
		if r := strings.TrimSpace(s[n:]); len(r) > 0 {
			if t2, n2 := nextToken(r); t2 == "ounce" || t2 == "ounces" {
				return "fluid ounce", r[n2:]
			}
		}
	case unitWords[tok]:
		return tok, s[n:]
	}
	return "", s
}

// stripLeadingOf drops a leading "of" — "1 can of tomatoes".
func stripLeadingOf(s string) string {
	rest := strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(rest), "of ") {
		rest = strings.TrimSpace(rest[3:])
	}
	return rest
}

// nextToken returns the first space-or-punctuation-delimited word, lowercased
// with a trailing period stripped, and its consumed length.
func nextToken(s string) (string, int) {
	i := 0
	for i < len(s) {
		c := s[i]
		if c == ' ' || c == '\t' || c == ',' || c == ';' || c == '(' {
			break
		}
		i++
	}
	return strings.TrimSuffix(strings.ToLower(s[:i]), "."), i
}

func parseDecimal(s string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	return f
}

// normalizeFractions rewrites unicode vulgar fractions as ASCII: "1½" and
// "½" become "1 1/2" and "1/2" so the quantity regexes see one shape.
// Fields+Join collapses the inserted and original whitespace.
func normalizeFractions(s string) string {
	var b strings.Builder
	for _, r := range s {
		if f, ok := vulgarFractions[r]; ok {
			b.WriteByte(' ')
			b.WriteString(f)
			continue
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
