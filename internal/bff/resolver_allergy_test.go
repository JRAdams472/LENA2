package bff

import (
	"context"
	"errors"
	"testing"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/testutil"
	"github.com/JRAdams472/LENA2/internal/userprefs"
)

func allergyTestCtx() context.Context {
	return testutil.WithHousehold(context.Background(), 11, 7, "member@example.com")
}

func ingIDPtr(v int64) *int64 { return &v }

func testAllergen(id int64, name string) inventory.Allergen {
	return inventory.Allergen{AllergenID: id, Name: name, IsActive: true}
}

// testAllergyContext builds a context with two ingredients (1 = catalog
// link, 2 = household override), one item, one flagged allergen set, and
// two members with different record kinds.
func testAllergyContext() *allergyContext {
	return &allergyContext{
		resolved: map[int64]*int64{10: ingIDPtr(2)},
		items: map[int64]inventory.Item{
			10: {ItemID: 10, IngredientID: ingIDPtr(1)},
		},
		flagsByIngredient: map[int64][]entityFlag{
			1: {{allergenID: 100, kind: inventory.AllergenFlagContains}},
			2: {{allergenID: 200, kind: inventory.AllergenFlagMayContain}},
		},
		flagsByItem: map[int64][]entityFlag{
			10: {{allergenID: 300, kind: inventory.AllergenFlagContains}},
		},
		allergens: map[int64]inventory.Allergen{
			100: testAllergen(100, "peanuts"),
			200: testAllergen(200, "milk"),
			300: testAllergen(300, "eggs"),
		},
		members: map[int64]identity.User{
			11: {UserID: 11, DisplayName: "alice"},
			12: {UserID: 12, DisplayName: "bob"},
		},
		records: []userprefs.UserAllergen{
			{UserID: 11, AllergenID: 200, Kind: userprefs.MemberAllergyKindAllergy},
			{UserID: 12, AllergenID: 300, Kind: userprefs.MemberAllergyKindDietary},
		},
	}
}

// Household override wins over the catalog link; item flags union in.
func TestAllergyContextItemSetResolutionOrder(t *testing.T) {
	ac := testAllergyContext()
	set := ac.itemSet(inventory.Item{ItemID: 10, IngredientID: ingIDPtr(1)})

	// Override ingredient 2's flag, not catalog ingredient 1's.
	assert.Equal(t, inventory.AllergenFlagMayContain, set[200])
	assert.Equal(t, inventory.AllergenFlagContains, set[300])
	_, hasCatalog := set[100]
	assert.False(t, hasCatalog, "catalog ingredient flags must not leak when an override remaps the item")
}

// Without an override the catalog link applies; an unlinked item yields
// only its product flags.
func TestAllergyContextItemSetCatalogFallback(t *testing.T) {
	ac := testAllergyContext()
	ac.resolved = map[int64]*int64{10: ingIDPtr(1)}

	set := ac.itemSet(inventory.Item{ItemID: 10, IngredientID: ingIDPtr(1)})
	assert.Equal(t, inventory.AllergenFlagContains, set[100])
	assert.Equal(t, inventory.AllergenFlagContains, set[300])
	_, hasOverride := set[200]
	assert.False(t, hasOverride)

	// Item absent from the resolved map falls back to its catalog link.
	delete(ac.resolved, 10)
	set = ac.itemSet(inventory.Item{ItemID: 10, IngredientID: ingIDPtr(1)})
	assert.Equal(t, inventory.AllergenFlagContains, set[100])

	// Unlinked item: product flags only. The context's item map is the
	// catalog-link fallback, so an item with no link anywhere resolves no
	// ingredient.
	delete(ac.items, 10)
	set = ac.itemSet(inventory.Item{ItemID: 10})
	assert.Equal(t, map[int64]string{300: inventory.AllergenFlagContains}, set)
}

// A line carrying both an explicit ingredient and a bound item unions all
// three sources: the line ingredient, the item flags, and the item's
// resolved ingredient.
func TestAllergyContextLineSetUnion(t *testing.T) {
	ac := testAllergyContext()
	set := ac.lineSet(ingIDPtr(1), ingIDPtr(10))
	assert.Equal(t, inventory.AllergenFlagContains, set[100])
	assert.Equal(t, inventory.AllergenFlagMayContain, set[200])
	assert.Equal(t, inventory.AllergenFlagContains, set[300])
}

