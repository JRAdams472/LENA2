// Command ocrimport imports printed recipes into LENA2 from scanned pages
// using local OCR and a local LLM.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JRAdams472/LENA2/internal/ocrimport"
	"github.com/JRAdams472/LENA2/internal/platform/bffclient"
	"github.com/JRAdams472/LENA2/internal/platform/config"
	"github.com/JRAdams472/LENA2/internal/platform/ocrclient"
	"github.com/JRAdams472/LENA2/internal/platform/ollamaclient"
)

const version = "0.0.1"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	switch cmd {
	case "import":
		importCmd()
	case "ocr":
		ocrCmd()
	case "draft":
		draftCmd()
	case "review":
		reviewCmd()
	case "persist":
		persistCmd()
	case "version", "-v", "--version":
		fmt.Println("ocrimport", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: ocrimport <command> [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  import   Scan --inbox and enqueue source pages")
	fmt.Fprintln(os.Stderr, "  ocr      Run the local OCR service over pending source pages")
	fmt.Fprintln(os.Stderr, "  draft    Convert OCR text to a RecipeDraft JSON using the local LLM")
	fmt.Fprintln(os.Stderr, "  review   Map draft ingredients to the catalog and produce a review report")
	fmt.Fprintln(os.Stderr, "  persist  Persist approved reviews to LENA2 via createRecipe")
	fmt.Fprintln(os.Stderr, "  version  Print version")
	fmt.Fprintln(os.Stderr, "  help     Show this help")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "All commands load configuration from LENA_* environment variables.")
}

func importCmd() {
	var inbox string
	var workDir string
	fset := flagSet("import")
	fset.StringVar(&inbox, "inbox", "", "Directory containing scanned recipe pages (png, jpg, pdf); defaults to LENA_IMPORT_INBOX")
	fset.StringVar(&workDir, "work-dir", "", "Work directory; defaults to LENA_IMPORT_WORK_DIR")
	if err := fset.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "parse import flags: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	if inbox == "" {
		inbox = cfg.ImportInbox
	}
	if workDir == "" {
		workDir = cfg.ImportWorkDir
	}

	queue, err := ocrimport.Open(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open work queue: %v\n", err)
		os.Exit(1)
	}

	count := 0
	if err := filepath.WalkDir(inbox, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".pdf" {
			return nil
		}
		id, err := queue.Add(path)
		if err != nil {
			return fmt.Errorf("enqueue %s: %w", path, err)
		}
		fmt.Printf("enqueued %s as %s\n", path, id)
		count++
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "walk inbox: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("import: enqueued %d page(s)\n", count)
	fmt.Printf("work queue: %s\n", workDir)
	fmt.Println("Next: run 'ocrimport ocr' to extract text.")
}

func ocrCmd() {
	var workDir string
	fset := flagSet("ocr")
	fset.StringVar(&workDir, "work-dir", "", "Work directory; defaults to LENA_IMPORT_WORK_DIR")
	if err := fset.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "parse ocr flags: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	if cfg.OCRServiceURL == "" {
		fmt.Fprintln(os.Stderr, "OCR is not configured.")
		fmt.Fprintln(os.Stderr, "Set LENA_OCR_SERVICE_URL to enable it.")
		os.Exit(1)
	}

	if workDir == "" {
		workDir = cfg.ImportWorkDir
	}

	queue, err := ocrimport.Open(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open work queue: %v\n", err)
		os.Exit(1)
	}

	client := ocrclient.New(cfg.OCRServiceURL, cfg.OCRTimeout)
	ctx := context.Background()

	pending := queue.List()
	processed, failed := 0, 0
	for _, page := range pending {
		if page.Status != ocrimport.StatusPending {
			continue
		}

		data, err := os.ReadFile(page.SourcePath)
		if err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("read file: %v", err))
			fmt.Fprintf(os.Stderr, "read %s: %v\n", page.SourcePath, err)
			failed++
			continue
		}

		res, err := client.ExtractTextResult(ctx, data, filepath.Base(page.SourcePath))
		if err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("ocr: %v", err))
			fmt.Fprintf(os.Stderr, "ocr %s: %v\n", page.SourcePath, err)
			failed++
			continue
		}

		minConf := float64(100)
		for _, p := range res.Pages {
			if p.MeanConfidence < minConf {
				minConf = p.MeanConfidence
			}
		}
		if minConf < float64(cfg.OCRConfidenceThreshold) {
			msg := fmt.Sprintf("mean confidence %.2f below threshold %d", minConf, cfg.OCRConfidenceThreshold)
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, msg)
			fmt.Fprintf(os.Stderr, "%s: %s\n", page.SourcePath, msg)
			failed++
			continue
		}

		pageDir := filepath.Join(workDir, filepath.Base(page.ID))
		if err := os.MkdirAll(pageDir, 0o750); err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("create page dir: %v", err))
			failed++
			continue
		}
		// #nosec G703 -- pageDir is re-derived from the importer-generated queue id and sanitized with filepath.Base/Clean above.
		if err := os.WriteFile(filepath.Join(pageDir, "ocr.txt"), []byte(res.Text), 0o600); err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("write ocr.txt: %v", err))
			failed++
			continue
		}

		jsonData, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("marshal json: %v", err))
			failed++
			continue
		}
		// #nosec G703 -- pageDir is re-derived from the importer-generated queue id and sanitized with filepath.Base/Clean above.
		if err := os.WriteFile(filepath.Join(pageDir, "ocr.json"), jsonData, 0o600); err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("write ocr.json: %v", err))
			failed++
			continue
		}

		if err := queue.SetStatus(page.ID, ocrimport.StatusOCR, ""); err != nil {
			fmt.Fprintf(os.Stderr, "update status for %s: %v\n", page.SourcePath, err)
			failed++
			continue
		}

		fmt.Printf("ocr: %s -> %d line(s)\n", page.SourcePath, len(res.Lines))
		processed++
	}

	fmt.Printf("ocr: processed %d, failed %d\n", processed, failed)
}

