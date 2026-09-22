package bff

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func testPDF(pages int) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	for i := 0; i < pages; i++ {
		b.WriteString("<< /Type /Page /Parent 1 0 R >>\n")
	}
	b.WriteString("<< /Type /Pages /Count 1 >>\n%%EOF")
	return b.Bytes()
}

func TestSniffUpload(t *testing.T) {
	png := testPNG(t, 8, 8)

	t.Run("valid png declared png", func(t *testing.T) {
		mt, err := sniffUpload(png, "image/png")
		require.NoError(t, err)
		assert.Equal(t, "image/png", mt)
	})

	t.Run("png body declared as pdf is rejected", func(t *testing.T) {
		_, err := sniffUpload(png, "application/pdf")
		assert.Error(t, err)
	})

	t.Run("pdf body declared as png is rejected", func(t *testing.T) {
		_, err := sniffUpload(testPDF(1), "image/png")
		assert.Error(t, err)
	})

	t.Run("valid pdf", func(t *testing.T) {
		mt, err := sniffUpload(testPDF(3), "application/pdf")
		require.NoError(t, err)
		assert.Equal(t, "application/pdf", mt)
	})

	t.Run("pdf over page cap is rejected", func(t *testing.T) {
		_, err := sniffUpload(testPDF(maxPDFPages+1), "application/pdf")
		assert.ErrorContains(t, err, "pages")
	})

	t.Run("oversized image is rejected", func(t *testing.T) {
		// 8000x6000 = 48 MP > 40 MP limit.
		big := testPNG(t, 8000, 6000)
		_, err := sniffUpload(big, "image/png")
		assert.ErrorContains(t, err, "pixel limit")
	})

	t.Run("undetectable content is rejected", func(t *testing.T) {
		_, err := sniffUpload([]byte("definitely not an image or pdf"), "image/png")
		assert.Error(t, err)
	})

	t.Run("empty body is rejected", func(t *testing.T) {
		_, err := sniffUpload(nil, "")
		assert.Error(t, err)
	})
}
