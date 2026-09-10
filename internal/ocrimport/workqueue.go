// Package ocrimport supports the recipe OCR import pipeline.
package ocrimport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Status describes where a source page is in the import pipeline.
type Status string

// Page status values used by the work queue.
const (
	StatusPending   Status = "pending"
	StatusOCR       Status = "ocred"
	StatusDraft     Status = "drafted"
	StatusReview    Status = "reviewing"
	StatusReady     Status = "ready"
	StatusPersisted Status = "persisted"
	StatusFailed    Status = "failed"
	StatusRejected  Status = "rejected"
)

// Page is one source recipe page and its progress through the pipeline.
type Page struct {
	ID           string    `json:"id"`
	SourcePath   string    `json:"source_path"`
	SourceHash   string    `json:"source_hash"`
	Status       Status    `json:"status"`
	WorkDir      string    `json:"work_dir"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// Queue persists source-page records in the importer work directory.
type Queue struct {
	workDir string
	path    string
	mu      sync.Mutex
	pages   []*Page
}

// Open loads or creates a queue at the given work directory.
func Open(workDir string) (*Queue, error) {
	if err := os.MkdirAll(workDir, 0o750); err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	q := &Queue{
		workDir: workDir,
		path:    filepath.Join(workDir, "queue.json"),
	}
	data, err := os.ReadFile(q.path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read queue: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &q.pages); err != nil {
			return nil, fmt.Errorf("parse queue: %w", err)
		}
	}
	return q, nil
}

// Add records a new source page in the queue and creates its per-page work
// directory. id is returned so callers can address the page later.
func (q *Queue) Add(sourcePath string) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	now := time.Now().UTC()
	page := &Page{
		ID:         id,
		SourcePath: sourcePath,
		Status:     StatusPending,
		WorkDir:    filepath.Join(q.workDir, id),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := os.MkdirAll(page.WorkDir, 0o750); err != nil {
		return "", fmt.Errorf("create page dir: %w", err)
	}
	q.pages = append(q.pages, page)
	return id, q.saveLocked()
}

// Get returns a page by id, or nil if not found.
func (q *Queue) Get(id string) *Page {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, p := range q.pages {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// List returns pages sorted by creation time (newest first).
func (q *Queue) List() []*Page {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]*Page, len(q.pages))
	copy(out, q.pages)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// SetStatus updates a page's status and writes the queue back to disk.
func (q *Queue) SetStatus(id string, status Status, errorMessage string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, p := range q.pages {
		if p.ID == id {
			p.Status = status
			p.ErrorMessage = errorMessage
			p.UpdatedAt = time.Now().UTC()
			return q.saveLocked()
		}
	}
	return fmt.Errorf("page %s not found", id)
}

func (q *Queue) saveLocked() error {
	data, err := json.MarshalIndent(q.pages, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal queue: %w", err)
	}
	if err := os.WriteFile(q.path, data, 0o600); err != nil {
		return fmt.Errorf("write queue: %w", err)
	}
	return nil
}
