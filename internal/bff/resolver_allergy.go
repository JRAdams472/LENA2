package bff

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"

	"github.com/graph-gophers/graphql-go"

	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/userprefs"
)

// entityFlag is one allergen flag on an entity: the allergen row id plus
// whether it is a confirmed "contains" or a "may_contain" advisory.
type entityFlag struct {
	allergenID int64
	kind       string
}

// allergyContext carries everything needed to render an entity's allergen
// flags and compute household warnings without issuing a query per row:
// the batch-loaded flag maps, the allergen registry rows, the household
// member roster, and the members' allergy/dietary records.
//
// An empty flag set means "no allergen data" — never "known safe". An
// empty record list means no member reported anything (including the
// no-household and not-loaded cases).
type allergyContext struct {
	resolved          map[int64]*int64
	items             map[int64]inventory.Item
	flagsByIngredient map[int64][]entityFlag
	flagsByItem       map[int64][]entityFlag
	allergens         map[int64]inventory.Allergen
	members           map[int64]identity.User
	records           []userprefs.UserAllergen
}

// allergySource provides the services and household scope needed to build
// an allergyContext lazily for resolvers constructed outside the preload
// paths (e.g. mutation responses). A nil source means warnings cannot be
// computed — resolvers must surface that as an error rather than
// silently reporting "no conflicts".
type allergySource struct {
	inv         ItemReader
	id          IdentityService
	up          UserPrefsService
	householdID int64
}

// allergySrc builds the lazy-load source for resolvers constructed in a
// Resolver method.
func (r *Resolver) allergySrc(u currentuser.User) *allergySource {
	return &allergySource{
		inv:         r.InventoryService,
		id:          r.IdentityService,
		up:          r.UserPrefsService,
		householdID: u.HouseholdID,
	}
}

func asOfItemChildren(ch *itemChildren) *allergySource {
	if ch == nil {
		return nil
	}
	return ch.as
}

func asOfRecipeChildren(rc *recipeChildren) *allergySource {
	if rc == nil {
		return nil
	}
	return asOfItemChildren(rc.itemChildren)
}

func asOfGroceryChildren(gc *groceryChildren) *allergySource {
	if gc == nil {
		return nil
	}
	return asOfItemChildren(gc.ch)
}

func firstSource(sources ...*allergySource) *allergySource {
	for _, s := range sources {
		if s != nil {
			return s
		}
	}
	return nil
}

var errNoAllergyContext = errors.New("allergy context unavailable")