func draftCmd() {
	var workDir string
	fset := flagSet("draft")
	fset.StringVar(&workDir, "work-dir", "", "Work directory; defaults to LENA_IMPORT_WORK_DIR")
	if err := fset.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "parse draft flags: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	if cfg.OllamaURL == "" {
		fmt.Fprintln(os.Stderr, "Ollama is not configured.")
		fmt.Fprintln(os.Stderr, "Set LENA_OLLAMA_URL to enable it.")
		os.Exit(1)
	}

	if workDir == "" {
		workDir = cfg.ImportWorkDir
	}

	queue, err := ocrimport.Open(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open work queue: %v\n", err)
		os.Exit(1)
	}

	client := ollamaclient.New(cfg.OllamaURL, cfg.OllamaModel, cfg.OllamaTemperature, cfg.OllamaNumCtx)
	ctx := context.Background()

	schemaJSON, _ := json.MarshalIndent(ocrimport.JSONSchema(), "", "  ")
	systemPrompt := "You are a precise recipe transcription assistant. " +
		"You are given OCR text extracted from a scanned recipe page. " +
		"Your job is to return a single JSON object matching this JSON Schema and nothing else. " +
		"Do not add, improve, or invent anything. Use null for missing fields. " +
		"Convert fractional quantities like \"1 1/2\" to decimals like 1.5. " +
		"For ranges such as \"2-3\", use the lower value and put the range in notes. " +
		"The ingredient should be the bare noun phrase; preparation goes in notes.\n\n" +
		"JSON Schema:\n" + string(schemaJSON)

	pending := queue.List()
	processed, failed := 0, 0
	for _, page := range pending {
		if page.Status != ocrimport.StatusOCR {
			continue
		}

		pageDir := filepath.Join(workDir, filepath.Base(page.ID))
		// #nosec G304 -- pageDir is re-derived from the importer-generated queue id and sanitized with filepath.Base.
		ocrText, err := os.ReadFile(filepath.Join(pageDir, "ocr.txt"))
		if err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("read ocr.txt: %v", err))
			fmt.Fprintf(os.Stderr, "read ocr.txt for %s: %v\n", page.ID, err)
			failed++
			continue
		}

		var logLines []string
		userPrompt := "OCR text:\n" + string(ocrText)

		content, err := client.Chat(ctx, systemPrompt, userPrompt)
		if err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("ollama: %v", err))
			fmt.Fprintf(os.Stderr, "ollama %s: %v\n", page.ID, err)
			failed++
			continue
		}
		logLines = append(logLines, "--- LLM attempt 1 ---", content)

		var draft ocrimport.RecipeDraft
		if err := json.Unmarshal([]byte(content), &draft); err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("parse draft: %v", err))
			fmt.Fprintf(os.Stderr, "parse draft %s: %v\n", page.ID, err)
			failed++
			continue
		}

		if vErr := ocrimport.ValidateDraft(&draft); vErr != nil {
			retryPrompt := userPrompt + "\n\nYour previous response did not validate. " +
				"Fix it and return only the corrected JSON object. Validation errors:\n" + vErr.Error()
			content, err = client.Chat(ctx, systemPrompt, retryPrompt)
			if err != nil {
				_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("ollama retry: %v", err))
				fmt.Fprintf(os.Stderr, "ollama retry %s: %v\n", page.ID, err)
				failed++
				continue
			}
			logLines = append(logLines, "--- LLM retry ---", content)
			if err := json.Unmarshal([]byte(content), &draft); err != nil {
				_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("parse retry draft: %v", err))
				fmt.Fprintf(os.Stderr, "parse retry draft %s: %v\n", page.ID, err)
				failed++
				continue
			}
			if vErr := ocrimport.ValidateDraft(&draft); vErr != nil {
				_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("draft invalid after retry: %v", vErr))
				fmt.Fprintf(os.Stderr, "draft %s invalid after retry: %v\n", page.ID, vErr)
				failed++
				continue
			}
		}

		draftData, err := ocrimport.MarshalDraftJSON(&draft)
		if err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("marshal draft: %v", err))
			failed++
			continue
		}
		// #nosec G703 -- pageDir is re-derived from the importer-generated queue id and sanitized with filepath.Base above.
		if err := os.WriteFile(filepath.Join(pageDir, "draft.json"), draftData, 0o600); err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("write draft.json: %v", err))
			failed++
			continue
		}
		logLines = append(logLines, "--- validation passed ---", string(draftData))
		// #nosec G703 -- pageDir is re-derived from the importer-generated queue id and sanitized with filepath.Base above.
		if err := os.WriteFile(filepath.Join(pageDir, "draft.log"), []byte(joinLines(logLines)), 0o600); err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("write draft.log: %v", err))
			failed++
			continue
		}

		if err := queue.SetStatus(page.ID, ocrimport.StatusDraft, ""); err != nil {
			fmt.Fprintf(os.Stderr, "update status for %s: %v\n", page.ID, err)
			failed++
			continue
		}

		fmt.Printf("draft: %s -> %s\n", page.SourcePath, draft.Name)
		processed++
	}

	fmt.Printf("draft: processed %d, failed %d\n", processed, failed)
}

