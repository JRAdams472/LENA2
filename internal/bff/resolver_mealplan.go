package bff

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/graph-gophers/graphql-go"
)

// MealPlan resolves a single meal plan by ID.
func (r *Resolver) MealPlan(ctx context.Context, args struct{ ID graphql.ID }) (*mealPlanResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	mp, err := r.MealPlanService.GetMealPlanByID(ctx, id, u.UserID)
	if err != nil {
		return nil, err
	}
	return &mealPlanResolver{mp: r.MealPlanService, inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, plan: mp}, nil
}

// MealPlans resolves the current user's meal plans.
func (r *Resolver) MealPlans(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*mealPlanPageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	plans, err := r.MealPlanService.ListMealPlans(ctx, u.UserID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.MealPlanService.CountMealPlans(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	planIDs := distinctIDs(plans, func(p mealplan.MealPlan) *int64 { return &p.MealPlanID })
	slotsByPlan := make(map[int64][]mealplan.MealSlot)
	slotItemsBySlot := make(map[int64][]mealplan.MealSlotItem)
	var rc *recipeChildren
	if len(planIDs) > 0 {
		slots, err := r.MealPlanService.ListMealSlotsByPlans(ctx, planIDs)
		if err != nil {
			return nil, err
		}
		for _, s := range slots {
			slotsByPlan[s.MealPlanID] = append(slotsByPlan[s.MealPlanID], s)
		}
		slotItems, err := r.MealPlanService.ListMealSlotItemsByPlans(ctx, planIDs)
		if err != nil {
			return nil, err
		}
		for _, si := range slotItems {
			slotItemsBySlot[si.SlotID] = append(slotItemsBySlot[si.SlotID], si)
		}
		rc, err = loadRecipeChildren(ctx, r.RecipeService, r.UserPrefsService, r.InventoryService, u.UserID,
			distinctIDs(slots, func(s mealplan.MealSlot) *int64 { return s.RecipeID }),
			distinctIDs(slotItems, func(si mealplan.MealSlotItem) *int64 { return si.ItemID }))
		if err != nil {
			return nil, err
		}
	}
	return &mealPlanPageResolver{mp: r.MealPlanService, inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, plans: plans, slotsByPlan: slotsByPlan, slotItemsBySlot: slotItemsBySlot, rc: rc, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// Nutrition returns a nutrition summary for a meal plan.
func (r *Resolver) Nutrition(ctx context.Context, args struct{ MealPlanID graphql.ID }) ([]*nutritionResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	mealPlanID, err := parseID(string(args.MealPlanID))
	if err != nil {
		return nil, err
	}
	if _, err := r.MealPlanService.GetMealPlanByID(ctx, mealPlanID, u.UserID); err != nil {
		return nil, err
	}
	slots, err := r.MealPlanService.ListMealSlotsForPlan(ctx, mealPlanID)
	if err != nil {
		return nil, err
	}

	// Batch-load instead of per-slot/per-recipe fan-out: one query for all
	// slot items, one for all referenced recipes, one for all their items,
	// and one for the nutrients of every distinct item involved.
	slotItems, err := r.MealPlanService.ListMealSlotItemsByPlan(ctx, mealPlanID)
	if err != nil {
		return nil, err
	}
	itemsBySlot := make(map[int64][]mealplan.MealSlotItem)
	for _, si := range slotItems {
		itemsBySlot[si.SlotID] = append(itemsBySlot[si.SlotID], si)
	}

	recipeIDSet := make(map[int64]bool)
	for _, slot := range slots {
		if slot.RecipeID != nil {
			recipeIDSet[*slot.RecipeID] = true
		}
	}
	recipeIDs := make([]int64, 0, len(recipeIDSet))
	for id := range recipeIDSet {
		recipeIDs = append(recipeIDs, id)
	}
	var recipes []recipe.Recipe
	var recipeItems []recipe.RecipeItem
	if len(recipeIDs) > 0 {
		recipes, err = r.RecipeService.GetRecipesByIDs(ctx, recipeIDs)
		if err != nil {
			return nil, err
		}
		recipeItems, err = r.RecipeService.ListRecipeItemsByRecipes(ctx, recipeIDs)
		if err != nil {
			return nil, err
		}
	}
	recipesByID := make(map[int64]recipe.Recipe, len(recipes))
	for _, rec := range recipes {
		recipesByID[rec.RecipeID] = rec
	}
	itemsByRecipe := make(map[int64][]recipe.RecipeItem)
	for _, ri := range recipeItems {
		itemsByRecipe[ri.RecipeID] = append(itemsByRecipe[ri.RecipeID], ri)
	}

	itemIDSet := make(map[int64]bool)
	for _, si := range slotItems {
		if si.ItemID != nil {
			itemIDSet[*si.ItemID] = true
		}
	}
	for _, ri := range recipeItems {
		itemIDSet[ri.ItemID] = true
	}
	itemIDs := make([]int64, 0, len(itemIDSet))
	for id := range itemIDSet {
		itemIDs = append(itemIDs, id)
	}
	nutrientsByItem := make(map[int64][]inventory.FoodNutrient)
	if len(itemIDs) > 0 {
		nutrients, err := r.InventoryService.ListFoodNutrientsByItems(ctx, itemIDs)
		if err != nil {
			return nil, err
		}
		for _, n := range nutrients {
			nutrientsByItem[n.ItemID] = append(nutrientsByItem[n.ItemID], n)
		}
	}

	type total struct {
		name   string
		unit   string
		amount float64
	}
	totals := make(map[int64]total)

	addNutrients := func(itemID int64, quantity float64) {
		for _, n := range nutrientsByItem[itemID] {
			t := totals[n.NutrientID]
			t.name = n.Name
			t.unit = n.Unit
			t.amount += n.Amount * quantity
			totals[n.NutrientID] = t
		}
	}

	for _, slot := range slots {
		overridden := make(map[int64]bool)
		for _, si := range itemsBySlot[slot.SlotID] {
			if si.ItemID == nil {
				continue
			}
			addNutrients(*si.ItemID, si.Quantity)
			if si.IsFromRecipe {
				overridden[*si.ItemID] = true
			}
		}

		if slot.RecipeID == nil {
			continue
		}
		rec, ok := recipesByID[*slot.RecipeID]
		if !ok || rec.Servings == nil || *rec.Servings <= 0 {
			continue
		}
		scale := 1.0
		if slot.Servings != nil {
			scale = float64(*slot.Servings) / float64(*rec.Servings)
		}
		for _, ri := range itemsByRecipe[rec.RecipeID] {
			if overridden[ri.ItemID] {
				continue
			}
			addNutrients(ri.ItemID, ri.Quantity*scale)
		}
	}

	out := make([]*nutritionResolver, 0, len(totals))
	for _, t := range totals {
		out = append(out, &nutritionResolver{nutrition: nutrition{Name: t.name, Unit: t.unit, Amount: t.amount}})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].nutrition.Name < out[j].nutrition.Name })
	return out, nil
}

// CreateMealPlan creates a new meal plan for the current user.
func (r *Resolver) CreateMealPlan(ctx context.Context, args struct{ Input createMealPlanInput }) (*mealPlanResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	d, err := time.Parse("2006-01-02", args.Input.WeekStartDate)
	if err != nil {
		return nil, err
	}
	var dayOfWeek int32 = 1
	if args.Input.WeekStartDayOfWeek != nil {
		dayOfWeek = *args.Input.WeekStartDayOfWeek
	}
	mp, err := r.MealPlanService.CreateMealPlan(ctx, mealplan.MealPlan{
		UserID:             u.UserID,
		Name:               args.Input.Name,
		WeekStartDate:      d,
		WeekStartDayOfWeek: int16(dayOfWeek),
		IsActive:           true,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &mealPlanResolver{mp: r.MealPlanService, inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, plan: mp}, nil
}

// UpdateMealPlan modifies an existing meal plan.
func (r *Resolver) UpdateMealPlan(ctx context.Context, args struct {
	ID    graphql.ID
	Input createMealPlanInput
}) (*mealPlanResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.MealPlanService.GetMealPlanByID(ctx, id, u.UserID)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if args.Input.Name != "" {
		name = args.Input.Name
	}
	weekStart := existing.WeekStartDate
	if args.Input.WeekStartDate != "" {
		if d, err := time.Parse("2006-01-02", args.Input.WeekStartDate); err == nil {
			weekStart = d
		} else {
			return nil, err
		}
	}
	dayOfWeek := existing.WeekStartDayOfWeek
	if args.Input.WeekStartDayOfWeek != nil {
		dayOfWeek = int16(*args.Input.WeekStartDayOfWeek)
	}
	if err := r.MealPlanService.UpdateMealPlan(ctx, id, u.UserID, mealplan.MealPlan{
		Name:               name,
		WeekStartDate:      weekStart,
		WeekStartDayOfWeek: dayOfWeek,
		IsActive:           existing.IsActive,
	}, u.Email); err != nil {
		return nil, err
	}
	updated, err := r.MealPlanService.GetMealPlanByID(ctx, id, u.UserID)
	if err != nil {
		return nil, err
	}
	return &mealPlanResolver{mp: r.MealPlanService, inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, plan: updated}, nil
}

// DeleteMealPlan removes a meal plan owned by the current user.
func (r *Resolver) DeleteMealPlan(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.MealPlanService.DeleteMealPlan(ctx, id, u.UserID); err != nil {
		return false, err
	}
	return true, nil
}

// AddMealSlot adds a slot to a meal plan.
func (r *Resolver) AddMealSlot(ctx context.Context, args struct{ Input addMealSlotInput }) (*mealSlotResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	mealPlanID, err := parseID(string(args.Input.MealPlanID))
	if err != nil {
		return nil, err
	}
	var recipeID *int64
	if args.Input.RecipeID != nil {
		rid, err := parseID(string(*args.Input.RecipeID))
		if err != nil {
			return nil, err
		}
		recipeID = &rid
	}
	slot, err := r.MealPlanService.AddMealSlot(ctx, mealplan.MealSlot{
		MealPlanID:      mealPlanID,
		DayOfWeek:       int16(args.Input.DayOfWeek),
		MealType:        args.Input.MealType,
		RecipeID:        recipeID,
		Servings:        args.Input.Servings,
		ReplacementNote: derefString(args.Input.ReplacementNote),
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &mealSlotResolver{mp: r.MealPlanService, inv: r.InventoryService, rec: r.RecipeService, slot: slot}, nil
}

// RemoveMealSlot removes a slot from a meal plan.
func (r *Resolver) RemoveMealSlot(ctx context.Context, args struct{ SlotID graphql.ID }) (bool, error) {
	if _, err := userFromContext(ctx); err != nil {
		return false, err
	}
	slotID, err := parseID(string(args.SlotID))
	if err != nil {
		return false, err
	}
	if err := r.MealPlanService.DeleteMealSlot(ctx, slotID); err != nil {
		return false, err
	}
	return true, nil
}

// AddMealSlotItem adds an ad-hoc item to a slot.
func (r *Resolver) AddMealSlotItem(ctx context.Context, args struct{ Input addMealSlotItemInput }) (*mealSlotItemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	slotID, err := parseID(string(args.Input.SlotID))
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(string(args.Input.ItemID))
	if err != nil {
		return nil, err
	}
	ingredientID, err := optionalID(args.Input.IngredientID)
	if err != nil {
		return nil, err
	}
	isFromRecipe := false
	if args.Input.IsFromRecipe != nil {
		isFromRecipe = *args.Input.IsFromRecipe
	}
	item, err := r.MealPlanService.AddMealSlotItem(ctx, mealplan.MealSlotItem{
		SlotID:       slotID,
		ItemID:       &itemID,
		IngredientID: ingredientID,
		Quantity:     args.Input.Quantity,
		Unit:         args.Input.Unit,
		IsFromRecipe: isFromRecipe,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &mealSlotItemResolver{inv: r.InventoryService, item: item}, nil
}

// RemoveMealSlotItem removes an item from a slot.
func (r *Resolver) RemoveMealSlotItem(ctx context.Context, args struct{ SlotItemID graphql.ID }) (bool, error) {
	if _, err := userFromContext(ctx); err != nil {
		return false, err
	}
	slotItemID, err := parseID(string(args.SlotItemID))
	if err != nil {
		return false, err
	}
	if err := r.MealPlanService.DeleteMealSlotItem(ctx, slotItemID); err != nil {
		return false, err
	}
	return true, nil
}

// mealPlanResolver resolves MealPlan fields. When rc is non-nil the
// batch-loaded slots, slot items and recipe data are used instead of
// per-plan service calls.
type mealPlanResolver struct {
	mp        MealPlanService
	inv       InventoryService
	rec       RecipeService
	up        UserPrefsService
	user      currentuser.User
	plan      mealplan.MealPlan
	slots     []mealplan.MealSlot
	slotItems map[int64][]mealplan.MealSlotItem
	rc        *recipeChildren
}

func (r *mealPlanResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.plan.MealPlanID, 10))
}

func (r *mealPlanResolver) Name() string { return r.plan.Name }

func (r *mealPlanResolver) WeekStartDate() string { return r.plan.WeekStartDate.Format("2006-01-02") }

func (r *mealPlanResolver) IsActive() bool { return r.plan.IsActive }

func (r *mealPlanResolver) Slots(ctx context.Context) ([]*mealSlotResolver, error) {
	var slots []mealplan.MealSlot
	if r.rc != nil {
		slots = r.slots
	} else {
		var err error
		slots, err = r.mp.ListMealSlotsForPlan(ctx, r.plan.MealPlanID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*mealSlotResolver, len(slots))
	for i := range slots {
		out[i] = &mealSlotResolver{mp: r.mp, inv: r.inv, rec: r.rec, up: r.up, user: r.user, slot: slots[i], items: r.slotItems[slots[i].SlotID], rc: r.rc}
	}
	return out, nil
}

// mealSlotResolver resolves MealSlot fields. When rc is non-nil the
// batch-loaded slot items, recipes and catalog rows are used instead of
// per-slot service calls.
type mealSlotResolver struct {
	mp    MealPlanService
	inv   InventoryService
	rec   RecipeService
	up    UserPrefsService
	user  currentuser.User
	slot  mealplan.MealSlot
	items []mealplan.MealSlotItem
	rc    *recipeChildren
}

func (r *mealSlotResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.slot.SlotID, 10)) }

func (r *mealSlotResolver) DayOfWeek() int32 { return int32(r.slot.DayOfWeek) }

func (r *mealSlotResolver) MealType() string { return r.slot.MealType }

func (r *mealSlotResolver) Servings() *int32 { return r.slot.Servings }

func (r *mealSlotResolver) ReplacementNote() *string { return nilIfEmpty(r.slot.ReplacementNote) }

func (r *mealSlotResolver) Recipe(ctx context.Context) (*recipeResolver, error) {
	if r.slot.RecipeID == nil {
		return nil, nil
	}
	if r.rc != nil {
		rec, ok := r.rc.recipes[*r.slot.RecipeID]
		if !ok {
			return nil, nil
		}
		return &recipeResolver{inv: r.inv, rec: r.rec, up: r.up, user: r.user, recipe: rec, rc: r.rc}, nil
	}
	rec, err := r.rec.GetRecipeByID(ctx, *r.slot.RecipeID)
	if err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.inv, rec: r.rec, up: r.up, user: r.user, recipe: rec}, nil
}

func (r *mealSlotResolver) Items(ctx context.Context) ([]*mealSlotItemResolver, error) {
	var items []mealplan.MealSlotItem
	var itemsByID map[int64]inventory.Item
	var ch *itemChildren
	if r.rc != nil {
		items = r.items
		itemsByID = r.rc.items
		ch = r.rc.itemChildren
	} else {
		var err error
		items, err = r.mp.ListMealSlotItems(ctx, r.slot.SlotID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*mealSlotItemResolver, len(items))
	for i := range items {
		out[i] = &mealSlotItemResolver{inv: r.inv, item: items[i], items: itemsByID, ch: ch}
	}
	return out, nil
}

// mealSlotItemResolver resolves MealSlotItem fields.
type mealSlotItemResolver struct {
	inv   InventoryService
	item  mealplan.MealSlotItem
	items map[int64]inventory.Item
	ch    *itemChildren
}

func (r *mealSlotItemResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.item.SlotItemID, 10))
}

func (r *mealSlotItemResolver) Quantity() float64 { return r.item.Quantity }

func (r *mealSlotItemResolver) Unit() string { return r.item.Unit }

func (r *mealSlotItemResolver) IsFromRecipe() bool { return r.item.IsFromRecipe }

// Ingredient resolves the brand-agnostic ingredient linked to this slot
// item, when set. Scaffolding only — nothing populates ingredient_id yet.
func (r *mealSlotItemResolver) Ingredient(ctx context.Context) (*ingredientResolver, error) {
	if r.item.IngredientID == nil {
		return nil, nil
	}
	in, err := r.inv.GetIngredientByID(ctx, *r.item.IngredientID)
	if err != nil {
		return nil, err
	}
	return &ingredientResolver{inv: r.inv, in: in}, nil
}

func (r *mealSlotItemResolver) Item(ctx context.Context) (*itemResolver, error) {
	if r.item.ItemID == nil {
		return nil, nil
	}
	if r.items != nil {
		it, ok := r.items[*r.item.ItemID]
		if !ok {
			return nil, nil
		}
		return &itemResolver{inv: r.inv, it: it, ch: r.ch}, nil
	}
	it, err := r.inv.GetItemByID(ctx, *r.item.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it}, nil
}

type mealPlanPageResolver struct {
	mp              MealPlanService
	inv             InventoryService
	rec             RecipeService
	up              UserPrefsService
	user            currentuser.User
	plans           []mealplan.MealPlan
	slotsByPlan     map[int64][]mealplan.MealSlot
	slotItemsBySlot map[int64][]mealplan.MealSlotItem
	rc              *recipeChildren
	page            int32
	pageSize        int32
	total           int32
}

func (r *mealPlanPageResolver) Items() []*mealPlanResolver {
	out := make([]*mealPlanResolver, len(r.plans))
	for i := range r.plans {
		out[i] = &mealPlanResolver{mp: r.mp, inv: r.inv, rec: r.rec, up: r.up, user: r.user, plan: r.plans[i], slots: r.slotsByPlan[r.plans[i].MealPlanID], slotItems: r.slotItemsBySlot, rc: r.rc}
	}
	return out
}

func (r *mealPlanPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

type nutrition struct {
	Name   string
	Unit   string
	Amount float64
}

// nutritionResolver resolves a nutrition summary row.
type nutritionResolver struct{ nutrition nutrition }

func (r *nutritionResolver) Name() string { return r.nutrition.Name }

func (r *nutritionResolver) Unit() string { return r.nutrition.Unit }

func (r *nutritionResolver) Amount() float64 { return r.nutrition.Amount }

type createMealPlanInput struct {
	Name               string
	WeekStartDate      string
	WeekStartDayOfWeek *int32
}

type addMealSlotInput struct {
	MealPlanID      graphql.ID
	DayOfWeek       int32
	MealType        string
	RecipeID        *graphql.ID
	Servings        *int32
	ReplacementNote *string
}

type addMealSlotItemInput struct {
	SlotID       graphql.ID
	ItemID       graphql.ID
	IngredientID *graphql.ID
	Quantity     float64
	Unit         string
	IsFromRecipe *bool
}