// loadAllergyContext batch-loads the flags, registry rows, members and
// records covering the given ingredient and item ids. resolved holds the
// override-aware item->ingredient map when the caller already computed it;
// when nil it is resolved here for itemIDs. householdID 0 (or nil id/up
// services) yields flags with no member records — valid for admin catalog
// browsing where warnings do not apply.
func (s *allergySource) loadAllergyContext(ctx context.Context, ingredientIDs, itemIDs []int64, resolved map[int64]*int64) (*allergyContext, error) {
	ac := &allergyContext{
		resolved:          make(map[int64]*int64),
		items:             make(map[int64]inventory.Item),
		flagsByIngredient: make(map[int64][]entityFlag),
		flagsByItem:       make(map[int64][]entityFlag),
		allergens:         make(map[int64]inventory.Allergen),
		members:           make(map[int64]identity.User),
	}
	if s == nil || s.inv == nil {
		return ac, nil
	}
	if len(itemIDs) > 0 {
		if resolved != nil {
			ac.resolved = resolved
		} else {
			r, err := s.inv.ResolveItemIngredients(ctx, s.householdID, itemIDs)
			if err != nil {
				return nil, err
			}
			ac.resolved = r
			// Item rows back the catalog-link fallback for entities whose
			// item is missing from the resolved map — only needed when the
			// caller did not supply a complete resolution.
			items, err := s.inv.GetItemsByIDs(ctx, itemIDs)
			if err != nil {
				return nil, err
			}
			for _, it := range items {
				ac.items[it.ItemID] = it
			}
		}
	}
	ingredientIDSet := make(map[int64]bool, len(ingredientIDs))
	for _, id := range ingredientIDs {
		ingredientIDSet[id] = true
	}
	// Resolved ingredient ids count toward the flag set too — an entity
	// bound only by item still pulls the override-aware ingredient flags.
	for _, v := range ac.resolved {
		if v != nil {
			ingredientIDSet[*v] = true
		}
	}
	allIngredientIDs := make([]int64, 0, len(ingredientIDSet))
	for id := range ingredientIDSet {
		allIngredientIDs = append(allIngredientIDs, id)
	}
	slices.Sort(allIngredientIDs)
	byIng := map[int64][]inventory.EntityAllergen{}
	if len(allIngredientIDs) > 0 {
		var err error
		byIng, err = s.inv.ListIngredientAllergensByIngredients(ctx, allIngredientIDs)
		if err != nil {
			return nil, err
		}
	}
	for ingredientID, flags := range byIng {
		for _, f := range flags {
			ac.flagsByIngredient[ingredientID] = append(ac.flagsByIngredient[ingredientID], entityFlag{allergenID: f.AllergenID, kind: f.Kind})
		}
	}
	byItem := map[int64][]inventory.EntityAllergen{}
	if len(itemIDs) > 0 {
		var err error
		byItem, err = s.inv.ListItemAllergensByItems(ctx, itemIDs)
		if err != nil {
			return nil, err
		}
	}
	for itemID, flags := range byItem {
		for _, f := range flags {
			ac.flagsByItem[itemID] = append(ac.flagsByItem[itemID], entityFlag{allergenID: f.AllergenID, kind: f.Kind})
		}
	}
	allergenIDSet := make(map[int64]bool)
	for _, flags := range byIng {
		for _, f := range flags {
			allergenIDSet[f.AllergenID] = true
		}
	}
	for _, flags := range byItem {
		for _, f := range flags {
			allergenIDSet[f.AllergenID] = true
		}
	}
	// Member roster + records — warnings label which member conflicts, so
	// the full household list loads once here rather than per entity.
	if s.id != nil && s.up != nil && s.householdID != 0 {
		members, err := s.id.ListUsersByHousehold(ctx, s.householdID)
		if err != nil {
			return nil, err
		}
		userIDs := make([]int64, 0, len(members))
		for _, m := range members {
			ac.members[m.UserID] = m
			userIDs = append(userIDs, m.UserID)
		}
		slices.Sort(userIDs)
		records, err := s.up.ListUserAllergensByUsers(ctx, userIDs)
		if err != nil {
			return nil, err
		}
		ac.records = records
		for _, r := range records {
			allergenIDSet[r.AllergenID] = true
		}
	}
	allergenIDs := make([]int64, 0, len(allergenIDSet))
	for id := range allergenIDSet {
		allergenIDs = append(allergenIDs, id)
	}
	slices.Sort(allergenIDs)
	if len(allergenIDs) > 0 {
		allergens, err := s.inv.GetAllergensByIDs(ctx, allergenIDs)
		if err != nil {
			return nil, err
		}
		ac.allergens = allergens
	}
	return ac, nil
}

// mergeFlags folds flags into set, keeping the strongest kind per
// allergen: "contains" outranks "may_contain".
func mergeFlags(set map[int64]string, flags []entityFlag) {
	for _, f := range flags {
		if set[f.allergenID] != inventory.AllergenFlagContains {
			if f.kind == inventory.AllergenFlagContains {
				set[f.allergenID] = inventory.AllergenFlagContains
			} else {
				set[f.allergenID] = f.kind
			}
		}
	}
}

// addInto folds one (ingredient, item) pair's flags into set. When the
// item resolves to an ingredient the override-aware flags merge in as
// well — over-warning beats under-warning.
func (ac *allergyContext) addInto(set map[int64]string, ingredientID, itemID *int64) {
	if ingredientID != nil {
		mergeFlags(set, ac.flagsByIngredient[*ingredientID])
	}
	if itemID != nil {
		mergeFlags(set, ac.flagsByItem[*itemID])
		ing := ac.effectiveIngredient(*itemID)
		if ing != nil && (ingredientID == nil || *ing != *ingredientID) {
			mergeFlags(set, ac.flagsByIngredient[*ing])
		}
	}
}

// effectiveIngredient is the item's override-aware ingredient, falling
// back to the catalog link when the resolved map has no entry.
func (ac *allergyContext) effectiveIngredient(itemID int64) *int64 {
	if ing, ok := ac.resolved[itemID]; ok {
		return ing
	}
	if it, ok := ac.items[itemID]; ok {
		return it.IngredientID
	}
	return nil
}

// lineSet builds the allergen set for one entity line.
func (ac *allergyContext) lineSet(ingredientID, itemID *int64) map[int64]string {
	set := make(map[int64]string)
	ac.addInto(set, ingredientID, itemID)
	return set
}

