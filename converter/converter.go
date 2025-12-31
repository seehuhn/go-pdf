package converter

import (
	"fmt"
	"math"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/reader"
	"seehuhn.de/go/postscript/cid"
)

// Converter orchestrates the conversion from PDF to high-level layout structures.
type Converter struct {
	Reader          *reader.Reader
	Pages           []*Page
	Tracker         *FontTracker
	currentPage     *Page
	currentFragment *Fragment
}

// NewConverter creates a new Converter.
func NewConverter(r pdf.Getter) *Converter {
	c := &Converter{
		Reader:  reader.New(r, nil),
		Tracker: &FontTracker{},
	}
	c.setupCallbacks()
	return c
}

func (c *Converter) setupCallbacks() {
	c.Reader.Character = func(cid cid.CID, text string, width float64) error {
		if c.currentPage == nil {
			return nil
		}

		x, y := c.Reader.GetTextPositionDevice()
		fs := c.Reader.TextFontSize
		hs := c.Reader.TextHorizontalScaling
		rise := c.Reader.TextRise

		// Calculate effective font size in pixels
		m0 := matrix.Matrix{fs * hs, 0, 0, fs, 0, rise}
		mFull := m0.Mul(c.Reader.TextMatrix).Mul(c.Reader.CTM)
		effFontSize := math.Sqrt(mFull[2]*mFull[2] + mFull[3]*mFull[3])

		// Map text space displacement to device width.
		// width from Reader already includes TextFontSize.
		m := c.Reader.TextMatrix.Mul(c.Reader.CTM)
		wx, wy := m[0]*width, m[1]*width
		charWidth := math.Sqrt(wx*wx + wy*wy)

		fontID := c.Tracker.AddFont(c.Reader.TextFont, effFontSize)

		// Map Y from PDF space (0 at bottom) to CSS space (0 at top)
		yTop := c.currentPage.Height - (y + effFontSize)

		// Determine if this char belongs to currentFragment
		if c.currentFragment != nil {
			// Looser merging for fragments within a line
			if math.Abs(c.currentFragment.YMin-yTop) < effFontSize*0.5 &&
				math.Abs(x-c.currentFragment.XMax) < effFontSize*0.5 {

				c.currentFragment.AddRun(text, fontID)
				c.currentFragment.XMax = x + charWidth
				return nil
			}
		}

		// Start new fragment
		c.currentFragment = &Fragment{
			XMin: x,
			XMax: x + charWidth,
			YMin: yTop,
			YMax: yTop + effFontSize,
			Dir:  TextDirLeftToRight,
		}
		c.currentFragment.AddRun(text, fontID)
		c.currentPage.AddFragment(c.currentFragment)
		return nil
	}
}

// ConvertPage processes a single page and returns the Page object.
func (c *Converter) ConvertPage(pageNum int, pageObj pdf.Object) (*Page, error) {
	// 1. Get page dimensions
	pageDict, err := pdf.GetDictTyped(c.Reader.R, pageObj, "Page")
	if err != nil {
		return nil, err
	}

	mediaBox, err := pdf.GetArray(c.Reader.R, pageDict["MediaBox"])
	if err != nil {
		return nil, err
	}
	if len(mediaBox) < 4 {
		return nil, fmt.Errorf("missing or invalid MediaBox for page %d", pageNum)
	}

	m2, err := pdf.GetNumber(c.Reader.R, mediaBox[0])
	if err != nil {
		return nil, err
	}
	m1, err := pdf.GetNumber(c.Reader.R, mediaBox[1])
	if err != nil {
		return nil, err
	}
	m3, err := pdf.GetNumber(c.Reader.R, mediaBox[2])
	if err != nil {
		return nil, err
	}
	m4, err := pdf.GetNumber(c.Reader.R, mediaBox[3])
	if err != nil {
		return nil, err
	}

	width := float64(m3 - m2)
	height := float64(m4 - m1)

	p := NewPage(pageNum, width, height, false)
	c.Pages = append(c.Pages, p)

	// Set state for callbacks
	c.currentPage = p
	c.currentFragment = nil

	// Reset reader for the new page
	c.Reader.Reset()

	err = c.Reader.ParsePage(pageObj, matrix.Identity)
	if err != nil {
		return nil, err
	}

	p.Coalesce()

	return p, nil
}

// ConvertDocument processes all pages in the PDF.
func (c *Converter) ConvertDocument() ([]*Page, error) {
	numPages, err := pagetree.NumPages(c.Reader.R)
	if err != nil {
		return nil, err
	}

	for i := 0; i < numPages; i++ {
		_, pageDict, err := pagetree.GetPage(c.Reader.R, i)
		if err != nil {
			return nil, err
		}
		_, err = c.ConvertPage(i+1, pageDict)
		if err != nil {
			return nil, err
		}
	}

	return c.Pages, nil
}
