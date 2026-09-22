package bff

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
)

// SubmitRecipeScan accepts a base64-encoded recipe scan (PNG/JPG/PDF) from
// an admin, validates it, and hands the bytes to the import service, which
// owns the inbox write and job row. It returns the created RecipeImport.
func (r *Resolver) SubmitRecipeScan(ctx context.Context, args struct {
	FileBase64 string
}) (*recipeImportResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}

	encoded := strings.TrimSpace(args.FileBase64)
	if encoded == "" {
		return nil, badInputf("fileBase64 is required")
	}

	mediaType, data, err := splitDataURI(encoded)
	if err != nil {
		return nil, badInputf("invalid fileBase64 data URI: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, badInputf("fileBase64 is not valid base64")
	}

	maxBytes := r.RecipeScanMaxBytes
	if maxBytes == 0 {
		maxBytes = 20 * 1024 * 1024
	}
	if len(decoded) > maxBytes {
		return nil, badInputf("recipe scan exceeds maximum size of %d bytes", maxBytes)
	}

	sniffed, err := sniffUpload(decoded, mediaType)
	if err != nil {
		return nil, badInputf("recipe scan rejected: %v", err)
	}
	if !r.uploadLimiter().allow(u.UserID) {
		return nil, &clientError{msg: "too many uploads; try again later", code: codeBusy}
	}

	ri, err := r.RecipeImportService.Submit(ctx, sniffed, decoded, &u.UserID, u.Email)
	if err != nil {
		return nil, err
	}
	return &recipeImportResolver{ri: ri, inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService}, nil
}

// splitDataURI extracts the media type and base64 payload from a data URI.
func splitDataURI(s string) (string, string, error) {
	if !strings.HasPrefix(s, "data:") {
		return "", s, nil
	}
	parts := strings.SplitN(s, ",", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed data URI")
	}
	header := parts[0]
	// header is "data:<media-type>;base64" or similar.
	mediaType := ""
	if idx := strings.Index(header, ";"); idx != -1 {
		mediaType = header[len("data:"):idx]
	} else {
		mediaType = header[len("data:"):]
	}
	return mediaType, parts[1], nil
}