// ingredientSet builds the set for an ingredient-only entity.
func (ac *allergyContext) ingredientSet(ingredientID int64) map[int64]string {
	set := make(map[int64]string)
	mergeFlags(set, ac.flagsByIngredient[ingredientID])
	return set
}

// itemSet builds the set for a catalog item: its resolved ingredient's
// flags unioned with its own product-level flags.
func (ac *allergyContext) itemSet(it inventory.Item) map[int64]string {
	set := make(map[int64]string)
	ac.addInto(set, nil, &it.ItemID)
	return set
}

// flagResolvers renders an allergen set as sorted AllergenFlag rows.
// Unknown allergen ids are skipped only in name resolution; the flag
// itself has already driven warnings.
func (ac *allergyContext) flagResolvers(set map[int64]string) []*allergenFlagResolver {
	out := make([]*allergenFlagResolver, 0, len(set))
	for allergenID, kind := range set {
		a, ok := ac.allergens[allergenID]
		if !ok {
			continue
		}
		out = append(out, &allergenFlagResolver{a: a, kind: kind})
	}
	slices.SortFunc(out, func(x, y *allergenFlagResolver) int {
		if c := compareString(x.a.Name, y.a.Name); c != 0 {
			return c
		}
		return compareString(x.kind, y.kind)
	})
	return out
}

// warningResolvers intersects an entity's allergen set with the member
// records, labeling each conflict with member, record kind and the
// entity's strongest flag for that allergen.
func (ac *allergyContext) warningResolvers(set map[int64]string) []*allergyWarningResolver {
	var out []*allergyWarningResolver
	for _, rec := range ac.records {
		entityKind, ok := set[rec.AllergenID]
		if !ok {
			continue
		}
		member, mok := ac.members[rec.UserID]
		allergen, aok := ac.allergens[rec.AllergenID]
		if !mok || !aok {
			continue
		}
		out = append(out, &allergyWarningResolver{
			member:     member,
			allergen:   allergen,
			memberKind: rec.Kind,
			entityKind: entityKind,
		})
	}
	slices.SortFunc(out, func(x, y *allergyWarningResolver) int {
		if c := compareString(x.member.DisplayName, y.member.DisplayName); c != 0 {
			return c
		}
		if c := compareString(x.allergen.Name, y.allergen.Name); c != 0 {
			return c
		}
		return compareString(x.memberKind, y.memberKind)
	})
	return out
}

func compareString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// allergenResolver resolves Allergen fields.
type allergenResolver struct {
	a inventory.Allergen
}

func (r *allergenResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.a.AllergenID, 10))
}

func (r *allergenResolver) Name() string { return r.a.Name }

func (r *allergenResolver) Description() *string { return nilIfEmpty(r.a.Description) }

func (r *allergenResolver) IsActive() bool { return r.a.IsActive }

// allergenFlagResolver resolves AllergenFlag fields.
type allergenFlagResolver struct {
	a    inventory.Allergen
	kind string
}

func (r *allergenFlagResolver) Allergen() *allergenResolver { return &allergenResolver{a: r.a} }

func (r *allergenFlagResolver) Kind() string { return r.kind }

// memberAllergenResolver resolves MemberAllergen fields.
type memberAllergenResolver struct {
	a    inventory.Allergen
	kind string
}

func (r *memberAllergenResolver) Allergen() *allergenResolver { return &allergenResolver{a: r.a} }

func (r *memberAllergenResolver) Kind() string { return r.kind }

// allergyWarningResolver resolves AllergyWarning fields.
type allergyWarningResolver struct {
	member     identity.User
	allergen   inventory.Allergen
	memberKind string
	entityKind string
}

func (r *allergyWarningResolver) Member() *householdUserResolver {
	return &householdUserResolver{u: r.member}
}

func (r *allergyWarningResolver) Allergen() *allergenResolver {
	return &allergenResolver{a: r.allergen}
}

func (r *allergyWarningResolver) MemberKind() string { return r.memberKind }

func (r *allergyWarningResolver) EntityKind() string { return r.entityKind }

// Allergens lists the admin-curated registry every member records
// allergies or dietary restrictions against.
func (r *Resolver) Allergens(ctx context.Context) ([]*allergenResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	all, err := r.InventoryService.ListAllergens(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*allergenResolver, len(all))
	for i, a := range all {
		out[i] = &allergenResolver{a: a}
	}
	return out, nil
}

