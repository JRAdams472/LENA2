// Package ocrimport supports the recipe OCR import pipeline: draft types,
// catalog matching, and the admin review model.
package ocrimport

import (
	"fmt"
	"sort"
	"strings"
)

// CatalogItem is a canonical catalog row used for ingredient matching.
type CatalogItem interface {
	ID() string
	Name() string
}

// CatalogIngredient is a generic ingredient used for fuzzy suggestions.
type CatalogIngredient interface {
	ID() string
	Name() string
}

// CatalogUnit is a canonical unit of measure.
type CatalogUnit interface {
	ID() string
	Name() string
	Abbreviation() string
}

// CatalogCategory is a catalog category used only for future enrichment.
type CatalogCategory interface {
	ID() string
	Name() string
}

// Catalog provides a snapshot of the inventory catalog to CatalogSnapshot.
type Catalog interface {
	Items() []CatalogItem
	Ingredients() []CatalogIngredient
	Units() []CatalogUnit
	Categories() []CatalogCategory
}

// StaticCatalog is a simple in-memory Catalog used by tests and adapters.
type StaticCatalog struct {
	ItemsField       []CatalogItem
	IngredientsField []CatalogIngredient
	UnitsField       []CatalogUnit
	CategoriesField  []CatalogCategory
}

// Items returns the catalog items.
func (c *StaticCatalog) Items() []CatalogItem { return c.ItemsField }

// Ingredients returns the catalog ingredients.
func (c *StaticCatalog) Ingredients() []CatalogIngredient { return c.IngredientsField }

// Units returns the catalog units.
func (c *StaticCatalog) Units() []CatalogUnit { return c.UnitsField }

// Categories returns the catalog categories.
func (c *StaticCatalog) Categories() []CatalogCategory { return c.CategoriesField }

// CatalogSnapshot is an in-memory view of the LENA2 inventory catalog used for
// mapping free-text ingredients and units to canonical catalog rows.
type CatalogSnapshot struct {
	Items       []CatalogItem
	Ingredients []CatalogIngredient
	Units       []CatalogUnit
	Categories  []CatalogCategory

	itemIndex       map[string][]CatalogItem
	ingredientIndex map[string][]CatalogIngredient
	unitByName      map[string]CatalogUnit
	unitByAbbr      map[string]CatalogUnit
	unitAliases     map[string]string // raw normalized form -> canonical unit name
}

// Suggestion is a candidate catalog match for an OCR'd ingredient.
type Suggestion struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Kind  string  `json:"kind"` // "item" or "ingredient"
	Score float64 `json:"score"`
}

// MatchResult is the outcome of mapping one DraftItem against the catalog.
type MatchResult struct {
	DraftItem DraftItem `json:"draftItem"`
	ItemID    string    `json:"itemId"`
	// ItemKind records which catalog the selected ItemID belongs to:
	// "item" (branded product) or "ingredient" (generic). Empty means
	// "item" — reviews saved before the field existed only ever held items.
	ItemKind    string       `json:"itemKind,omitempty"`
	ItemName    string       `json:"itemName"`
	Unit        string       `json:"unit"`
	UnitID      string       `json:"unitId"`
	Confidence  float64      `json:"confidence"`
	Suggestions []Suggestion `json:"suggestions"`
	Status      string       `json:"status"` // "accepted", "suggested", "unmatched"
	Notes       string       `json:"notes"`
	Approved    bool         `json:"approved"`
}

// NewCatalogSnapshot builds a snapshot with normalized lookup indexes.
func NewCatalogSnapshot(catalog Catalog) *CatalogSnapshot {
	items := catalog.Items()
	ingredients := catalog.Ingredients()
	units := catalog.Units()
	categories := catalog.Categories()

	s := &CatalogSnapshot{
		Items:           items,
		Ingredients:     ingredients,
		Units:           units,
		Categories:      categories,
		itemIndex:       make(map[string][]CatalogItem),
		ingredientIndex: make(map[string][]CatalogIngredient),
		unitByName:      make(map[string]CatalogUnit),
		unitByAbbr:      make(map[string]CatalogUnit),
		unitAliases:     buildUnitAliases(units),
	}

	for _, it := range items {
		key := NormalizeName(it.Name())
		s.itemIndex[key] = append(s.itemIndex[key], it)
	}
	for _, in := range ingredients {
		key := NormalizeName(in.Name())
		s.ingredientIndex[key] = append(s.ingredientIndex[key], in)
	}
	for _, u := range units {
		s.unitByName[strings.ToLower(u.Name())] = u
		if u.Abbreviation() != "" {
			s.unitByAbbr[strings.ToLower(u.Abbreviation())] = u
		}
	}
	return s
}

