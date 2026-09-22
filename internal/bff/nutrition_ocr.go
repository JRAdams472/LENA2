package bff

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/inventory/nutritionparse"
)

// nutrientLabelMaxLen matches the implicit catalog limit for nutrient type
// names; anything longer is not a real nutrient and is ignored.
const nutrientLabelMaxLen = 100

// processNutritionPhoto decodes the image, extracts text via OCR, parses the
// text into nutrients, and writes the resulting nutrient rows to the item.
// Unknown nutrient labels are not auto-created; only existing catalog types
// are applied so non-admin members cannot add global reference data.
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

	entries := make([]inventory.NutrientEntry, 0, len(parsed))
	matchedTypes := 0
	skipped := 0
	unknown := 0
	for _, n := range parsed {
		label := strings.TrimSpace(n.Label)
		if label == "" || len(label) > nutrientLabelMaxLen {
			skipped++
			continue
		}
		nt, err := inv.GetNutrientTypeByName(ctx, label)
		if err != nil {
			// Unknown labels are not created here; admins use createNutrientType.
			unknown++
			continue
		}
		matchedTypes++
		entries = append(entries, inventory.NutrientEntry{
			NutrientID: nt.NutrientID,
			Amount:     n.Amount,
		})
	}

	if err := inv.SetItemNutrients(ctx, itemID, entries, by); err != nil {
		return fmt.Errorf("set item nutrients: %w", err)
	}

	slog.Default().Info("nutrition ocr completed",
		"item_id", itemID,
		"parsed", len(parsed),
		"matched", matchedTypes,
		"unknown", unknown,
		"skipped", skipped,
		"by", by,
	)
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