// MyAllergies lists the caller's own allergy/dietary records.
func (r *Resolver) MyAllergies(ctx context.Context) ([]*memberAllergenResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	records, err := r.UserPrefsService.ListUserAllergens(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(records))
	for _, rec := range records {
		ids = append(ids, rec.AllergenID)
	}
	allergens, err := r.InventoryService.GetAllergensByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*memberAllergenResolver, 0, len(records))
	for _, rec := range records {
		a, ok := allergens[rec.AllergenID]
		if !ok {
			continue
		}
		out = append(out, &memberAllergenResolver{a: a, kind: rec.Kind})
	}
	slices.SortFunc(out, func(x, y *memberAllergenResolver) int {
		return compareString(x.a.Name, y.a.Name)
	})
	return out, nil
}

// SetMyAllergy records or clears one of the caller's allergy/dietary
// records.
func (r *Resolver) SetMyAllergy(ctx context.Context, args struct {
	AllergenID graphql.ID
	Kind       string
	On         bool
}) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	allergenID, err := parseID(string(args.AllergenID))
	if err != nil {
		return false, err
	}
	by := u.Email
	if args.On {
		if err := r.UserPrefsService.SetUserAllergen(ctx, u.UserID, allergenID, args.Kind, by); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := r.UserPrefsService.ClearUserAllergen(ctx, u.UserID, allergenID); err != nil {
		return false, err
	}
	return true, nil
}

type createAllergenInput struct {
	Name        string
	Description *string
}

type updateAllergenInput struct {
	Name        *string
	Description *string
	IsActive    *bool
}

// CreateAllergen adds a registry entry (admin).
func (r *Resolver) CreateAllergen(ctx context.Context, args struct {
	Input createAllergenInput
}) (*allergenResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	var desc string
	if args.Input.Description != nil {
		desc = *args.Input.Description
	}
	a, err := r.InventoryService.CreateAllergen(ctx, args.Input.Name, desc, u.Email)
	if err != nil {
		return nil, err
	}
	return &allergenResolver{a: a}, nil
}

// UpdateAllergen renames, redescribes, or activates/deactivates a registry
// entry (admin). Unset fields keep their stored values.
func (r *Resolver) UpdateAllergen(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateAllergenInput
}) (*allergenResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.InventoryService.GetAllergenByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	desc := existing.Description
	isActive := existing.IsActive
	if args.Input.Name != nil {
		name = *args.Input.Name
	}
	if args.Input.Description != nil {
		desc = *args.Input.Description
	}
	if args.Input.IsActive != nil {
		isActive = *args.Input.IsActive
	}
	a, err := r.InventoryService.UpdateAllergen(ctx, id, name, desc, isActive, u.Email)
	if err != nil {
		return nil, err
	}
	return &allergenResolver{a: a}, nil
}

// SetIngredientAllergen upserts an ingredient flag, or clears it when
// kind is null (admin).
func (r *Resolver) SetIngredientAllergen(ctx context.Context, args struct {
	IngredientID graphql.ID
	AllergenID   graphql.ID
	Kind         *string
}) (bool, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return false, err
	}
	ingredientID, err := parseID(string(args.IngredientID))
	if err != nil {
		return false, err
	}
	allergenID, err := parseID(string(args.AllergenID))
	if err != nil {
		return false, err
	}
	if args.Kind == nil {
		if err := r.InventoryService.ClearIngredientAllergen(ctx, ingredientID, allergenID); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := r.InventoryService.SetIngredientAllergen(ctx, ingredientID, allergenID, *args.Kind, u.Email); err != nil {
		return false, err
	}
	return true, nil
}

// SetItemAllergen upserts a product-level flag, or clears it when kind is
// null (admin).
func (r *Resolver) SetItemAllergen(ctx context.Context, args struct {
	ItemID     graphql.ID
	AllergenID graphql.ID
	Kind       *string
}) (bool, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return false, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return false, err
	}
	allergenID, err := parseID(string(args.AllergenID))
	if err != nil {
		return false, err
	}
	if args.Kind == nil {
		if err := r.InventoryService.ClearItemAllergen(ctx, itemID, allergenID); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := r.InventoryService.SetItemAllergen(ctx, itemID, allergenID, *args.Kind, u.Email); err != nil {
		return false, err
	}
	return true, nil
}

// lazyAllergenCtx loads a single-entity context for resolvers built
// outside the preload path.
func lazyAllergenCtx(ctx context.Context, as *allergySource, ingredientIDs, itemIDs []int64) (*allergyContext, error) {
	if as == nil {
		slog.Default().Warn("allergy context missed preload and no source available")
		return nil, errNoAllergyContext
	}
	return as.loadAllergyContext(ctx, ingredientIDs, itemIDs, nil)
}
