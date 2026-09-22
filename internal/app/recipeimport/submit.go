package recipeimport

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Submit stores an uploaded scan in the configured import inbox and creates
// the pending import row, so the file write and its cleanup live with the
// same component that owns the job record. A failed insert removes the
// orphaned inbox file.
func (s *Service) Submit(ctx context.Context, mediaType string, data []byte, submittedByUserID *int64, by string) (*RecipeImport, error) {
	inbox := strings.TrimSpace(s.cfg.InboxDir)
	if inbox == "" {
		return nil, errors.New("import inbox is not configured")
	}
	ext := extensionForMediaType(mediaType)
	if ext == "" {
		return nil, fmt.Errorf("unsupported media type %q", mediaType)
	}

	// The filename is generated, so the write always lands inside the
	// configured inbox.
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
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, fmt.Errorf("write recipe scan: %w", err)
	}

	ri, err := s.Create(ctx, filename, path, hashBytes(data), submittedByUserID, by)
	if err != nil {
		if rmErr := os.Remove(path); rmErr != nil {
			slog.Default().Warn("remove orphaned recipe scan failed", "path", path, "error", rmErr)
		}
		return nil, fmt.Errorf("create recipe import: %w", err)
	}
	return ri, nil
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

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
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
