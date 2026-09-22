// Package recipeimport is an application service: it orchestrates the
// server-side recipe OCR pipeline across the recipe and inventory domains
// and stores the resulting artifacts in its own recipe_import schema.
package recipeimport

import (
	"time"

	"github.com/JRAdams472/LENA2/internal/ocrimport"
)

// RecipeImport mirrors the recipe.recipe_import database row.
type RecipeImport struct {
	ID                int64
	SubmittedByUserID *int64
	SourceFilename    string
	SourcePath        string
	SourceHash        string
	OCRText           string
	OCRJSON           []byte
	DraftJSON         []byte
	ReviewJSON        []byte
	ProfanityFlag     bool
	ProfanityReason   string
	Status            Status
	RecipeID          *int64
	ErrorMessage      string
	CreatedBy         string
	UpdatedBy         string
	CreatedAt         time.Time
	UpdatedAt         *time.Time
	ApprovedByUserID  *int64
	ApprovedAt        *time.Time
}

// ReviewRecipe and MatchResult are reused from the ocrimport package.
type ReviewRecipe = ocrimport.ReviewRecipe

// MatchResult is reused from the ocrimport package.
type MatchResult = ocrimport.MatchResult
