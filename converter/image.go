package converter

import (
	"fmt"
	"image"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"
)

// RenderPageToImage renders a single page of the PDF to an image.Image.
// pageNum is 1-based.
// dpi specifies the resolution in dots per inch (72 is the default PDF resolution).
func (c *Converter) RenderPageToImage(pageNum int, dpi float64) (image.Image, error) {
	// 1. Get the page object
	_, pageDict, err := pagetree.GetPage(c.Reader.R, pageNum-1)
	if err != nil {
		return nil, fmt.Errorf("failed to get page %d: %w", pageNum, err)
	}

	// 2. Get page dimensions (MediaBox)
	mediaBox, err := pdf.GetArray(c.Reader.R, pageDict["MediaBox"])
	if err != nil {
		return nil, err
	}
	if len(mediaBox) < 4 {
		return nil, fmt.Errorf("missing or invalid MediaBox for page %d", pageNum)
	}

	m2, _ := pdf.GetNumber(c.Reader.R, mediaBox[0])
	m1, _ := pdf.GetNumber(c.Reader.R, mediaBox[1])
	m3, _ := pdf.GetNumber(c.Reader.R, mediaBox[2])
	m4, _ := pdf.GetNumber(c.Reader.R, mediaBox[3])

	widthPts := float64(m3 - m2)
	heightPts := float64(m4 - m1)

	// 3. Calculate pixel dimensions
	scale := dpi / 72.0
	widthPx := int(widthPts * scale)
	heightPx := int(heightPts * scale)

	// 4. Create renderer
	render := NewImageRenderer(c, widthPx, heightPx, dpi, float64(m2), float64(m1))
	render.Setup()

	// 5. Parse the page
	c.Reader.Reset()
	err = c.Reader.ParsePage(pageDict, matrix.Identity)
	if err != nil {
		return nil, err
	}

	return render.Image, nil
}
