package bff

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go"

	"github.com/JRAdams472/LENA2/internal/inventory/nutritionparse"
)

// processNutritionPhoto decodes the image, extracts text via OCR, parses
// the text into nutrients, and hands them to the inventory domain, which
// owns nutrient-type matching and the nutrient write path.
func processNutritionPhoto(ctx context.Context, inv InventoryService, ocr OCRClient, itemID int64, image []byte, by string) error {
	text, err := ocr.ExtractText(ctx, image)
	if err != nil {
		return fmt.Errorf("ocr extract text: %w", err)
	}

	parsed := nutritionparse.Parse(text)
	if len(parsed) == 0 {
		slog.Default().Info("nutrition ocr produced no parseable nutrients", "item_id", itemID)
		return nil
	}

	if err := inv.ApplyNutritionLabel(ctx, itemID, parsed, by); err != nil {
		return fmt.Errorf("apply nutrition label: %w", err)
	}
	return nil
}

// SubmitItemNutritionPhoto accepts a base64-encoded nutrition label photo,
// validates it, and queues the OCR + nutrient parsing as a background task.
// The mutation returns immediately once the job is queued.
func (r *Resolver) SubmitItemNutritionPhoto(ctx context.Context, args struct {
	ItemID      graphql.ID
	PhotoBase64 string
}) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return false, err
	}
	it, err := r.InventoryService.GetItemByID(ctx, itemID)
	if err != nil {
		return false, err
	}
	if !canModifyItem(it, u) {
		return false, errForbidden()
	}

	photo := strings.TrimSpace(args.PhotoBase64)
	if photo == "" {
		return false, badInputf("photoBase64 is required")
	}
	decoded, err := base64.StdEncoding.DecodeString(photo)
	if err != nil {
		return false, badInputf("photoBase64 is not valid base64")
	}
	maxBytes := r.NutritionPhotoMaxBytes
	if maxBytes == 0 {
		maxBytes = 6 * 1024 * 1024
	}
	if len(decoded) > maxBytes {
		return false, badInputf("photo exceeds maximum size of %d bytes", maxBytes)
	}
	if sniffed, err := sniffUpload(decoded, ""); err != nil {
		return false, badInputf("photo rejected: %v", err)
	} else if sniffed == "application/pdf" {
		return false, badInputf("photo must be an image, not a pdf")
	}

	r.runAsync("nutrition-ocr", 30*time.Second, func(ctx context.Context) error {
		return processNutritionPhoto(ctx, r.InventoryService, r.OCRClient, itemID, decoded, u.Email)
	})
	return true, nil
}
