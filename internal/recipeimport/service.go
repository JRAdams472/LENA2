package recipeimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"

	"github.com/JRAdams472/LENA2/internal/ocrimport"
	"github.com/JRAdams472/LENA2/internal/platform/config"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/ocrclient"
	"github.com/JRAdams472/LENA2/internal/platform/ollamaclient"
	"github.com/JRAdams472/LENA2/internal/platform/profanity"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

// Config holds the recipe import service thresholds.
type Config struct {
	OCRConfidenceThreshold     int
	ImportAutoAcceptConfidence float64
	ImportReviewThreshold      float64
	ImportWorkerConcurrency    int
}

// ConfigFromPlatform converts the platform config to the local config.
func ConfigFromPlatform(cfg *config.Config) Config {
	c := Config{
		OCRConfidenceThreshold:     cfg.OCRConfidenceThreshold,
		ImportAutoAcceptConfidence: cfg.ImportAutoAcceptConfidence,
		ImportReviewThreshold:      cfg.ImportReviewThreshold,
		ImportWorkerConcurrency:    cfg.ImportWorkerConcurrency,
	}
	if c.ImportWorkerConcurrency <= 0 {
		c.ImportWorkerConcurrency = 1
	}
	return c
}

//go:generate go run go.uber.org/mock/mockgen -source=service.go -package=mock -destination=mock/service.go Store,RecipeWriter

// RecipeWriter is the subset of recipe.Service needed by the importer.
type RecipeWriter interface {
	CreateRecipeWithChildren(ctx context.Context, arg recipe.Recipe, items []recipe.RecipeItem, steps []recipe.RecipeStep, by string) (recipe.Recipe, error)
}

// Service orchestrates the server-side recipe OCR pipeline.
type Service struct {
	pool      dbtx.Pool
	store     Store
	ocr       *ocrclient.Client
	ollama    *ollamaclient.Client
	inv       InventoryReader
	rec       RecipeWriter
	profanity *profanity.Detector
	cfg       Config

	workerSem chan struct{}
	workerWG  sync.WaitGroup
	shutdown  chan struct{}
}

// NewService builds a recipe import service.
func NewService(
	pool dbtx.Pool,
	ocr *ocrclient.Client,
	ollama *ollamaclient.Client,
	inv InventoryReader,
	rec RecipeWriter,
	profanity *profanity.Detector,
	cfg Config,
) *Service {
	if cfg.ImportWorkerConcurrency <= 0 {
		cfg.ImportWorkerConcurrency = 1
	}
	s := &Service{
		pool:      pool,
		store:     NewStore(pool),
		ocr:       ocr,
		ollama:    ollama,
		inv:       inv,
		rec:       rec,
		profanity: profanity,
		cfg:       cfg,
		workerSem: make(chan struct{}, cfg.ImportWorkerConcurrency),
		shutdown:  make(chan struct{}),
	}
	return s
}

// Create inserts a pending import job and enqueues it for processing.
func (s *Service) Create(ctx context.Context, sourceFilename, sourcePath, sourceHash string, submittedByUserID *int64, createdBy string) (*RecipeImport, error) {
	ri := RecipeImport{
		SubmittedByUserID: submittedByUserID,
		SourceFilename:    sourceFilename,
		SourcePath:        sourcePath,
		SourceHash:        sourceHash,
		Status:            "pending",
		CreatedBy:         createdBy,
		UpdatedBy:         createdBy,
	}
	created, err := s.store.Create(ctx, ri)
	if err != nil {
		return nil, err
	}
	s.EnqueueProcess(created.ID)
	return created, nil
}

// Get returns a recipe import by id.
func (s *Service) Get(ctx context.Context, id int64) (*RecipeImport, error) {
	return s.store.Get(ctx, id)
}

// List returns a paged list of recipe imports.
func (s *Service) List(ctx context.Context, status string, page, pageSize int32) ([]RecipeImport, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}
	return s.store.List(ctx, status, pageSize, (page-1)*pageSize)
}

// Count returns the total number of recipe imports, optionally filtered by status.
func (s *Service) Count(ctx context.Context, status string) (int64, error) {
	return s.store.Count(ctx, status)
}

