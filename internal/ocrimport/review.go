package ocrimport

// ReviewRecipe holds the mapping decisions for one draft recipe. It is the
// interface between the LLM draft and the admin review step.
type ReviewRecipe struct {
	PageID      string        `json:"pageId"`
	Name        string        `json:"name"`
	Description *string       `json:"description,omitempty"`
	Servings    *int          `json:"servings,omitempty"`
	PrepTime    *int          `json:"prepTimeMinutes,omitempty"`
	CookTime    *int          `json:"cookTimeMinutes,omitempty"`
	SourceHint  *string       `json:"sourceHint,omitempty"`
	Items       []MatchResult `json:"items"`
	Steps       []DraftStep   `json:"steps"`
	Approved    bool          `json:"approved"`
}

// AllResolved reports whether every item has been explicitly resolved to a
// catalog item and unit. A fuzzy "suggested" match does not count: the item
// must have been auto-accepted above the accept threshold or approved by an
// admin, and it must carry both an itemId and a unitId.
func (r *ReviewRecipe) AllResolved() bool {
	if len(r.Items) == 0 {
		return false
	}
	for _, it := range r.Items {
		if it.Status != "accepted" && !it.Approved {
			return false
		}
		if it.ItemID == "" || it.UnitID == "" {
			return false
		}
	}
	return true
}
