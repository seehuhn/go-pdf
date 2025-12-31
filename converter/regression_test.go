package converter

import (
	"image"
	"image/png"
	"os"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestRegression_Page0002(t *testing.T) {
	// Paths
	pdfPath := "../testdata/fixtures/page0002.pdf"
	pngPath := "../testdata/fixtures/page0002.png"

	// 1. Render PDF
	f, err := os.Open(pdfPath)
	if err != nil {
		t.Fatalf("failed to open PDF: %v", err)
	}
	defer f.Close()

	r, err := pdf.NewReader(f, nil)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	c := NewConverter(r)
	img, err := c.RenderPageToImage(1, 72.0)
	if err != nil {
		t.Fatalf("failed to render page: %v", err)
	}

	// 2. Load Golden Image
	fPng, err := os.Open(pngPath)
	if err != nil {
		t.Fatalf("failed to open golden PNG: %v", err)
	}
	defer fPng.Close()

	golden, err := png.Decode(fPng)
	if err != nil {
		t.Fatalf("failed to decode golden PNG: %v", err)
	}

	// 3. Compare
	compareImages(t, img, golden)
}

func compareImages(t *testing.T, actual, expected image.Image) {
	b1 := actual.Bounds()
	b2 := expected.Bounds()

	if b1 != b2 {
		t.Fatalf("image bounds differ: actual %v, expected %v", b1, b2)
	}

	for y := b1.Min.Y; y < b1.Max.Y; y++ {
		for x := b1.Min.X; x < b1.Max.X; x++ {
			r1, g1, b1, a1 := actual.At(x, y).RGBA()
			r2, g2, b2, a2 := expected.At(x, y).RGBA()

			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				t.Fatalf("pixel mismatch at (%d, %d): actual (%d,%d,%d,%d), expected (%d,%d,%d,%d)",
					x, y, r1, g1, b1, a1, r2, g2, b2, a2)
			}
		}
	}
}