// contains outranks may_contain when both kinds flag the same allergen.
func TestAllergyContextKindPrecedence(t *testing.T) {
	ac := &allergyContext{
		flagsByIngredient: map[int64][]entityFlag{
			1: {{allergenID: 100, kind: inventory.AllergenFlagMayContain}},
		},
		flagsByItem: map[int64][]entityFlag{
			10: {{allergenID: 100, kind: inventory.AllergenFlagContains}},
		},
		resolved:  map[int64]*int64{10: ingIDPtr(1)},
		items:     map[int64]inventory.Item{},
		allergens: map[int64]inventory.Allergen{},
		members:   map[int64]identity.User{},
	}
	set := ac.lineSet(ingIDPtr(1), ingIDPtr(10))
	assert.Equal(t, inventory.AllergenFlagContains, set[100])
}

// Warnings name the conflicting member and carry both record and entity
// kinds — the shape the clients render.
func TestAllergyWarningShapeAndMemberLabeling(t *testing.T) {
	ac := testAllergyContext()
	set := map[int64]string{
		200: inventory.AllergenFlagMayContain,
		300: inventory.AllergenFlagContains,
	}
	warnings := ac.warningResolvers(set)
	require.Len(t, warnings, 2)

	// Sorted by member display name: alice first.
	assert.Equal(t, "alice", *warnings[0].Member().DisplayName())
	assert.Equal(t, "milk", warnings[0].Allergen().Name())
	assert.Equal(t, "allergy", warnings[0].MemberKind())
	assert.Equal(t, "may_contain", warnings[0].EntityKind())

	assert.Equal(t, "bob", *warnings[1].Member().DisplayName())
	assert.Equal(t, "eggs", warnings[1].Allergen().Name())
	assert.Equal(t, "dietary", warnings[1].MemberKind())
	assert.Equal(t, "contains", warnings[1].EntityKind())
}

// No member records → no warnings, never silently "safe" data loss: the
// allergen flags still resolve for display.
func TestAllergyWarningsNoRecords(t *testing.T) {
	ac := testAllergyContext()
	ac.records = nil
	set := ac.itemSet(inventory.Item{ItemID: 10})
	assert.Empty(t, ac.warningResolvers(set))
	assert.Len(t, ac.flagResolvers(set), 2, "flags still render when no member has records")
}

// loadAllergyContext batches: flag maps, registry rows, members, records —
// and includes the record-referenced allergens so labels resolve.
func TestLoadAllergyContextBatches(t *testing.T) {
	ctrl := gomock.NewController(t)
	inv := mock.NewMockItemReader(ctrl)
	ids := mock.NewMockIdentityService(ctrl)
	up := mock.NewMockUserPrefsService(ctrl)

	inv.EXPECT().ResolveItemIngredients(gomock.Any(), int64(7), []int64{10}).Return(map[int64]*int64{10: ingIDPtr(2)}, nil)
	inv.EXPECT().GetItemsByIDs(gomock.Any(), []int64{10}).Return([]inventory.Item{{ItemID: 10, IngredientID: ingIDPtr(1)}}, nil)
	inv.EXPECT().ListIngredientAllergensByIngredients(gomock.Any(), gomock.Any()).Return(map[int64][]inventory.EntityAllergen{
		2: {{AllergenID: 200, Kind: inventory.AllergenFlagContains}},
	}, nil)
	inv.EXPECT().ListItemAllergensByItems(gomock.Any(), []int64{10}).Return(map[int64][]inventory.EntityAllergen{
		10: {{AllergenID: 300, Kind: inventory.AllergenFlagMayContain}},
	}, nil)
	ids.EXPECT().ListUsersByHousehold(gomock.Any(), int64(7)).Return([]identity.User{
		{UserID: 11, DisplayName: "alice"},
	}, nil)
	up.EXPECT().ListUserAllergensByUsers(gomock.Any(), []int64{11}).Return([]userprefs.UserAllergen{
		{UserID: 11, AllergenID: 200, Kind: userprefs.MemberAllergyKindAllergy},
	}, nil)
	inv.EXPECT().GetAllergensByIDs(gomock.Any(), gomock.Any()).Return(map[int64]inventory.Allergen{
		200: testAllergen(200, "milk"),
		300: testAllergen(300, "eggs"),
	}, nil)

	src := &allergySource{inv: inv, id: ids, up: up, householdID: 7}
	ac, err := src.loadAllergyContext(context.Background(), nil, []int64{10}, nil)
	require.NoError(t, err)

	require.Equal(t, int64(2), *ac.resolved[10])
	require.Len(t, ac.flagsByIngredient[2], 1)
	require.Len(t, ac.flagsByItem[10], 1)
	require.Len(t, ac.members, 1)
	require.Len(t, ac.records, 1)
	assert.Contains(t, ac.allergens, int64(200))
	assert.Contains(t, ac.allergens, int64(300))
}

