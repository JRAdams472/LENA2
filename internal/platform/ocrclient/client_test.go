package ocrclient

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractTextResult_Success(t *testing.T) {
	var gotFilename, gotField string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ocr", r.URL.Path)
		mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		require.NoError(t, err)
		assert.Equal(t, "multipart/form-data", mt)
		mr := multipart.NewReader(r.Body, params["boundary"])
		part, err := mr.NextPart()
		require.NoError(t, err)
		gotField = part.FormName()
		gotFilename = part.FileName()
		body, err := io.ReadAll(part)
		require.NoError(t, err)
		assert.Equal(t, []byte("fake-image-bytes"), body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"hello world","pages":[{"page":1,"text":"hello world","mean_confidence":91.5}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)
	res, err := c.ExtractTextResult(context.Background(), []byte("fake-image-bytes"), "scan.pdf")
	require.NoError(t, err)
	assert.Equal(t, "hello world", res.Text)
	require.Len(t, res.Pages, 1)
	assert.Equal(t, 91.5, res.Pages[0].MeanConfidence)
	assert.Equal(t, "image", gotField)
	assert.Equal(t, "scan.pdf", gotFilename)
}

func TestExtractTextResult_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("engine exploded"))
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)
	_, err := c.ExtractTextResult(context.Background(), []byte("img"), "x.png")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
	assert.Contains(t, err.Error(), "engine exploded")
}

func TestExtractTextResult_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not json`))
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)
	_, err := c.ExtractTextResult(context.Background(), []byte("img"), "x.png")
	assert.ErrorContains(t, err, "decode ocr response")
}

func TestExtractTextResult_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()

	c := New(srv.URL, 50*time.Millisecond)
	_, err := c.ExtractTextResult(context.Background(), []byte("img"), "x.png")
	require.Error(t, err)
}

func TestExtractText_UsesDefaultFilename(t *testing.T) {
	var gotFilename string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		require.NoError(t, err)
		assert.Equal(t, "multipart/form-data", mt)
		mr := multipart.NewReader(r.Body, params["boundary"])
		part, err := mr.NextPart()
		require.NoError(t, err)
		gotFilename = part.FileName()
		_, _ = w.Write([]byte(`{"text":"ok"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)
	text, err := c.ExtractText(context.Background(), []byte("img"))
	require.NoError(t, err)
	assert.Equal(t, "ok", text)
	assert.Equal(t, "image.jpg", gotFilename)
}
