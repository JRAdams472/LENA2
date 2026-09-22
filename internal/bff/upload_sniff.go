package bff

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG for image.DecodeConfig
	_ "image/png"  // register PNG for image.DecodeConfig
	"net/http"
	"regexp"
	"strings"
)

const (
	// maxImagePixels bounds decoded image dimensions; larger images are
	// rejected before they reach the OCR sidecar (decompression bombs).
	maxImagePixels = 40_000_000
	// maxPDFPages bounds how many pages a scanned recipe PDF may contain.
	maxPDFPages = 20
)

// pdfPagePattern matches the /Type /Page marker of each page object.
var pdfPagePattern = regexp.MustCompile(`/Type\s*/Page[^s]`)

// sniffUpload validates decoded upload bytes against the declared media type
// and bounds their complexity. It returns the detected media type
// ("image/png", "image/jpeg", or "application/pdf").
//
// The declared type is a hint only: the bytes themselves must sniff to an
// allowed type, and a non-empty declaration that disagrees with the sniffed
// type is rejected.
func sniffUpload(decoded []byte, declaredMediaType string) (string, error) {
	if len(decoded) == 0 {
		return "", fmt.Errorf("empty upload")
	}

	detected := http.DetectContentType(decoded)
	declared := strings.ToLower(strings.TrimSpace(declaredMediaType))
	if declared == "image/jpg" {
		declared = "image/jpeg"
	}

	switch detected {
	case "image/png", "image/jpeg":
		if declared != "" && declared != detected && !strings.HasPrefix(declared, "image/") {
			return "", fmt.Errorf("declared media type %q does not match detected %q", declaredMediaType, detected)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(decoded))
		if err != nil {
			return "", fmt.Errorf("cannot decode image: %w", err)
		}
		if int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
			return "", fmt.Errorf("image %dx%d exceeds %d pixel limit", cfg.Width, cfg.Height, maxImagePixels)
		}
		return detected, nil

	case "application/pdf":
		if declared != "" && declared != "application/pdf" && declared != "pdf" {
			return "", fmt.Errorf("declared media type %q does not match detected %q", declaredMediaType, detected)
		}
		if !bytes.HasPrefix(decoded, []byte("%PDF-")) {
			return "", fmt.Errorf("missing PDF header")
		}
		if n := len(pdfPagePattern.FindAll(decoded, -1)); n > maxPDFPages {
			return "", fmt.Errorf("pdf has %d pages; maximum is %d", n, maxPDFPages)
		}
		return detected, nil

	default:
		return "", fmt.Errorf("unsupported content type %q", detected)
	}
}
