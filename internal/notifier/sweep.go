package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/notifier/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

// minutesPerDay marks a recipe step as "advance prep" — a step this long or
// longer implies the recipe needs to start days before the meal.
const minutesPerDay = 24 * 60

// gramsPerPound converts the weight-unit base (grams) to pounds for the
// defrost lead-time rule.
const gramsPerPound = 453.592

// displayDateLayout formats dates in notification text ("Mon Jan 2").
const displayDateLayout = "Mon Jan 2"

// Start launches the hourly reminder sweep on a background goroutine. The
// caller's context owns the sweep lifetime — cancelling stops it; Stop()
// drains the goroutine.
func (s *Service) Start(ctx context.Context) {
	sweepCtx, cancel := context.WithCancel(context.Background())
	s.stop = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// An immediate sweep covers downtime between deploys; the ticker
		// keeps it current after that.
		s.runSweep(sweepCtx)
		t := time.NewTicker(s.cfg.SweepInterval)
		defer t.Stop()
		for {
			select {
			case <-sweepCtx.Done():
				return
			case <-ctx.Done():
				return
			case <-t.C:
				s.runSweep(sweepCtx)
			}
		}
	}()
}

// Stop cancels the sweep goroutine and waits for any in-flight sweep to
// finish. Called from Resolver.Shutdown ahead of pool close.
func (s *Service) Stop() {
	if s.stop != nil {
		s.stop()
	}
	if s.delivery != nil {
		s.delivery.Stop()
	}
	s.wg.Wait()
}

func (s *Service) runSweep(ctx context.Context) {
	n, err := s.Sweep(ctx, time.Now())
	if err != nil {
		slog.Warn("notification sweep failed", "error", err)
		return
	}
	if n > 0 {
		slog.Info("notification sweep created reminders", "count", n)
	}
}

// Sweep materializes every reminder whose trigger has passed: protein
// defrost and advance-prep reminders from active meal plans, and expiry
// reminders from pantry items nearing expires_at. Inserts are dedup-keyed,
// so re-running a sweep for the same period creates nothing twice. Returns
// the number of notifications actually created.
func (s *Service) Sweep(ctx context.Context, now time.Time) (int, error) {
	created := 0
	n, err := s.sweepMealReminders(ctx, now)
	if err != nil {
		return created, err
	}
	created += n
	n, err = s.sweepExpiry(ctx, now)
	if err != nil {
		return created, err
	}
	created += n
	return created, nil
}

// slotDate resolves a recurring slot to a concrete date within its plan's
// week: the weekday relative to the plan's configured week start.
func slotDate(s sqlc.ListMealSlotsWithRecipesRow) time.Time {
	off := (int(s.DayOfWeek) - int(s.WeekStartDayOfWeek) + 7) % 7
	return s.WeekStartDate.Time.AddDate(0, 0, off)
}

// defrostLeadDays implements the spec: 48h baseline up to 8 lb, then 24h
// per 4 lb rounded up (10 lb -> 3 days, 20 lb -> 5 days).
func defrostLeadDays(lbs float64) int {
	d := int(math.Ceil(lbs / 4))
	if d < 2 {
		return 2
	}
	return d
}

// dueAt is the server-local moment a reminder fires: leadDays before the
// meal date at the configured notify hour.
func (s *Service) dueAt(mealDate time.Time, leadDays int) time.Time {
	d := mealDate.AddDate(0, 0, -leadDays)
	return time.Date(d.Year(), d.Month(), d.Day(), s.cfg.NotifyHour, 0, 0, 0, time.Local)
}

// proteinPounds sums weight-convertible protein ingredient quantities,
// scaled from the recipe's base servings to the slot's servings.
func proteinPounds(items []sqlc.ListProteinItemsForRecipesRow, scale float64) float64 {
	var grams float64
	for _, it := range items {
		if it.UnitKind.String != "weight" {
			continue
		}
		qty, err1 := it.Quantity.Float64Value()
		factor, err2 := it.ToBaseFactor.Float64Value()
		if err1 != nil || err2 != nil {
			continue
		}
		grams += qty.Float64 * factor.Float64
	}
	return grams * scale / gramsPerPound
}