// Admin/catalog scope (householdID 0 or nil member services) loads flags
// with no member queries — warnings yield an empty list, flags still work.
func TestLoadAllergyContextNoHousehold(t *testing.T) {
	ctrl := gomock.NewController(t)
	inv := mock.NewMockItemReader(ctrl)
	// No EXPECT on identity/userprefs — any call would fail the test.

	inv.EXPECT().ListIngredientAllergensByIngredients(gomock.Any(), gomock.Any()).Return(map[int64][]inventory.EntityAllergen{
		2: {{AllergenID: 200, Kind: inventory.AllergenFlagContains}},
	}, nil)
	inv.EXPECT().GetAllergensByIDs(gomock.Any(), gomock.Any()).Return(map[int64]inventory.Allergen{
		200: testAllergen(200, "milk"),
	}, nil)

	src := &allergySource{inv: inv, householdID: 0}
	ac, err := src.loadAllergyContext(context.Background(), []int64{2}, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, ac.members)
	assert.Empty(t, ac.records)
	assert.Len(t, ac.flagsByIngredient[2], 1)
	assert.Empty(t, ac.warningResolvers(ac.ingredientSet(2)))
}

// A resolver built without preload or source errors on warnings rather
// than silently reporting "no conflicts".
func TestItemAllergyWarningsMissingContext(t *testing.T) {
	r := &itemResolver{it: inventory.Item{ItemID: 10}}
	_, err := r.Allergens(allergyTestCtx())
	assert.Error(t, err)
	_, err = r.AllergyWarnings(allergyTestCtx())
	assert.Error(t, err)
}

// The item field resolvers read the preloaded context — flags union the
// resolved ingredient with product flags, and warnings label members.
func TestItemAllergenFieldsFromPreload(t *testing.T) {
	ac := testAllergyContext()
	r := &itemResolver{it: inventory.Item{ItemID: 10, IngredientID: ingIDPtr(1)}, ch: &itemChildren{ac: ac}}

	flags, err := r.Allergens(allergyTestCtx())
	require.NoError(t, err)
	require.Len(t, flags, 2)
	assert.Equal(t, "eggs", flags[0].Allergen().Name())
	assert.Equal(t, "contains", flags[0].Kind())
	assert.Equal(t, "milk", flags[1].Allergen().Name())
	assert.Equal(t, "may_contain", flags[1].Kind())

	warnings, err := r.AllergyWarnings(allergyTestCtx())
	require.NoError(t, err)
	require.Len(t, warnings, 2)
	assert.Equal(t, "alice", *warnings[0].Member().DisplayName())
	assert.Equal(t, "bob", *warnings[1].Member().DisplayName())
}

// Recipes union every line's resolved flags — an ingredient-keyed line
// contributes its flags even with no item bound.
func TestRecipeAllergenUnion(t *testing.T) {
	ac := testAllergyContext()
	r := &recipeResolver{
		rc: &recipeChildren{
			itemChildren: &itemChildren{ac: ac},
			itemsBy: map[int64][]recipe.RecipeItem{
				5: {
					{IngredientID: ingIDPtr(1)},                       // ingredient-keyed line
					{ItemID: ingIDPtr(10), IngredientID: ingIDPtr(2)}, // item line (override)
					{Notes: "handful of parsley"},                     // manual line — no flags
				},
			},
		},
		recipe: recipe.Recipe{RecipeID: 5},
	}

	flags, err := r.Allergens(allergyTestCtx())
	require.NoError(t, err)
	require.Len(t, flags, 3)
	names := []string{flags[0].Allergen().Name(), flags[1].Allergen().Name(), flags[2].Allergen().Name()}
	assert.Equal(t, []string{"eggs", "milk", "peanuts"}, names)
}

