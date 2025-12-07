package lab

import (
	"io"

	"seehuhn.de/go/pdf"
	pdfcolor "seehuhn.de/go/pdf/graphics/color"
	pdfimage "seehuhn.de/go/pdf/graphics/image"
)

type Lab8 struct {
	Width  int
	Height int

	// PixData holds the image pixel data in Lab color space.
	// Each pixel is represented by 3 consecutive uint8 values: L, a, and b.
	PixData []uint8
}

func NewLab8(width, height int) *Lab8 {
	return &Lab8{
		Width:   width,
		Height:  height,
		PixData: make([]uint8, width*height*3),
	}
}

// Subtype returns the PDF XObject subtype for images.
func (im *Lab8) Subtype() pdf.Name {
	return pdf.Name("Image")
}

// Embed converts the Go representation of the object into a PDF object,
// corresponding to the PDF version of the output file.
//
// The return value is the PDF representation of the object.
// If the object is embedded in the PDF file, this may be a [Reference].
func (im *Lab8) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	cs, err := pdfcolor.Lab(pdfcolor.WhitePointD65, nil, nil)
	if err != nil {
		return nil, err
	}
	dict := &pdfimage.Dict{
		Width:            im.Width,
		Height:           im.Height,
		ColorSpace:       cs,
		BitsPerComponent: 8,
		Decode:           []float64{0, 100, -100, 100, -100, 100},
		WriteData: func(w io.Writer) error {
			_, err := w.Write(im.PixData)
			return err
		},
		Interpolate: true,
	}
	return dict.Embed(e)
}