// ListPending returns jobs that may appear in the review queue.
func (s *Service) ListPending(ctx context.Context, page, pageSize int32) ([]RecipeImport, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}
	// Pending list merges all statuses an admin may need to act on. The SQL
	// store currently filters by a single status, so we return the union by
	// querying per status and merging; in practice these lists stay small.
	statuses := []string{"pending", "processing", "ocred", "drafted", "reviewing", "ready", "profanity", "failed"}
	out := make([]RecipeImport, 0, pageSize)
	for _, st := range statuses {
		if int64(len(out)) >= int64(pageSize) {
			break
		}
		rows, err := s.store.List(ctx, st, pageSize, (page-1)*pageSize)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
		if int64(len(out)) > int64(pageSize) {
			out = out[:pageSize]
		}
	}
	return out, nil
}

// UpdateReview stores an edited review and validates that item/unit ids resolve.
func (s *Service) UpdateReview(ctx context.Context, id int64, review *ocrimport.ReviewRecipe, updatedBy string) (*RecipeImport, error) {
	if review == nil {
		return nil, errors.New("review is nil")
	}
	for i, it := range review.Items {
		if it.ItemID != "" {
			itemID, err := strconv.ParseInt(it.ItemID, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("item %d has invalid itemId: %w", i, err)
			}
			if _, err := s.inv.GetItemByID(ctx, itemID); err != nil {
				return nil, fmt.Errorf("item %d catalog item invalid: %w", i, err)
			}
		}
		if it.UnitID != "" {
			unitID, err := strconv.ParseInt(it.UnitID, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("item %d has invalid unitId: %w", i, err)
			}
			if _, err := s.inv.GetUnitByID(ctx, unitID); err != nil {
				return nil, fmt.Errorf("item %d catalog unit invalid: %w", i, err)
			}
		}
		if it.Unit != "" && it.UnitID == "" {
			unit, err := s.inv.GetUnitByName(ctx, it.Unit)
			if err != nil {
				return nil, fmt.Errorf("item %d unit name invalid: %w", i, err)
			}
			review.Items[i].UnitID = strconv.FormatInt(unit.UnitID, 10)
		}
	}

	reviewJSON, err := json.Marshal(review)
	if err != nil {
		return nil, fmt.Errorf("marshal review: %w", err)
	}

	status := "reviewing"
	if review.AllResolved() {
		status = "ready"
	}
	if err := s.store.UpdateReview(ctx, id, reviewJSON, status, updatedBy); err != nil {
		return nil, err
	}
	return s.store.Get(ctx, id)
}

// Approve persists the review as a recipe.
func (s *Service) Approve(ctx context.Context, id int64, approvedBy currentuser.User) (*recipe.Recipe, *RecipeImport, error) {
	ri, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if ri.Status == "persisted" {
		return nil, nil, fmt.Errorf("recipe import %d already persisted", id)
	}
	if ri.Status == "profanity" {
		return nil, nil, fmt.Errorf("recipe import %d has profanity and cannot be approved", id)
	}

	var review ocrimport.ReviewRecipe
	if err := json.Unmarshal(ri.ReviewJSON, &review); err != nil {
		return nil, nil, fmt.Errorf("parse review json: %w", err)
	}
	if !review.AllResolved() {
		return nil, nil, errors.New("review is not fully resolved")
	}

	rcp, items, steps, err := s.buildRecipe(ctx, &review)
	if err != nil {
		return nil, nil, err
	}

	created, err := s.rec.CreateRecipeWithChildren(ctx, rcp, items, steps, approvedBy.Email)
	if err != nil {
		return nil, nil, fmt.Errorf("create recipe from import: %w", err)
	}
	if err := s.store.SetPersisted(ctx, id, created.RecipeID, approvedBy.UserID); err != nil {
		return nil, nil, fmt.Errorf("mark import persisted: %w", err)
	}

	rc, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return &created, rc, nil
}