// servingScale converts a slot's servings into a multiplier on the recipe's
// base quantities; falls back to 1 when either side is unset.
func servingScale(s sqlc.ListMealSlotsWithRecipesRow) float64 {
	if !s.Servings.Valid || !s.RecipeServings.Valid || s.RecipeServings.Int32 <= 0 {
		return 1
	}
	return float64(s.Servings.Int32) / float64(s.RecipeServings.Int32)
}

func (s *Service) sweepMealReminders(ctx context.Context, now time.Time) (int, error) {
	slots, err := s.q.ListMealSlotsWithRecipes(ctx)
	if err != nil {
		return 0, fmt.Errorf("list meal slots: %w", domainerr.FromStorage(err))
	}
	if len(slots) == 0 {
		return 0, nil
	}
	recipeIDs := distinctInt64s(func() []int64 {
		ids := make([]int64, 0, len(slots))
		for _, sl := range slots {
			ids = append(ids, sl.RecipeID)
		}
		return ids
	}())
	proteinByRecipe, maxStepByRecipe, err := s.recipeSignals(ctx, recipeIDs)
	if err != nil {
		return 0, err
	}

	householdIDs := distinctInt64s(func() []int64 {
		ids := make([]int64, 0, len(slots))
		for _, sl := range slots {
			ids = append(ids, sl.HouseholdID)
		}
		return ids
	}())
	members, err := s.membersByHousehold(ctx, householdIDs)
	if err != nil {
		return 0, err
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	created := 0
	for _, sl := range slots {
		meal := slotDate(sl)
		// A meal dated before today is already over — too late to remind.
		if meal.Before(today) {
			continue
		}
		created += s.slotReminders(ctx, sl, meal, now, servingScale(sl),
			slotContext{proteinByRecipe, maxStepByRecipe, members})
	}
	return created, nil
}

// recipeSignals loads the per-recipe inputs both reminder rules need:
// protein items for defrost lead time and longest-step duration for
// advance prep.
func (s *Service) recipeSignals(ctx context.Context, recipeIDs []int64) (map[int64][]sqlc.ListProteinItemsForRecipesRow, map[int64]int32, error) {
	proteinByRecipe := map[int64][]sqlc.ListProteinItemsForRecipesRow{}
	proteinRows, err := s.q.ListProteinItemsForRecipes(ctx, recipeIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("list protein items: %w", domainerr.FromStorage(err))
	}
	for _, r := range proteinRows {
		proteinByRecipe[r.RecipeID] = append(proteinByRecipe[r.RecipeID], r)
	}
	maxStepByRecipe := map[int64]int32{}
	stepRows, err := s.q.ListLongStepsForRecipes(ctx, recipeIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("list long steps: %w", domainerr.FromStorage(err))
	}
	for _, r := range stepRows {
		maxStepByRecipe[r.RecipeID] = r.MaxDurationMinutes
	}
	return proteinByRecipe, maxStepByRecipe, nil
}

// slotContext carries the precomputed per-recipe signals and household
// membership a slot's reminders consult.
type slotContext struct {
	proteinByRecipe map[int64][]sqlc.ListProteinItemsForRecipesRow
	maxStepByRecipe map[int64]int32
	members         map[int64][]int64
}

// slotReminders emits the defrost and advance-prep reminders for one meal
// slot whose date is not in the past.
func (s *Service) slotReminders(ctx context.Context, sl sqlc.ListMealSlotsWithRecipesRow, meal, now time.Time, scale float64, sc slotContext) int {
	created := 0
	if items := sc.proteinByRecipe[sl.RecipeID]; len(items) > 0 {
		lbs := proteinPounds(items, scale)
		lead := defrostLeadDays(lbs)
		if !now.Before(s.dueAt(meal, lead)) {
			created += s.insertRecipeReminder(ctx, sc.members[sl.HouseholdID], sl.HouseholdID, recipeReminder{
				kind:  KindProteinDefrost,
				title: fmt.Sprintf("Defrost protein for %s", meal.Format(displayDateLayout)),
				body: fmt.Sprintf("%s calls for %.1f lb of protein, which needs about %d days to thaw — take it out of the freezer today.",
					sl.RecipeName, lbs, lead),
				recipeID:  sl.RecipeID,
				dedupBase: fmt.Sprintf("%s:%d", KindProteinDefrost, sl.SlotID),
			})
		}
	}
	if maxDur, ok := sc.maxStepByRecipe[sl.RecipeID]; ok {
		lead := int(math.Ceil(float64(maxDur) / float64(minutesPerDay)))
		if !now.Before(s.dueAt(meal, lead)) {
			created += s.insertRecipeReminder(ctx, sc.members[sl.HouseholdID], sl.HouseholdID, recipeReminder{
				kind:  KindMealPrepAdvance,
				title: fmt.Sprintf("Start prep for %s", sl.RecipeName),
				body: fmt.Sprintf("%s on %s has a step that takes about %d day(s) — start it today to be ready in time.",
					sl.RecipeName, meal.Format(displayDateLayout), lead),
				recipeID:  sl.RecipeID,
				dedupBase: fmt.Sprintf("%s:%d", KindMealPrepAdvance, sl.SlotID),
			})
		}
	}
	return created
}

func (s *Service) sweepExpiry(ctx context.Context, now time.Time) (int, error) {
	windowEnd := now.AddDate(0, 0, s.cfg.ExpiryDays)
	items, err := s.q.ListExpiringHouseholdItems(ctx, sqlc.ListExpiringHouseholdItemsParams{
		ExpiresAt:   pgtype.Timestamptz{Time: now, Valid: true},
		ExpiresAt_2: pgtype.Timestamptz{Time: windowEnd, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("list expiring items: %w", domainerr.FromStorage(err))
	}
	if len(items) == 0 {
		return 0, nil
	}
	householdIDs := distinctInt64s(func() []int64 {
		ids := make([]int64, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.HouseholdID)
		}
		return ids
	}())
	members, err := s.membersByHousehold(ctx, householdIDs)
	if err != nil {
		return 0, err
	}
	created := 0
	for _, it := range items {
		exp := it.ExpiresAt.Time.Format(displayDateLayout)
		title := fmt.Sprintf("%s expires soon", it.ItemName)
		body := fmt.Sprintf("%s expires %s — use it or add a replacement to your grocery list.", it.ItemName, exp)
		created += s.insertItemReminder(ctx, members[it.HouseholdID], it.HouseholdID, itemReminder{
			kind:   KindItemExpiring,
			title:  title,
			body:   body,
			itemID: it.ItemID,
			dedupBase: fmt.Sprintf("%s:%d:%s", KindItemExpiring, it.HouseholdItemID,
				it.ExpiresAt.Time.Format(time.DateOnly)),
		})
	}
	return created, nil
}

// membersByHousehold fetches member user IDs grouped by household.
func (s *Service) membersByHousehold(ctx context.Context, householdIDs []int64) (map[int64][]int64, error) {
	rows, err := s.q.ListMemberIDsForHouseholds(ctx, householdIDs)
	if err != nil {
		return nil, fmt.Errorf("list household members: %w", domainerr.FromStorage(err))
	}
	out := map[int64][]int64{}
	for _, r := range rows {
		if r.HouseholdID.Valid {
			out[r.HouseholdID.Int64] = append(out[r.HouseholdID.Int64], r.UserID)
		}
	}
	return out, nil
}

// recipeReminder carries one recipe-scoped notification's content and
// dedup identity, bundled so the member fan-out stays under the parameter
// limit.
type recipeReminder struct {
	kind      string
	title     string
	body      string
	recipeID  int64
	dedupBase string
}

// itemReminder is the item-scoped counterpart of recipeReminder.
type itemReminder struct {
	kind      string
	title     string
	body      string
	itemID    int64
	dedupBase string
}

// insertRecipeReminder writes one protein/prep notification per member,
// honoring each member's opt-outs. dedupBase is suffixed with :userID so
// the per-user keys differ.
func (s *Service) insertRecipeReminder(ctx context.Context, memberIDs []int64, householdID int64, r recipeReminder) int {
	created := 0
	for _, uid := range memberIDs {
		feedOK, pushOK := s.channelDecisions(ctx, uid, r.kind)
		if !feedOK && !pushOK {
			continue
		}
		dedup := fmt.Sprintf("%s:%d", r.dedupBase, uid)
		if feedOK {
			tag, err := s.q.InsertRecipeReminderNotification(ctx, sqlc.InsertRecipeReminderNotificationParams{
				UserID:      uid,
				HouseholdID: pgtype.Int8{Int64: householdID, Valid: true},
				Kind:        r.kind,
				Title:       pgtype.Text{String: r.title, Valid: true},
				Body:        pgtype.Text{String: r.body, Valid: true},
				RecipeID:    pgtype.Int8{Int64: r.recipeID, Valid: true},
				DedupKey:    pgtype.Text{String: dedup, Valid: true},
			})
			if err != nil {
				slog.Warn("reminder insert failed", "kind", r.kind, "user", uid, "error", err)
				continue
			}
			created += int(tag.RowsAffected())
			_ = s.q.PruneReadNotifications(ctx, uid)
		}
		if pushOK {
			s.enqueuePush(ctx, sqlc.InsertPushDeliveryParams{
				UserID:      uid,
				Kind:        r.kind,
				HouseholdID: pgtype.Int8{Int64: householdID, Valid: true},
				RecipeID:    pgtype.Int8{Int64: r.recipeID, Valid: true},
				Title:       pgtype.Text{String: r.title, Valid: true},
				Body:        pgtype.Text{String: r.body, Valid: true},
				DedupKey:    pgtype.Text{String: dedup, Valid: true},
			})
		}
	}
	return created
}

func (s *Service) insertItemReminder(ctx context.Context, memberIDs []int64, householdID int64, r itemReminder) int {
	created := 0
	for _, uid := range memberIDs {
		feedOK, pushOK := s.channelDecisions(ctx, uid, r.kind)
		if !feedOK && !pushOK {
			continue
		}
		dedup := fmt.Sprintf("%s:%d", r.dedupBase, uid)
		if feedOK {
			tag, err := s.q.InsertItemReminderNotification(ctx, sqlc.InsertItemReminderNotificationParams{
				UserID:      uid,
				HouseholdID: pgtype.Int8{Int64: householdID, Valid: true},
				Kind:        r.kind,
				Title:       pgtype.Text{String: r.title, Valid: true},
				Body:        pgtype.Text{String: r.body, Valid: true},
				ItemID:      pgtype.Int8{Int64: r.itemID, Valid: true},
				DedupKey:    pgtype.Text{String: dedup, Valid: true},
			})
			if err != nil {
				slog.Warn("reminder insert failed", "kind", r.kind, "user", uid, "error", err)
				continue
			}
			created += int(tag.RowsAffected())
			_ = s.q.PruneReadNotifications(ctx, uid)
		}
		if pushOK {
			s.enqueuePush(ctx, sqlc.InsertPushDeliveryParams{
				UserID:      uid,
				Kind:        r.kind,
				HouseholdID: pgtype.Int8{Int64: householdID, Valid: true},
				ItemID:      pgtype.Int8{Int64: r.itemID, Valid: true},
				Title:       pgtype.Text{String: r.title, Valid: true},
				Body:        pgtype.Text{String: r.body, Valid: true},
				DedupKey:    pgtype.Text{String: dedup, Valid: true},
			})
		}
	}
	return created
}

// channelDecisions evaluates the feed and push gates for one recipient.
// Feed errors fail open (deliver); push errors fail closed — a prefs
// outage must not send pushes to a phone.
func (s *Service) channelDecisions(ctx context.Context, userID int64, kind string) (feedOK, pushOK bool) {
	ok, err := s.Allowed(ctx, userID, kind)
	if err != nil {
		slog.Warn("notification suppression check failed, delivering anyway", "user", userID, "kind", kind, "error", err)
		ok = true
	}
	feedOK = ok
	ok, err = s.PushAllowed(ctx, userID, kind)
	if err != nil {
		slog.Warn("push suppression check failed, skipping push", "user", userID, "kind", kind, "error", err)
		return feedOK, false
	}
	return feedOK, ok
}

// enqueuePush writes the outbox row the delivery worker drains; dedup_key
// makes a repeat sweep insert a no-op, matching the feed insert's idempotency.
func (s *Service) enqueuePush(ctx context.Context, p sqlc.InsertPushDeliveryParams) {
	if _, err := s.q.InsertPushDelivery(ctx, p); err != nil {
		slog.Warn("push delivery enqueue failed", "kind", p.Kind, "user", p.UserID, "error", err)
	}
}

// distinctInt64s dedups preserving first-seen order.
func distinctInt64s(vals []int64) []int64 {
	seen := map[int64]bool{}
	out := []int64{}
	for _, v := range vals {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
