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
func (c *Client) ExtractText(ctx context.Context, image []byte) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("image", "image.jpg")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(image); err != nil {
		return "", fmt.Errorf("write image: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ocr", &buf)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ocr request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ocr returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode ocr response: %w", err)
	}
	return result.Text, nil
}
