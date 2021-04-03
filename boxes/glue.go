package boxes

import "seehuhn.de/go/pdf/pages"

type stretcher interface {
	Stretch() *stretchAmount
}

type stretchAmount struct {
	Val   float64
	Level int
}

type glue struct {
	Length float64
	Plus   stretchAmount
	Minus  stretchAmount
}

// NewGlue returns a new "glue" box with the given natural length and
// stretchability.
func NewGlue(length float64, plus float64, plusLevel int, minus float64, minusLevel int) Box {
	return &glue{
		Length: length,
		Plus:   stretchAmount{plus, plusLevel},
		Minus:  stretchAmount{minus, minusLevel},
	}
}

func (obj *glue) Extent() *BoxExtent {
	return &BoxExtent{
		Width:          obj.Length,
		Height:         obj.Length,
		WhiteSpaceOnly: true,
	}
}

func (obj *glue) Draw(page *pages.Page, xPos, yPos float64) {}

func (obj *glue) Stretch() *stretchAmount {
	return &obj.Plus
}