func joinLines(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}

func reviewCmd() {
	var workDir string
	var approve bool
	fset := flagSet("review")
	fset.StringVar(&workDir, "work-dir", "", "Work directory; defaults to LENA_IMPORT_WORK_DIR")
	fset.BoolVar(&approve, "approve", false, "Mark the recipe approved if all items resolve")
	if err := fset.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "parse review flags: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	if cfg.APIURL == "" {
		fmt.Fprintln(os.Stderr, "API URL is not configured.")
		fmt.Fprintln(os.Stderr, "Set LENA_API_URL to enable catalog mapping.")
		os.Exit(1)
	}

	if workDir == "" {
		workDir = cfg.ImportWorkDir
	}

	queue, err := ocrimport.Open(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open work queue: %v\n", err)
		os.Exit(1)
	}

	client := bffclient.New(cfg.APIURL, cfg.ImportAdminToken)
	ctx := context.Background()

	catalogData, err := client.ListCatalog(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog snapshot: %v\n", err)
		os.Exit(1)
	}
	catalog := ocrimport.NewCatalogSnapshot(catalogData)

	pending := queue.List()
	processed, failed := 0, 0
	for _, page := range pending {
		if page.Status != ocrimport.StatusDraft {
			continue
		}

		pageDir := filepath.Join(workDir, filepath.Base(page.ID))

		// #nosec G304 -- pageDir is re-derived from the importer-generated queue id and sanitized with filepath.Base.
		draftData, err := os.ReadFile(filepath.Join(pageDir, "draft.json"))
		if err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("read draft.json: %v", err))
			fmt.Fprintf(os.Stderr, "read draft.json for %s: %v\n", page.ID, err)
			failed++
			continue
		}

		var draft ocrimport.RecipeDraft
		if err := json.Unmarshal(draftData, &draft); err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("parse draft.json: %v", err))
			fmt.Fprintf(os.Stderr, "parse draft.json for %s: %v\n", page.ID, err)
			failed++
			continue
		}

		// #nosec G304 -- pageDir is re-derived from the importer-generated queue id and sanitized with filepath.Base.
		ocrText, err := os.ReadFile(filepath.Join(pageDir, "ocr.txt"))
		if err != nil {
			ocrText = []byte("(ocr text not available)")
		}

		existing, _ := ocrimport.LoadReview(pageDir)

		mapped := &ocrimport.ReviewRecipe{
			PageID:      page.ID,
			Name:        draft.Name,
			Description: draft.Description,
			Servings:    draft.Servings,
			PrepTime:    draft.PrepTimeMinutes,
			CookTime:    draft.CookTimeMinutes,
			SourceHint:  draft.SourceHint,
			Steps:       draft.Steps,
			Items:       make([]ocrimport.MatchResult, 0, len(draft.Items)),
		}
		for _, dItem := range draft.Items {
			mapped.Items = append(mapped.Items, catalog.MapDraftItem(dItem, cfg.ImportAutoAcceptConfidence, cfg.ImportReviewThreshold))
		}

		mapped = ocrimport.MergeExistingReview(existing, mapped)

		if approve && mapped.AllResolved() {
			mapped.Approved = true
		}

		if err := ocrimport.SaveReview(pageDir, mapped); err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("write review.json: %v", err))
			fmt.Fprintf(os.Stderr, "write review.json for %s: %v\n", page.ID, err)
			failed++
			continue
		}

		report := ocrimport.RenderReviewMarkdown(mapped, string(ocrText))
		// #nosec G703 -- pageDir is re-derived from the importer-generated queue id and sanitized with filepath.Base above.
		if err := os.WriteFile(filepath.Join(pageDir, "review.md"), []byte(report), 0o600); err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("write review.md: %v", err))
			fmt.Fprintf(os.Stderr, "write review.md for %s: %v\n", page.ID, err)
			failed++
			continue
		}

		newStatus := ocrimport.StatusReview
		if mapped.Approved && mapped.AllResolved() {
			newStatus = ocrimport.StatusReady
		}
		if err := queue.SetStatus(page.ID, newStatus, ""); err != nil {
			fmt.Fprintf(os.Stderr, "update status for %s: %v\n", page.ID, err)
			failed++
			continue
		}

		fmt.Printf("review: %s -> %s (%d items, %s)\n", page.SourcePath, draft.Name, len(mapped.Items), newStatus)
		processed++
	}

	fmt.Printf("review: processed %d, failed %d\n", processed, failed)
}

