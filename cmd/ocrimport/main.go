// Command ocrimport imports printed recipes into LENA2 from scanned pages
// using local OCR and a local LLM.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/JRAdams472/LENA2/internal/ocrimport"
	"github.com/JRAdams472/LENA2/internal/platform/config"
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
	fmt.Fprintln(os.Stderr, "  import   Scan --inbox and enqueue source pages for OCR/LLM processing")
	fmt.Fprintln(os.Stderr, "  review   Start the human-review step (not yet implemented)")
	fmt.Fprintln(os.Stderr, "  persist  Persist approved drafts to LENA2 (not yet implemented)")
	fmt.Fprintln(os.Stderr, "  version  Print version")
	fmt.Fprintln(os.Stderr, "  help     Show this help")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "All commands load configuration from LENA_* environment variables.")
	fmt.Fprintln(os.Stderr, "The import feature is disabled when LENA_OLLAMA_URL is empty.")
}

func importCmd() {
	var inbox string
	var workDir string
	fset := flagSet("import")
	fset.StringVar(&inbox, "inbox", "./import/inbox", "Directory containing scanned recipe pages (png, jpg, pdf)")
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

	if cfg.OCRServiceURL == "" || cfg.OllamaURL == "" {
		fmt.Fprintln(os.Stderr, "Recipe OCR import is not configured.")
		fmt.Fprintln(os.Stderr, "Set LENA_OCR_SERVICE_URL and LENA_OLLAMA_URL to enable it.")
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
	fmt.Println("Next: run 'ocrimport review' when OCR and drafting are complete.")
}

func reviewCmd() {
	fmt.Fprintln(os.Stderr, "review: not yet implemented (planned for p3)")
}

func persistCmd() {
	fmt.Fprintln(os.Stderr, "persist: not yet implemented (planned for p4)")
}

func flagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}