func (s *Service) buildRecipe(ctx context.Context, review *ocrimport.ReviewRecipe) (recipe.Recipe, []recipe.RecipeItem, []recipe.RecipeStep, error) {
	rcp := recipe.Recipe{
		Name:            review.Name,
		IsActive:        true,
		Servings:        int32Ptr(review.Servings),
		PrepTimeMinutes: int32Ptr(review.PrepTime),
		CookTimeMinutes: int32Ptr(review.CookTime),
	}
	if review.Description != nil {
		rcp.Description = *review.Description
	}

	items := make([]recipe.RecipeItem, 0, len(review.Items))
	for i, it := range review.Items {
		itemID, err := strconv.ParseInt(it.ItemID, 10, 64)
		if err != nil {
			return rcp, nil, nil, fmt.Errorf("item %d invalid itemId: %w", i, err)
		}
		unitID, err := strconv.ParseInt(it.UnitID, 10, 64)
		if err != nil {
			// fall back to name lookup
			unit, err := s.inv.GetUnitByName(ctx, it.Unit)
			if err != nil {
				return rcp, nil, nil, fmt.Errorf("item %d unresolved unit: %w", i, err)
			}
			unitID = unit.UnitID
		}
		qty := 1.0
		if it.DraftItem.Quantity != nil {
			qty = *it.DraftItem.Quantity
		}
		section := ""
		if it.DraftItem.Section != nil {
			section = *it.DraftItem.Section
		}
		notes := ""
		if it.DraftItem.Notes != nil {
			notes = *it.DraftItem.Notes
		}
		items = append(items, recipe.RecipeItem{
			ItemID:       itemID,
			Quantity:     qty,
			UnitID:       unitID,
			SectionName:  section,
			DisplayOrder: safeInt32(i + 1),
			Notes:        notes,
			IsOptional:   it.DraftItem.IsOptional,
		})
	}

	steps := make([]recipe.RecipeStep, 0, len(review.Steps))
	for _, st := range review.Steps {
		steps = append(steps, recipe.RecipeStep{
			StepNumber:  safeInt32(st.StepNumber),
			Instruction: st.Instruction,
		})
	}

	return rcp, items, steps, nil
}

// Reject marks a recipe import as rejected.
func (s *Service) Reject(ctx context.Context, id int64) error {
	return s.store.SetRejected(ctx, id)
}

// Retry resets a job to pending and re-enqueues processing.
func (s *Service) Retry(ctx context.Context, id int64) error {
	if err := s.store.SetPending(ctx, id); err != nil {
		return err
	}
	s.EnqueueProcess(id)
	return nil
}

// EnqueueProcess submits an import job to the worker pool.
func (s *Service) EnqueueProcess(id int64) {
	s.workerWG.Add(1)
	go func() {
		defer s.workerWG.Done()
		select {
		case s.workerSem <- struct{}{}:
		case <-s.shutdown:
			return
		}
		defer func() { <-s.workerSem }()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := s.Process(ctx, id); err != nil {
			_ = s.store.MarkFailed(ctx, id, err.Error())
		}
	}()
}