func persistCmd() {
	var workDir string
	var force bool
	fset := flagSet("persist")
	fset.StringVar(&workDir, "work-dir", "", "Work directory; defaults to LENA_IMPORT_WORK_DIR")
	fset.BoolVar(&force, "force", false, "Re-persist recipes even when a persisted.json record exists")
	if err := fset.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "parse persist flags: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	if cfg.APIURL == "" {
		fmt.Fprintln(os.Stderr, "API URL is not configured.")
		fmt.Fprintln(os.Stderr, "Set LENA_API_URL to enable persistence.")
		os.Exit(1)
	}
	if cfg.ImportAdminToken == "" {
		fmt.Fprintln(os.Stderr, "Admin token is not configured.")
		fmt.Fprintln(os.Stderr, "Set LENA_IMPORT_ADMIN_TOKEN to enable persistence.")
		os.Exit(1)
	}

	if workDir == "" {
		workDir = cfg.ImportWorkDir
	}

	queue, err := ocrimport.Open(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open work queue: %v\n", err)
		os.Exit(1)
	}

	client := bffclient.New(cfg.APIURL, cfg.ImportAdminToken)
	ctx := context.Background()

	pending := queue.List()
	processed, failed, skipped := 0, 0, 0
	for _, page := range pending {
		if page.Status != ocrimport.StatusReady {
			continue
		}

		pageDir := filepath.Join(workDir, filepath.Base(page.ID))

		// #nosec G304 -- pageDir is re-derived from the importer-generated queue id and sanitized with filepath.Base.
		reviewData, err := os.ReadFile(filepath.Join(pageDir, "review.json"))
		if err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("read review.json: %v", err))
			fmt.Fprintf(os.Stderr, "read review.json for %s: %v\n", page.ID, err)
			failed++
			continue
		}

		var review ocrimport.ReviewRecipe
		if err := json.Unmarshal(reviewData, &review); err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("parse review.json: %v", err))
			fmt.Fprintf(os.Stderr, "parse review.json for %s: %v\n", page.ID, err)
			failed++
			continue
		}

		if !review.Approved || !review.AllResolved() {
			msg := "review not approved or not fully resolved"
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, msg)
			fmt.Fprintf(os.Stderr, "%s: %s\n", page.SourcePath, msg)
			failed++
			continue
		}

		if !force {
			existing, err := ocrimport.LoadPersisted(pageDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "load persisted for %s: %v\n", page.ID, err)
			}
			if existing != nil && existing.RecipeID != "" {
				fmt.Printf("persist: %s -> already persisted as %s\n", page.SourcePath, existing.RecipeID)
				skipped++
				continue
			}

			if rec, ok, err := client.GetRecipeByName(ctx, review.Name); err == nil && ok {
				fmt.Printf("persist: %s -> recipe with name %q already exists (%s); skipping\n", page.SourcePath, review.Name, rec.ID)
				skipped++
				continue
			} else if err != nil {
				fmt.Fprintf(os.Stderr, "recipe name lookup for %s: %v\n", page.ID, err)
			}
		}

		input, err := review.ToCreateRecipeInput()
		if err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("invalid review: %v", err))
			_ = ocrimport.SaveError(pageDir, ocrimport.ErrorRecord{
				PageID:  page.ID,
				Error:   err.Error(),
				Created: time.Now().UTC(),
			})
			fmt.Fprintf(os.Stderr, "validate review %s: %v\n", page.SourcePath, err)
			failed++
			continue
		}

		recipeID, err := client.CreateRecipe(ctx, *input)
		if err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("createRecipe: %v", err))
			_ = ocrimport.SaveError(pageDir, ocrimport.ErrorRecord{
				PageID:  page.ID,
				Error:   err.Error(),
				Created: time.Now().UTC(),
			})
			fmt.Fprintf(os.Stderr, "createRecipe %s: %v\n", page.SourcePath, err)
			failed++
			continue
		}

		if err := ocrimport.SavePersisted(pageDir, &ocrimport.PersistedRecord{
			RecipeID:   recipeID,
			Name:       review.Name,
			SourcePath: page.SourcePath,
			SourceHash: page.SourceHash,
			CreatedAt:  time.Now().UTC(),
		}); err != nil {
			_ = queue.SetStatus(page.ID, ocrimport.StatusFailed, fmt.Sprintf("write persisted.json: %v", err))
			fmt.Fprintf(os.Stderr, "write persisted.json for %s: %v\n", page.SourcePath, err)
			failed++
			continue
		}

		if err := queue.SetStatus(page.ID, ocrimport.StatusPersisted, ""); err != nil {
			fmt.Fprintf(os.Stderr, "update status for %s: %v\n", page.ID, err)
			failed++
			continue
		}

		fmt.Printf("persist: %s -> %s (%s)\n", page.SourcePath, review.Name, recipeID)
		processed++
	}

	fmt.Printf("persist: processed %d, skipped %d, failed %d\n", processed, skipped, failed)
}

func flagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}
