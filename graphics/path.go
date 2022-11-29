package graphics

import "fmt"

// MoveTo starts a new path at the given coordinates.
func (p *Page) MoveTo(x, y float64) {
	if !p.valid("MoveTo", stateGlobal, statePath) {
		return
	}
	p.state = statePath
	_, p.err = fmt.Fprintln(p.w, coord(x), coord(y), "m")
}

// Rectangle appends a rectangle to the current path as a closed subpath.
func (p *Page) Rectangle(x, y, width, height float64) {
	if !p.valid("Rectangle", stateGlobal, statePath) {
		return
	}
	p.state = statePath
	_, p.err = fmt.Fprintln(p.w, coord(x), coord(y), coord(width), coord(height), "re")
}

// Arc appends an arc of a circle to the current path.
func (p *Page) Arc(x, y, radius, startAngle, endAngle float64) {
	panic("not implemented")
}

// LineTo appends a straight line segment to the current path.
func (p *Page) LineTo(x, y float64) {
	if !p.valid("LineTo", statePath, stateClipped) {
		return
	}
	_, p.err = fmt.Fprintln(p.w, coord(x), coord(y), "l")
}

// Stroke strokes the current path.
func (p *Page) Stroke() {
	if !p.valid("Stroke", statePath, stateClipped) {
		return
	}
	p.state = stateGlobal
	_, p.err = fmt.Fprintln(p.w, "S")
}

// CloseAndStroke closes and strokes the current path.
func (p *Page) CloseAndStroke() {
	if !p.valid("CloseAndStroke", statePath, stateClipped) {
		return
	}
	p.state = stateGlobal
	_, p.err = fmt.Fprintln(p.w, "s")
}

// Fill fills the current path.  Any subpaths that are open are implicitly
// closed before being filled.
func (p *Page) Fill() {
	if !p.valid("Fill", statePath, stateClipped) {
		return
	}
	p.state = stateGlobal
	_, p.err = fmt.Fprintln(p.w, "f")
}
