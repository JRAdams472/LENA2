package recipeimport

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/app/recipeimport/sqlc"
)

// Store is the persistence interface for recipe import jobs.
type Store interface {
	Create(ctx context.Context, ri RecipeImport) (*RecipeImport, error)
	Get(ctx context.Context, id int64) (*RecipeImport, error)
	List(ctx context.Context, status string, limit, offset int32) ([]RecipeImport, error)
	Count(ctx context.Context, status string) (int64, error)
	ListByStatuses(ctx context.Context, statuses []Status, limit, offset int32) ([]RecipeImport, error)
	CountByStatuses(ctx context.Context, statuses []Status) (int64, error)
	ListClaimableIDs(ctx context.Context) ([]int64, error)
	ResetProcessing(ctx context.Context) error
	// Claim moves a claimable job to processing and returns the updated row.
	// It returns domainerr.ErrConflict when the job is not claimable.
	Claim(ctx context.Context, id int64) (*RecipeImport, error)
	UpdateOCR(ctx context.Context, id int64, ocrText string, ocrJSON []byte) error
	UpdateDraft(ctx context.Context, id int64, draftJSON []byte) error
	UpdateReview(ctx context.Context, id int64, reviewJSON []byte, status Status, updatedBy string) error
	MarkProfanity(ctx context.Context, id int64, reason string) error
	MarkFailed(ctx context.Context, id int64, errorMessage string) error
	SetPersisted(ctx context.Context, id int64, recipeID int64, approvedByUserID int64) error
	SetRejected(ctx context.Context, id int64) error
	SetPending(ctx context.Context, id int64) error
	WithTx(tx pgx.Tx) Store
}

// SQLStore is the Postgres-backed Store implementation.
type SQLStore struct {
	q *sqlc.Queries
}

// NewStore creates a Store backed by the given pool. The querier resolves a
// ctx-carried transaction first (see dbtx.ContextExecer) so calls made inside
// a UnitOfWork join that transaction automatically.
func NewStore(pool dbtx.Pool) Store {
	return &SQLStore{q: sqlc.New(dbtx.NewTimedExecer(dbtx.ContextExecer(pool), "recipeimport"))}
}

// WithTx returns a Store whose queries run on the provided transaction.
func (s *SQLStore) WithTx(tx pgx.Tx) Store {
	return &SQLStore{q: sqlc.New(dbtx.NewTimedExecer(tx, "recipeimport"))}
}

// conflictOnNoRows maps a zero-row conditional transition to ErrConflict.
func conflictOnNoRows(n int64, id int64) error {
	if n == 0 {
		return fmt.Errorf("recipe import %d: %w", id, domainerr.ErrConflict)
	}
	return nil
}

func toRecipeImport(row sqlc.RecipeRecipeImport) *RecipeImport {
	ri := &RecipeImport{
		ID:              row.RecipeImportID,
		SourceFilename:  row.SourceFilename,
		SourcePath:      row.SourcePath,
		SourceHash:      row.SourceHash.String,
		OCRText:         row.OcrText.String,
		OCRJSON:         row.OcrJson,
		DraftJSON:       row.DraftJson,
		ReviewJSON:      row.ReviewJson,
		ProfanityFlag:   row.ProfanityFlag,
		ProfanityReason: row.ProfanityReason.String,
		Status:          Status(row.Status),
		ErrorMessage:    row.ErrorMessage.String,
		CreatedBy:       row.CreatedBy,
		UpdatedBy:       row.UpdatedBy.String,
		CreatedAt:       row.CreatedAt,
	}
	if row.SubmittedByUserID.Valid {
		v := row.SubmittedByUserID.Int64
		ri.SubmittedByUserID = &v
	}
	if row.RecipeID.Valid {
		v := row.RecipeID.Int64
		ri.RecipeID = &v
	}
	if row.ApprovedByUserID.Valid {
		v := row.ApprovedByUserID.Int64
		ri.ApprovedByUserID = &v
	}
	if row.UpdatedAt.Valid {
		ri.UpdatedAt = &row.UpdatedAt.Time
	}
	if row.ApprovedAt.Valid {
		ri.ApprovedAt = &row.ApprovedAt.Time
	}
	return ri
}

