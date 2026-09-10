package bff

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SubmitRecipeScan accepts a base64-encoded recipe scan (PNG/JPG/PDF) from an
// admin, writes it to the configured import inbox, and enqueues server-side
// OCR processing. It returns the created RecipeImport.
func (r *Resolver) SubmitRecipeScan(ctx context.Context, args struct {
	FileBase64 string
}) (*recipeImportResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}

	inbox := strings.TrimSpace(r.ImportInbox)
	if inbox == "" {
		return nil, badInputf("import inbox is not configured")
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

	ext := extensionForMediaType(mediaType)
	if ext == "" {
		return nil, badInputf("unsupported media type %q", mediaType)
	}

	// Sanitize the target directory so files are written only inside the inbox.
	if err := os.MkdirAll(inbox, 0o700); err != nil {
		return nil, fmt.Errorf("create import inbox: %w", err)
	}

	suffix, err := randomHex(8)
	if err != nil {
		return nil, fmt.Errorf("generate filename: %w", err)
	}
	filename := fmt.Sprintf("scan-%d-%s%s", time.Now().UnixMilli(), suffix, ext)
	path := filepath.Join(inbox, filename)
	// #nosec G304 -- path is constructed inside the configured inbox using a generated filename.
	if err := os.WriteFile(path, decoded, 0o600); err != nil {
		return nil, fmt.Errorf("write recipe scan: %w", err)
	}

	sourceHash := hashBytes(decoded)
	submittedBy := &u.UserID
	ri, err := r.RecipeImportService.Create(ctx, filename, path, sourceHash, submittedBy, u.Email)
	if err != nil {
		return nil, fmt.Errorf("create recipe import: %w", err)
	}
	return &recipeImportResolver{ri: ri, inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService}, nil
}

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
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

// extensionForMediaType maps common image/PDF media types to file extensions.
func extensionForMediaType(mediaType string) string {
	mt := strings.ToLower(strings.TrimSpace(mediaType))
	switch {
	case mt == "application/pdf", mt == "pdf":
		return ".pdf"
	case mt == "image/png":
		return ".png"
	case mt == "image/jpeg", mt == "image/jpg":
		return ".jpg"
	case strings.HasPrefix(mt, "image/"):
		return ".png"
	case mediaType == "":
		return ".png"
	default:
		return ""
	}
}

// randomHex returns a cryptographically random hex suffix for uploaded
// filenames to avoid name collisions.
func randomHex(n int) (string, error) {
	b := make([]byte, n/2)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:n], nil
}
