package ocrimport

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReviewRecipe_AllResolved(t *testing.T) {
	cases := []struct {
		name  string
		items []MatchResult
		want  bool
	}{
		{
			name:  "empty review is not resolved",
			items: nil,
			want:  false,
		},
		{
			name: "accepted item with item and unit ids resolves",
			items: []MatchResult{
				{ItemID: "10", Unit: "cup", UnitID: "1", Status: "accepted"},
			},
			want: true,
		},
		{
			name: "fuzzy suggested match is not resolved (A3-10)",
			items: []MatchResult{
				{ItemID: "10", Unit: "cup", UnitID: "1", Status: "suggested", Confidence: 0.95},
			},
			want: false,
		},
		{
			name: "suggested match approved by admin resolves",
			items: []MatchResult{
				{ItemID: "10", Unit: "cup", UnitID: "1", Status: "suggested", Approved: true},
			},
			want: true,
		},
		{
			name: "accepted without unit id is not resolved",
			items: []MatchResult{
				{ItemID: "10", Unit: "", UnitID: "", Status: "accepted"},
			},
			want: false,
		},
		{
			name: "accepted without item id is not resolved",
			items: []MatchResult{
				{ItemID: "", Unit: "cup", UnitID: "1", Status: "accepted"},
			},
			want: false,
		},
		{
			name: "one unresolved item fails the whole review",
			items: []MatchResult{
				{ItemID: "10", Unit: "cup", UnitID: "1", Status: "accepted"},
				{ItemID: "", Unit: "cup", UnitID: "1", Status: "unmatched"},
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &ReviewRecipe{Items: tc.items}
			assert.Equal(t, tc.want, r.AllResolved())
		})
	}
}
