package recipeimport

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/recipeimport/sqlc"
)

// Store is the persistence interface for recipe import jobs.
type Store interface {
	Create(ctx context.Context, ri RecipeImport) (*RecipeImport, error)
	Get(ctx context.Context, id int64) (*RecipeImport, error)
	List(ctx context.Context, status string, limit, offset int32) ([]RecipeImport, error)
	Count(ctx context.Context, status string) (int64, error)
	UpdateOCR(ctx context.Context, id int64, ocrText string, ocrJSON []byte) error
	UpdateDraft(ctx context.Context, id int64, draftJSON []byte) error
	UpdateReview(ctx context.Context, id int64, reviewJSON []byte, status string, updatedBy string) error
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

// NewStore creates a Store backed by the given pool.
func NewStore(pool dbtx.Pool) Store {
	return &SQLStore{q: sqlc.New(dbtx.NewTimedExecer(pool, "recipeimport"))}
}

// WithTx returns a Store whose queries run on the provided transaction.
func (s *SQLStore) WithTx(tx pgx.Tx) Store {
	return &SQLStore{q: sqlc.New(dbtx.NewTimedExecer(tx, "recipeimport"))}
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
		Status:          row.Status,
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
		Status:            ri.Status,
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
		return nil, fmt.Errorf("get recipe import: %w", err)
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

// UpdateOCR stores OCR text and JSON.
func (s *SQLStore) UpdateOCR(ctx context.Context, id int64, ocrText string, ocrJSON []byte) error {
	if err := s.q.UpdateRecipeImportOCR(ctx, sqlc.UpdateRecipeImportOCRParams{
		RecipeImportID: id,
		OcrText:        pgtype.Text{String: ocrText, Valid: ocrText != ""},
		OcrJson:        ocrJSON,
	}); err != nil {
		return fmt.Errorf("update ocr: %w", err)
	}
	return nil
}

// UpdateDraft stores the LLM draft JSON and advances status to drafted.
func (s *SQLStore) UpdateDraft(ctx context.Context, id int64, draftJSON []byte) error {
	if err := s.q.UpdateRecipeImportDraft(ctx, sqlc.UpdateRecipeImportDraftParams{
		RecipeImportID: id,
		DraftJson:      draftJSON,
	}); err != nil {
		return fmt.Errorf("update draft: %w", err)
	}
	return nil
}

// UpdateReview stores the review JSON and status.
func (s *SQLStore) UpdateReview(ctx context.Context, id int64, reviewJSON []byte, status string, updatedBy string) error {
	if err := s.q.UpdateRecipeImportReview(ctx, sqlc.UpdateRecipeImportReviewParams{
		RecipeImportID: id,
		ReviewJson:     reviewJSON,
		Status:         status,
		UpdatedBy:      pgtype.Text{String: updatedBy, Valid: updatedBy != ""},
	}); err != nil {
		return fmt.Errorf("update review: %w", err)
	}
	return nil
}

// MarkProfanity sets the profanity flag and status.
func (s *SQLStore) MarkProfanity(ctx context.Context, id int64, reason string) error {
	if err := s.q.MarkRecipeImportProfanity(ctx, sqlc.MarkRecipeImportProfanityParams{
		RecipeImportID:  id,
		ProfanityReason: pgtype.Text{String: reason, Valid: reason != ""},
	}); err != nil {
		return fmt.Errorf("mark profanity: %w", err)
	}
	return nil
}

// MarkFailed sets status failed and stores the error message.
func (s *SQLStore) MarkFailed(ctx context.Context, id int64, errorMessage string) error {
	if err := s.q.MarkRecipeImportFailed(ctx, sqlc.MarkRecipeImportFailedParams{
		RecipeImportID: id,
		ErrorMessage:   pgtype.Text{String: errorMessage, Valid: errorMessage != ""},
	}); err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return nil
}

// SetPersisted links the persisted recipe and records the approver.
func (s *SQLStore) SetPersisted(ctx context.Context, id int64, recipeID int64, approvedByUserID int64) error {
	if err := s.q.SetRecipeImportPersisted(ctx, sqlc.SetRecipeImportPersistedParams{
		RecipeImportID:   id,
		RecipeID:         pgtype.Int8{Int64: recipeID, Valid: true},
		ApprovedByUserID: pgtype.Int8{Int64: approvedByUserID, Valid: true},
	}); err != nil {
		return fmt.Errorf("set persisted: %w", err)
	}
	return nil
}

// SetRejected sets status rejected.
func (s *SQLStore) SetRejected(ctx context.Context, id int64) error {
	if err := s.q.SetRecipeImportRejected(ctx, id); err != nil {
		return fmt.Errorf("set rejected: %w", err)
	}
	return nil
}

// SetPending resets the job to pending for retry.
func (s *SQLStore) SetPending(ctx context.Context, id int64) error {
	if err := s.q.SetRecipeImportPending(ctx, id); err != nil {
		return fmt.Errorf("set pending: %w", err)
	}
	return nil
}