// UnitCount returns the number of units in the snapshot.
func (s *CatalogSnapshot) UnitCount() int { return len(s.Units) }

// ItemCount returns the number of items in the snapshot.
func (s *CatalogSnapshot) ItemCount() int { return len(s.Items) }

// ResolveUnit turns a raw unit string into a canonical catalog unit. It uses a
// static alias table and then matches lower-cased name or abbreviation.
func (s *CatalogSnapshot) ResolveUnit(raw string) (CatalogUnit, bool) {
	if raw == "" {
		// Unquantified ingredients default to "each" if available.
		if u, ok := s.unitByName["each"]; ok {
			return u, true
		}
		return nil, false
	}

	clean := strings.ToLower(strings.TrimSpace(raw))
	clean = strings.TrimSuffix(clean, ".")

	// Static aliases for common printed forms.
	if canonical, ok := s.unitAliases[clean]; ok {
		clean = canonical
	}

	// Direct lookup by name or abbreviation.
	if u, ok := s.unitByName[clean]; ok {
		return u, true
	}
	if u, ok := s.unitByAbbr[clean]; ok {
		return u, true
	}

	// Try simple de-pluralized version.
	sing := singularize(clean)
	if sing != clean {
		if u, ok := s.unitByName[sing]; ok {
			return u, true
		}
		if u, ok := s.unitByAbbr[sing]; ok {
			return u, true
		}
	}

	return nil, false
}

// ingredientBoost nudges generic-ingredient candidates above same-scored
// branded SKUs when ranking suggestions. Recipes want the generic item
// ("water", "salt") more often than a specific product.
const ingredientBoost = 0.05

// matchStopwords are tokens that carry no matching signal even at length.
var matchStopwords = map[string]bool{
	"and": true, "the": true, "for": true, "with": true, "per": true,
	"fresh": true, "dried": true, "chopped": true, "minced": true, "sliced": true,
}

// contentWords returns the normalized tokens that are meaningful for matching:
// at least 3 chars, not purely numeric, and not a catalog unit (name,
// abbreviation, or alias) or generic cooking verb/stopword.
func (s *CatalogSnapshot) contentWords(str string) map[string]bool {
	out := make(map[string]bool)
	for _, w := range strings.Fields(NormalizeName(str)) {
		if len(w) < 3 || isNumericToken(w) || matchStopwords[w] {
			continue
		}
		if _, ok := s.unitByName[w]; ok {
			continue
		}
		if _, ok := s.unitByAbbr[w]; ok {
			continue
		}
		if _, ok := s.unitAliases[w]; ok {
			continue
		}
		out[w] = true
	}
	return out
}

