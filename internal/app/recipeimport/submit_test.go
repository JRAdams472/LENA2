package recipeimport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtensionForMediaType(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"application/pdf", ".pdf"},
		{"pdf", ".pdf"},
		{"image/png", ".png"},
		{"image/jpeg", ".jpg"},
		{"image/jpg", ".jpg"},
		{"image/webp", ".png"}, // unknown image subtypes save as png
		{" IMAGE/PNG ", ".png"},
		{"", ".png"},
		{"text/plain", ""},
		{"application/json", ""},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, extensionForMediaType(tc.in), "media type %q", tc.in)
	}
}

func TestSubmit_WritesGeneratedFileInInbox(t *testing.T) {
	inbox := t.TempDir()
	svc := &Service{store: newMemoryStore(), cfg: Config{InboxDir: inbox}}
	submitter := int64(7)

	data := []byte("fake-png-bytes")
	ri, err := svc.Submit(context.Background(), "image/png", data, &submitter, "test")
	require.NoError(t, err)
	assert.Equal(t, StatusPending, ri.Status)
	require.NotNil(t, ri.SubmittedByUserID)
	assert.Equal(t, int64(7), *ri.SubmittedByUserID)

	// The generated name lands inside the inbox — never client-controlled.
	assert.True(t, strings.HasPrefix(ri.SourceFilename, "scan-"))
	assert.True(t, strings.HasSuffix(ri.SourceFilename, ".png"))
	assert.Equal(t, filepath.Join(inbox, ri.SourceFilename), ri.SourcePath)

	written, err := os.ReadFile(ri.SourcePath)
	require.NoError(t, err)
	assert.Equal(t, data, written)
	assert.Equal(t, hashBytes(data), ri.SourceHash)
}

func TestSubmit_RejectsUnsupportedMediaType(t *testing.T) {
	svc := &Service{store: newMemoryStore(), cfg: Config{InboxDir: t.TempDir()}}
	_, err := svc.Submit(context.Background(), "text/html", []byte("<html>"), nil, "test")
	assert.ErrorContains(t, err, "unsupported media type")
}

func TestSubmit_RequiresInbox(t *testing.T) {
	svc := &Service{store: newMemoryStore()}
	_, err := svc.Submit(context.Background(), "image/png", []byte("x"), nil, "test")
	assert.ErrorContains(t, err, "inbox")
}

// failCreateStore fails only Create, to prove the orphan file is removed.
type failCreateStore struct {
	Store
	err error
}

func (f failCreateStore) Create(context.Context, RecipeImport) (*RecipeImport, error) {
	return nil, f.err
}

func TestSubmit_InsertFailureRemovesOrphanFile(t *testing.T) {
	inbox := t.TempDir()
	svc := &Service{
		store: failCreateStore{Store: newMemoryStore(), err: errors.New("db down")},
		cfg:   Config{InboxDir: inbox},
	}
	_, err := svc.Submit(context.Background(), "image/png", []byte("x"), nil, "test")
	assert.ErrorContains(t, err, "create recipe import")

	entries, rerr := os.ReadDir(inbox)
	require.NoError(t, rerr)
	assert.Empty(t, entries, "failed insert must remove the orphaned inbox file")
}
