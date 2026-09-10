package ocrimport

import (
	"fmt"
	"sort"
	"strings"

	"github.com/JRAdams472/LENA2/internal/platform/bffclient"
)

// CatalogSnapshot is an in-memory view of the LENA2 inventory catalog used for
// mapping free-text ingredients and units to canonical catalog rows.
type CatalogSnapshot struct {
	Items       []bffclient.Item
	Ingredients []bffclient.Ingredient
	Units       []bffclient.Unit
	Categories  []bffclient.Category

	itemIndex       map[string][]bffclient.Item
	ingredientIndex map[string][]bffclient.Ingredient
	unitByName      map[string]bffclient.Unit
	unitByAbbr      map[string]bffclient.Unit
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
	DraftItem   DraftItem    `json:"draftItem"`
	ItemID      string       `json:"itemId"`
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
func NewCatalogSnapshot(c *bffclient.Catalog) *CatalogSnapshot {
	s := &CatalogSnapshot{
		Items:           c.Items,
		Ingredients:     c.Ingredients,
		Units:           c.Units,
		Categories:      c.Categories,
		itemIndex:       make(map[string][]bffclient.Item),
		ingredientIndex: make(map[string][]bffclient.Ingredient),
		unitByName:      make(map[string]bffclient.Unit),
		unitByAbbr:      make(map[string]bffclient.Unit),
		unitAliases:     buildUnitAliases(c.Units),
	}

	for _, it := range c.Items {
		key := NormalizeName(it.Name)
		s.itemIndex[key] = append(s.itemIndex[key], it)
	}
	for _, in := range c.Ingredients {
		key := NormalizeName(in.Name)
		s.ingredientIndex[key] = append(s.ingredientIndex[key], in)
	}
	for _, u := range c.Units {
		s.unitByName[strings.ToLower(u.Name)] = u
		if u.Abbreviation != "" {
			s.unitByAbbr[strings.ToLower(u.Abbreviation)] = u
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
func (s *CatalogSnapshot) ResolveUnit(raw string) (bffclient.Unit, bool) {
	if raw == "" {
		// Unquantified ingredients default to "each" if available.
		if u, ok := s.unitByName["each"]; ok {
			return u, true
		}
		return bffclient.Unit{}, false
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

	return bffclient.Unit{}, false
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

	query := NormalizeName(raw)

	// Exact normalized match on item name.
	if items, ok := s.itemIndex[query]; ok && len(items) > 0 {
		it := items[0]
		return MatchResult{
			ItemID:     it.ID,
			ItemName:   it.Name,
			Confidence: 1.0,
			Status:     "accepted",
		}
	}

	// Exact normalized match on ingredient name.
	if ings, ok := s.ingredientIndex[query]; ok && len(ings) > 0 {
		in := ings[0]
		return MatchResult{
			ItemID:      "",
			ItemName:    in.Name,
			Confidence:  1.0,
			Status:      "suggested",
			Notes:       fmt.Sprintf("matched generic ingredient %s; create or choose a catalog item", in.ID),
			Suggestions: []Suggestion{{ID: in.ID, Name: in.Name, Kind: "ingredient", Score: 1.0}},
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

	for _, it := range s.Items {
		score := Similarity(raw, it.Name)
		candidates = append(candidates, scored{it.ID, it.Name, "item", score})
	}
	for _, in := range s.Ingredients {
		score := Similarity(raw, in.Name)
		candidates = append(candidates, scored{in.ID, in.Name, "ingredient", score})
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })

	if len(candidates) > 0 {
		best := candidates[0]
		if best.score >= autoAccept && best.kind == "item" {
			return MatchResult{
				ItemID:     best.id,
				ItemName:   best.name,
				Confidence: best.score,
				Status:     "accepted",
			}
		}

		top := candidates
		if len(top) > 3 {
			top = top[:3]
		}
		suggestions := make([]Suggestion, 0, len(top))
		for _, c := range top {
			suggestions = append(suggestions, Suggestion{ID: c.id, Name: c.name, Kind: c.kind, Score: c.score})
		}
		result.Suggestions = suggestions

		if best.score >= reviewThreshold {
			result.Status = "suggested"
			if best.kind == "item" {
				result.ItemID = best.id
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
	result.Unit = unit.Name
	result.UnitID = unit.ID

	return result
}

func buildUnitAliases(units []bffclient.Unit) map[string]string {
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
		aliases[strings.ToLower(u.Name)] = u.Name
		if u.Abbreviation != "" {
			aliases[strings.ToLower(u.Abbreviation)] = u.Name
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