func isNumericToken(w string) bool {
	for _, r := range w {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(w) > 0
}

// sharesContentWord reports whether the candidate name shares at least one
// content word with the query words. Jaro-Winkler is character-level, so
// without this gate "2-3 dried ancho chiles" scores 0.84 against
// "Strudel Apple Mini 3.2 Oz" purely on shared digits and short tokens.
func (s *CatalogSnapshot) sharesContentWord(queryWords map[string]bool, name string) bool {
	if len(queryWords) == 0 {
		return true // nothing to gate on — fail open
	}
	for w := range s.contentWords(name) {
		if queryWords[w] {
			return true
		}
	}
	return false
}

// MatchItem maps a raw ingredient string to the catalog. It returns the best
// result along with suggestions and a status string.
func (s *CatalogSnapshot) MatchItem(raw string, autoAccept, reviewThreshold float64) MatchResult {
	result := MatchResult{
		Status: "unmatched",
	}

	if strings.TrimSpace(raw) == "" {
		result.Notes = "empty ingredient"
		return result
	}

	// Score the bare ingredient noun phrase — a leading quantity/unit left
	// over from a dirty draft line only feeds noise into the fuzzy sweep.
	matchText := raw
	if qty, _, _, rest := splitLeadingQuantity(raw); qty != nil && strings.TrimSpace(rest) != "" {
		matchText = rest
	}

	query := NormalizeName(matchText)

	// Exact normalized match on item name.
	if items, ok := s.itemIndex[query]; ok && len(items) > 0 {
		it := items[0]
		return MatchResult{
			ItemID:     it.ID(),
			ItemKind:   "item",
			ItemName:   it.Name(),
			Confidence: 1.0,
			Status:     "accepted",
		}
	}

	// Exact normalized match on ingredient name.
	if ings, ok := s.ingredientIndex[query]; ok && len(ings) > 0 {
		in := ings[0]
		return MatchResult{
			ItemID:      "",
			ItemName:    in.Name(),
			Confidence:  1.0,
			Status:      "suggested",
			Notes:       fmt.Sprintf("matched generic ingredient %s; create or choose a catalog item", in.ID()),
			Suggestions: []Suggestion{{ID: in.ID(), Name: in.Name(), Kind: "ingredient", Score: 1.0}},
		}
	}

	// Fuzzy sweep over items and ingredients.
	type scored struct {
		id    string
		name  string
		kind  string
		score float64
	}
	candidates := make([]scored, 0, len(s.Items)+len(s.Ingredients))
	queryWords := s.contentWords(matchText)

	for _, it := range s.Items {
		if !s.sharesContentWord(queryWords, it.Name()) {
			continue
		}
		score := Similarity(matchText, it.Name())
		candidates = append(candidates, scored{it.ID(), it.Name(), "item", score})
	}
	for _, in := range s.Ingredients {
		if !s.sharesContentWord(queryWords, in.Name()) {
			continue
		}
		score := Similarity(matchText, in.Name())
		candidates = append(candidates, scored{in.ID(), in.Name(), "ingredient", score})
	}

	// Rank by score, with a small ingredient-kind preference so generic
	// ingredients outrank same-scored branded products.
	sort.Slice(candidates, func(i, j int) bool {
		si, sj := candidates[i].score, candidates[j].score
		if candidates[i].kind == "ingredient" {
			si += ingredientBoost
		}
		if candidates[j].kind == "ingredient" {
			sj += ingredientBoost
		}
		return si > sj
	})

	if len(candidates) > 0 {
		best := candidates[0]
		if best.score >= autoAccept && best.kind == "item" {
			return MatchResult{
				ItemID:     best.id,
				ItemKind:   "item",
				ItemName:   best.name,
				Confidence: best.score,
				Status:     "accepted",
			}
		}

		suggestions := make([]Suggestion, 0, 3)
		for _, c := range candidates {
			if c.score < reviewThreshold {
				break
			}
			if len(suggestions) >= 3 {
				break
			}
			suggestions = append(suggestions, Suggestion{ID: c.id, Name: c.name, Kind: c.kind, Score: c.score})
		}
		result.Suggestions = suggestions

		if best.score >= reviewThreshold {
			result.Status = "suggested"
			if best.kind == "item" {
				result.ItemID = best.id
				result.ItemKind = "item"
				result.ItemName = best.name
				result.Confidence = best.score
			} else {
				result.Notes = fmt.Sprintf("matched generic ingredient %s; create or choose a catalog item", best.id)
			}
		} else {
			result.Status = "unmatched"
		}
	}

	return result
}

// MapDraftItem combines unit normalization and ingredient matching into a
// single MatchResult. The DraftItem is embedded for round-tripping.
func (s *CatalogSnapshot) MapDraftItem(d DraftItem, autoAccept, reviewThreshold float64) MatchResult {
	result := s.MatchItem(d.Ingredient, autoAccept, reviewThreshold)
	result.DraftItem = d

	unit, ok := s.ResolveUnit(derefString(d.Unit))
	if !ok {
		result.Status = "suggested"
		if result.Notes != "" {
			result.Notes += "; "
		}
		result.Notes += fmt.Sprintf("unknown unit %q", derefString(d.Unit))
		return result
	}
	result.Unit = unit.Name()
	result.UnitID = unit.ID()

	return result
}

func buildUnitAliases(units []CatalogUnit) map[string]string {
	aliases := map[string]string{
		"tbs":   "tablespoon",
		"tbsp":  "tablespoon",
		"tsp":   "teaspoon",
		"oz":    "ounce",
		"fl oz": "fluid ounce",
		"lb":    "pound",
		"lbs":   "pound",
		"g":     "gram",
		"kg":    "kilogram",
		"ml":    "milliliter",
		"l":     "liter",
		"pt":    "pint",
		"qt":    "quart",
		"gal":   "gallon",
		"ea":    "each",
		"pkg":   "package",
	}

	for _, u := range units {
		aliases[strings.ToLower(u.Name())] = u.Name()
		if u.Abbreviation() != "" {
			aliases[strings.ToLower(u.Abbreviation())] = u.Name()
		}
	}

	return aliases
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