// Create inserts a new recipe import row.
func (s *SQLStore) Create(ctx context.Context, ri RecipeImport) (*RecipeImport, error) {
	submitted := pgtype.Int8{Valid: false}
	if ri.SubmittedByUserID != nil {
		submitted = pgtype.Int8{Int64: *ri.SubmittedByUserID, Valid: true}
	}
	updatedBy := pgtype.Text{Valid: false}
	if ri.UpdatedBy != "" {
		updatedBy = pgtype.Text{String: ri.UpdatedBy, Valid: true}
	}
	row, err := s.q.CreateRecipeImport(ctx, sqlc.CreateRecipeImportParams{
		SubmittedByUserID: submitted,
		SourceFilename:    ri.SourceFilename,
		SourcePath:        ri.SourcePath,
		SourceHash:        pgtype.Text{String: ri.SourceHash, Valid: ri.SourceHash != ""},
		Status:            string(ri.Status),
		CreatedBy:         ri.CreatedBy,
		UpdatedBy:         updatedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("create recipe import: %w", err)
	}
	return toRecipeImport(row), nil
}

// Get returns a recipe import by id.
func (s *SQLStore) Get(ctx context.Context, id int64) (*RecipeImport, error) {
	row, err := s.q.GetRecipeImport(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get recipe import: %w", domainerr.FromStorage(err))
	}
	return toRecipeImport(row), nil
}

// List returns a paged list of recipe imports, optionally filtered by status.
func (s *SQLStore) List(ctx context.Context, status string, limit, offset int32) ([]RecipeImport, error) {
	rows, err := s.q.ListRecipeImports(ctx, sqlc.ListRecipeImportsParams{
		Column1: status,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list recipe imports: %w", err)
	}
	out := make([]RecipeImport, len(rows))
	for i, row := range rows {
		out[i] = *toRecipeImport(row)
	}
	return out, nil
}

// Count returns the number of recipe imports, optionally filtered by status.
func (s *SQLStore) Count(ctx context.Context, status string) (int64, error) {
	n, err := s.q.CountRecipeImports(ctx, status)
	if err != nil {
		return 0, fmt.Errorf("count recipe imports: %w", err)
	}
	return n, nil
}

// ListByStatuses returns a paged list of recipe imports in any of the given
// statuses, ordered newest first. One query replaces the previous
// per-status merge, which applied the same offset to every status and could
// skip or duplicate rows.
func (s *SQLStore) ListByStatuses(ctx context.Context, statuses []Status, limit, offset int32) ([]RecipeImport, error) {
	rows, err := s.q.ListRecipeImportsByStatuses(ctx, sqlc.ListRecipeImportsByStatusesParams{
		Column1: statusStrings(statuses),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list recipe imports by statuses: %w", err)
	}
	out := make([]RecipeImport, len(rows))
	for i, row := range rows {
		out[i] = *toRecipeImport(row)
	}
	return out, nil
}

// CountByStatuses returns the number of recipe imports in any of the given
// statuses.
func (s *SQLStore) CountByStatuses(ctx context.Context, statuses []Status) (int64, error) {
	n, err := s.q.CountRecipeImportsByStatuses(ctx, statusStrings(statuses))
	if err != nil {
		return 0, fmt.Errorf("count recipe imports by statuses: %w", err)
	}
	return n, nil
}

// ListClaimableIDs returns ids of jobs a worker may claim, oldest first.
func (s *SQLStore) ListClaimableIDs(ctx context.Context) ([]int64, error) {
	ids, err := s.q.ListClaimableRecipeImportIDs(ctx, statusStrings(claimableStatuses))
	if err != nil {
		return nil, fmt.Errorf("list claimable recipe imports: %w", err)
	}
	return ids, nil
}

// ResetProcessing moves orphaned processing rows back to pending. Called once
// at service start; a row left in processing means the worker that claimed it
// died before reaching a staged status.
func (s *SQLStore) ResetProcessing(ctx context.Context) error {
	if err := s.q.ResetProcessingRecipeImports(ctx); err != nil {
		return fmt.Errorf("reset processing recipe imports: %w", err)
	}
	return nil
}

// Claim moves a claimable job to processing atomically. Exactly one claimant
// wins; concurrent or repeated claims get domainerr.ErrConflict.
func (s *SQLStore) Claim(ctx context.Context, id int64) (*RecipeImport, error) {
	row, err := s.q.ClaimRecipeImport(ctx, sqlc.ClaimRecipeImportParams{
		RecipeImportID: id,
		Column2:        statusStrings(claimableStatuses),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("claim recipe import %d: %w", id, domainerr.ErrConflict)
		}
		return nil, fmt.Errorf("claim recipe import: %w", err)
	}
	return toRecipeImport(row), nil
}

// UpdateOCR stores OCR text and JSON (processing -> ocred).
func (s *SQLStore) UpdateOCR(ctx context.Context, id int64, ocrText string, ocrJSON []byte) error {
	n, err := s.q.UpdateRecipeImportOCR(ctx, sqlc.UpdateRecipeImportOCRParams{
		RecipeImportID: id,
		OcrText:        pgtype.Text{String: ocrText, Valid: ocrText != ""},
		OcrJson:        ocrJSON,
	})
	if err != nil {
		return fmt.Errorf("update ocr: %w", err)
	}
	return conflictOnNoRows(n, id)
}

// UpdateDraft stores the LLM draft JSON (ocred -> drafted).
func (s *SQLStore) UpdateDraft(ctx context.Context, id int64, draftJSON []byte) error {
	n, err := s.q.UpdateRecipeImportDraft(ctx, sqlc.UpdateRecipeImportDraftParams{
		RecipeImportID: id,
		DraftJson:      draftJSON,
	})
	if err != nil {
		return fmt.Errorf("update draft: %w", err)
	}
	return conflictOnNoRows(n, id)
}

// UpdateReview stores the review JSON and moves to reviewing or ready.
// Legal sources are drafted (worker), reviewing and ready (admin edits).
func (s *SQLStore) UpdateReview(ctx context.Context, id int64, reviewJSON []byte, status Status, updatedBy string) error {
	n, err := s.q.UpdateRecipeImportReview(ctx, sqlc.UpdateRecipeImportReviewParams{
		RecipeImportID: id,
		ReviewJson:     reviewJSON,
		Status:         string(status),
		UpdatedBy:      pgtype.Text{String: updatedBy, Valid: updatedBy != ""},
		Column5:        []string{string(StatusDrafted), string(StatusReviewing), string(StatusReady)},
	})
	if err != nil {
		return fmt.Errorf("update review: %w", err)
	}
	return conflictOnNoRows(n, id)
}

// MarkProfanity sets the profanity flag and status (worker stages only).
func (s *SQLStore) MarkProfanity(ctx context.Context, id int64, reason string) error {
	n, err := s.q.MarkRecipeImportProfanity(ctx, sqlc.MarkRecipeImportProfanityParams{
		RecipeImportID:  id,
		ProfanityReason: pgtype.Text{String: reason, Valid: reason != ""},
		Column3:         []string{string(StatusProcessing), string(StatusOCRED), string(StatusDrafted)},
	})
	if err != nil {
		return fmt.Errorf("mark profanity: %w", err)
	}
	return conflictOnNoRows(n, id)
}

// MarkFailed sets status failed (worker stages only; it cannot regress a
// rejected or persisted job).
func (s *SQLStore) MarkFailed(ctx context.Context, id int64, errorMessage string) error {
	n, err := s.q.MarkRecipeImportFailed(ctx, sqlc.MarkRecipeImportFailedParams{
		RecipeImportID: id,
		ErrorMessage:   pgtype.Text{String: errorMessage, Valid: errorMessage != ""},
		Column3:        []string{string(StatusProcessing), string(StatusOCRED), string(StatusDrafted)},
	})
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return conflictOnNoRows(n, id)
}

// SetPersisted links the persisted recipe and records the approver.
// Only a ready (or in-review) import may persist, so a second Approve on the
// same job is a conflict and cannot create a duplicate recipe.
func (s *SQLStore) SetPersisted(ctx context.Context, id int64, recipeID int64, approvedByUserID int64) error {
	n, err := s.q.SetRecipeImportPersisted(ctx, sqlc.SetRecipeImportPersistedParams{
		RecipeImportID:   id,
		RecipeID:         pgtype.Int8{Int64: recipeID, Valid: true},
		ApprovedByUserID: pgtype.Int8{Int64: approvedByUserID, Valid: true},
		Column4:          fromStatuses(StatusPersisted),
	})
	if err != nil {
		return fmt.Errorf("set persisted: %w", err)
	}
	return conflictOnNoRows(n, id)
}

// SetRejected sets status rejected from any non-terminal state.
func (s *SQLStore) SetRejected(ctx context.Context, id int64) error {
	n, err := s.q.SetRecipeImportRejected(ctx, sqlc.SetRecipeImportRejectedParams{
		RecipeImportID: id,
		Column2:        fromStatuses(StatusRejected),
	})
	if err != nil {
		return fmt.Errorf("set rejected: %w", err)
	}
	return conflictOnNoRows(n, id)
}

// SetPending resets a failed or quarantined job to pending for retry.
func (s *SQLStore) SetPending(ctx context.Context, id int64) error {
	n, err := s.q.SetRecipeImportPending(ctx, sqlc.SetRecipeImportPendingParams{
		RecipeImportID: id,
		Column2:        fromStatuses(StatusPending),
	})
	if err != nil {
		return fmt.Errorf("set pending: %w", err)
	}
	return conflictOnNoRows(n, id)
}