// A slot with no linked recipe and no slot items produces an empty
// warning list — not an error and not a fabricated conflict.
func TestMealSlotAllergyWarningsEmpty(t *testing.T) {
	ac := testAllergyContext()
	r := &mealSlotResolver{
		slot:  mealplan.MealSlot{SlotID: 3},
		items: nil,
		rc:    &recipeChildren{itemChildren: &itemChildren{ac: ac}},
	}
	warnings, err := r.AllergyWarnings(allergyTestCtx())
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

// A grocery line's set merges its explicit ingredient, its bound item's
// product flags, and the item's override-resolved ingredient.
func TestGroceryLineAllergenSources(t *testing.T) {
	ac := testAllergyContext()
	r := &groceryListItemResolver{
		ch:   &itemChildren{ac: ac},
		item: grocery.GroceryListItem{GroceryListItemID: 9, IngredientID: ingIDPtr(1), ItemID: ingIDPtr(10)},
	}
	flags, err := r.Allergens(allergyTestCtx())
	require.NoError(t, err)
	require.Len(t, flags, 3, "line ingredient + item flags + item's resolved ingredient")

	warnings, err := r.AllergyWarnings(allergyTestCtx())
	require.NoError(t, err)
	require.Len(t, warnings, 2)
	assert.Equal(t, "alice", *warnings[0].Member().DisplayName())
	assert.Equal(t, "bob", *warnings[1].Member().DisplayName())
}

// myAllergies returns the caller's records only.
func TestMyAllergies(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	inv := mock.NewMockInventoryService(ctrl)

	up.EXPECT().ListUserAllergens(gomock.Any(), int64(11)).Return([]userprefs.UserAllergen{
		{UserID: 11, AllergenID: 200, Kind: userprefs.MemberAllergyKindDietary},
	}, nil)
	inv.EXPECT().GetAllergensByIDs(gomock.Any(), []int64{200}).Return(map[int64]inventory.Allergen{
		200: testAllergen(200, "milk"),
	}, nil)

	r := &Resolver{UserPrefsService: up, InventoryService: inv}
	recs, err := r.MyAllergies(allergyTestCtx())
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, "milk", recs[0].Allergen().Name())
	assert.Equal(t, "dietary", recs[0].Kind())
}

// setMyAllergy writes under the caller's identity; the service validates
// the kind and its error propagates. on=false clears the record.
func TestSetMyAllergy(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	up.EXPECT().SetUserAllergen(gomock.Any(), int64(11), int64(200), userprefs.MemberAllergyKindAllergy, "member@example.com").Return(nil)
	ok, err := r.SetMyAllergy(allergyTestCtx(), struct {
		AllergenID graphql.ID
		Kind       string
		On         bool
	}{AllergenID: graphql.ID("200"), Kind: "allergy", On: true})
	require.NoError(t, err)
	assert.True(t, ok)

	// Invalid kind: the service rejects and the error surfaces.
	up.EXPECT().SetUserAllergen(gomock.Any(), int64(11), int64(200), "sometimes", "member@example.com").Return(errors.New("invalid kind"))
	_, err = r.SetMyAllergy(allergyTestCtx(), struct {
		AllergenID graphql.ID
		Kind       string
		On         bool
	}{AllergenID: graphql.ID("200"), Kind: "sometimes", On: true})
	assert.Error(t, err)

	// on=false clears the record.
	up.EXPECT().ClearUserAllergen(gomock.Any(), int64(11), int64(200)).Return(nil)
	ok, err = r.SetMyAllergy(allergyTestCtx(), struct {
		AllergenID graphql.ID
		Kind       string
		On         bool
	}{AllergenID: graphql.ID("200"), Kind: "allergy", On: false})
	require.NoError(t, err)
	assert.True(t, ok)
}
