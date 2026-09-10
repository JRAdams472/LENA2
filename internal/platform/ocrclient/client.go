// Package ocrclient is a thin HTTP client for the OCR microservice.
package ocrclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Line is one line of OCR text with its confidence and bounding box.
type Line struct {
	Text string  `json:"text"`
	Conf float64 `json:"conf"`
	BBox *BBox   `json:"bbox,omitempty"`
}

// BBox is the bounding box of a line or word.
type BBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// PageResult is the OCR output for a single page (image or PDF page).
type PageResult struct {
	Page           int     `json:"page"`
	Text           string  `json:"text"`
	Lines          []Line  `json:"lines"`
	MeanConfidence float64 `json:"mean_confidence"`
}

// Result is the full response from the OCR service.
type Result struct {
	Text  string       `json:"text"`
	Lines []Line       `json:"lines"`
	Pages []PageResult `json:"pages"`
}

// Client calls the OCR service.
type Client struct {
	baseURL string
	client  *http.Client
	timeout time.Duration
}

// New creates a Client. baseURL should be the root URL of the OCR service
// (e.g. "http://ocr:8000").
func New(baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = "http://ocr:8000"
	}
	if timeout == 0 {
		timeout = 20 * time.Second
	}
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
		timeout: timeout,
	}
}

// ExtractText POSTs the image to /ocr and returns the extracted text.
// It is the simple interface used by the existing nutrition-label OCR flow.
func (c *Client) ExtractText(ctx context.Context, image []byte) (string, error) {
	res, err := c.ExtractTextResult(ctx, image, "image.jpg")
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

// ExtractTextResult POSTs the image to /ocr and returns the full structured
// OCR result. It is used by the recipe import pipeline. filename is sent as
// the multipart filename so the service can detect PDFs by extension.
func (c *Client) ExtractTextResult(ctx context.Context, image []byte, filename string) (*Result, error) {
	if filename == "" {
		filename = "image.jpg"
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("image", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(image); err != nil {
		return nil, fmt.Errorf("write image: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ocr", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ocr request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ocr returned status %d: %s", resp.StatusCode, string(body))
	}

	var result Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode ocr response: %w", err)
	}
	return &result, nil
}
