package converter

import (
	"os"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestRenderPageToImage(t *testing.T) {
	// 1. Open sample PDF
	f, err := os.Open("../testdata/fixtures/page0002.pdf")
	if err != nil {
		t.Skip("sample PDF not found, skipping test")
		return
	}
	defer f.Close()

	r, err := pdf.NewReader(f, nil)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	c := NewConverter(r)

	// 2. Render page 1
	img, err := c.RenderPageToImage(1, 72.0)
	if err != nil {
		t.Fatalf("failed to render page: %v", err)
	}

	if img == nil {
		t.Fatal("rendered image is nil")
	}

	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		t.Fatalf("invalid image dimensions: %dx%d", bounds.Dx(), bounds.Dy())
	}

	t.Logf("Successfully rendered image: %dx%d", bounds.Dx(), bounds.Dy())
}