// Process runs the OCR -> draft -> review pipeline for one import.
func (s *Service) Process(ctx context.Context, id int64) error {
	ri, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if ri.Status != "pending" && ri.Status != "processing" && ri.Status != "ocred" && ri.Status != "drafted" {
		// Already processed or terminal; skip.
		return nil
	}

	if err := s.store.UpdateReview(ctx, id, ri.ReviewJSON, "processing", ri.UpdatedBy); err != nil {
		return err
	}

	data, err := os.ReadFile(ri.SourcePath)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}

	res, err := s.ocr.ExtractTextResult(ctx, data, ri.SourceFilename)
	if err != nil {
		return fmt.Errorf("ocr: %w", err)
	}

	minConf := float64(100)
	for _, p := range res.Pages {
		if p.MeanConfidence < minConf {
			minConf = p.MeanConfidence
		}
	}
	if minConf < float64(s.cfg.OCRConfidenceThreshold) {
		return fmt.Errorf("mean confidence %.2f below threshold %d", minConf, s.cfg.OCRConfidenceThreshold)
	}

	ocrText := res.Text
	if matched, terms := s.profanity.Check(ocrText); matched {
		reason := fmt.Sprintf("profanity detected in OCR text: %s", terms)
		_ = s.store.MarkProfanity(ctx, id, reason)
		return nil
	}

	ocrJSON, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("marshal ocr json: %w", err)
	}
	if err := s.store.UpdateOCR(ctx, id, ocrText, ocrJSON); err != nil {
		return err
	}

	draft, err := s.structuredDraft(ctx, ocrText)
	if err != nil {
		return err
	}

	if draft.ProfanityDetected {
		reason := "profanity detected in structured draft"
		if draft.ProfanityReason != nil {
			reason = *draft.ProfanityReason
		}
		_ = s.store.MarkProfanity(ctx, id, reason)
		return nil
	}

	draftJSON, err := json.Marshal(draft)
	if err != nil {
		return fmt.Errorf("marshal draft json: %w", err)
	}
	if err := s.store.UpdateDraft(ctx, id, draftJSON); err != nil {
		return err
	}

	cat, err := newCatalogSnapshot(ctx, s.inv)
	if err != nil {
		return err
	}

	review := &ocrimport.ReviewRecipe{
		PageID:      strconv.FormatInt(id, 10),
		Name:        draft.Name,
		Description: draft.Description,
		Servings:    draft.Servings,
		PrepTime:    draft.PrepTimeMinutes,
		CookTime:    draft.CookTimeMinutes,
		SourceHint:  draft.SourceHint,
		Items:       make([]ocrimport.MatchResult, 0, len(draft.Items)),
		Steps:       draft.Steps,
	}
	for _, dItem := range draft.Items {
		review.Items = append(review.Items, cat.MapDraftItem(dItem, s.cfg.ImportAutoAcceptConfidence, s.cfg.ImportReviewThreshold))
	}

	reviewJSON, err := json.Marshal(review)
	if err != nil {
		return fmt.Errorf("marshal review json: %w", err)
	}

	status := "reviewing"
	if review.AllResolved() {
		status = "ready"
	}
	if err := s.store.UpdateReview(ctx, id, reviewJSON, status, ri.UpdatedBy); err != nil {
		return err
	}

	return nil
}

func (s *Service) structuredDraft(ctx context.Context, ocrText string) (*ocrimport.RecipeDraft, error) {
	if s.ollama == nil {
		return nil, errors.New("ollama client not configured")
	}

	schemaJSON, _ := json.MarshalIndent(ocrimport.JSONSchema(), "", "  ")
	systemPrompt := "You are a precise recipe transcription assistant. " +
		"You are given OCR text extracted from a scanned recipe page. " +
		"Your job is to return a single JSON object matching this JSON Schema and nothing else. " +
		"Do not add, improve, or invent anything. Use null for missing fields. " +
		"Convert fractional quantities like \"1 1/2\" to decimals like 1.5. " +
		"For ranges such as \"2-3\", use the lower value and put the range in notes. " +
		"The ingredient should be the bare noun phrase; preparation goes in notes. " +
		"If the source text contains profanity or hateful language, set profanityDetected to true and explain in profanityReason.\n\n" +
		"JSON Schema:\n" + string(schemaJSON)

	userPrompt := "OCR text:\n" + ocrText

	content, err := s.ollama.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}

	var draft ocrimport.RecipeDraft
	if err := json.Unmarshal([]byte(content), &draft); err != nil {
		return nil, fmt.Errorf("parse draft: %w", err)
	}
	if vErr := ocrimport.ValidateDraft(&draft); vErr != nil {
		return nil, fmt.Errorf("validate draft: %w", vErr)
	}

	// Second opinion: if the OCR text itself was clean but the LLM produced
	// profanity markers, trust the LLM. Also apply the hardcoded detector to
	// the structured fields so we are robust against prompt injection.
	if !draft.ProfanityDetected {
		text := draft.Name
		for _, it := range draft.Items {
			text += " " + it.Ingredient
			if it.Notes != nil {
				text += " " + *it.Notes
			}
		}
		for _, st := range draft.Steps {
			text += " " + st.Instruction
		}
		if matched, terms := s.profanity.Check(text); matched {
			draft.ProfanityDetected = true
			reason := fmt.Sprintf("profanity detected in draft fields: %s", terms)
			draft.ProfanityReason = &reason
		}
	}

	return &draft, nil
}

// Shutdown drains the worker pool.
func (s *Service) Shutdown(ctx context.Context) error {
	close(s.shutdown)
	done := make(chan struct{})
	go func() {
		s.workerWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Helpers for pointer conversion and safe int casting.

func int32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	i := safeInt32(*v)
	return &i
}

func safeInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}
