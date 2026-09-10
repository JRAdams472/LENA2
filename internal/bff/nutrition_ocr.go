package bff

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/jackc/pgx/v5"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/inventory/nutritionparse"
)

// processNutritionPhoto decodes the image, extracts text via OCR, parses the
// text into nutrients, and writes the resulting nutrient rows to the item.
func processNutritionPhoto(ctx context.Context, inv InventoryService, ocr OCRClient, itemID int64, image []byte) error {
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
	createdTypes := 0
	matchedTypes := 0
	skipped := 0
	for _, n := range parsed {
		label := strings.TrimSpace(n.Label)
		if label == "" {
			skipped++
			continue
		}
		nt, err := inv.GetNutrientTypeByName(ctx, label)
		if err != nil {
			if !strings.Contains(err.Error(), pgx.ErrNoRows.Error()) {
				return fmt.Errorf("get nutrient type by name: %w", err)
			}
			// Create a new nutrient type from the OCR text.
			nt, err = inv.CreateNutrientType(ctx, label, n.Unit)
			if err != nil {
				return fmt.Errorf("create nutrient type %q: %w", label, err)
			}
			createdTypes++
		} else {
			matchedTypes++
		}
		entries = append(entries, inventory.NutrientEntry{
			NutrientID: nt.NutrientID,
			Amount:     n.Amount,
		})
	}

	if err := inv.SetItemNutrients(ctx, itemID, entries, "ocr-system"); err != nil {
		return fmt.Errorf("set item nutrients: %w", err)
	}

	slog.Default().Info("nutrition ocr completed",
		"item_id", itemID,
		"parsed", len(parsed),
		"matched", matchedTypes,
		"created", createdTypes,
		"skipped", skipped,
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

	r.runAsync("nutrition-ocr", 30*time.Second, func(ctx context.Context) error {
		return processNutritionPhoto(ctx, r.InventoryService, r.OCRClient, itemID, decoded)
	})
	return true, nil
}
