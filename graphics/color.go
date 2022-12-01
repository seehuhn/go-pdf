package graphics

import "fmt"

// SetFillGray sets the fill color to the given gray value.
// The value must be in the range from 0 (black) to 1 (white).
func (p *Page) SetFillGray(g float64) {
	if !p.valid("SetFillGray", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintf(p.w, "%g g\n", g)
}

// SetStrokeGray sets the stroke color to the given gray value.
// The value must be in the range from 0 (black) to 1 (white).
func (p *Page) SetStrokeGray(g float64) {
	if !p.valid("SetStrokeGray", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintf(p.w, "%g G\n", g)
}

// SetFillRGB sets the fill color to the given RGB values.
// Each component must be in the range [0, 1].
func (p *Page) SetFillRGB(r, g, b float64) {
	if !p.valid("SetFillRGB", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintf(p.w, "%g %g %g rg\n", r, g, b)
}

// SetStrokeRGB sets the stroke color to the given RGB values.
// Each component must be in the range [0, 1].
func (p *Page) SetStrokeRGB(r, g, b float64) {
	if !p.valid("SetStrokeRGB", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintf(p.w, "%g %g %g RG\n", r, g, b)
}
