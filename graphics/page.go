package graphics

import (
	"fmt"
	"io"
	"math"
)

// Page is a PDF page.
type Page struct {
	w     io.Writer
	state state
	err   error
}

// NewPage creates a new page.
func NewPage(w io.Writer) *Page {
	return &Page{
		w:     w,
		state: stateGlobal,
	}
}

// Close must be called after drawing the page is complete.
// Any error that occurred during drawing is returned here.
func (p *Page) Close() error {
	return p.err
}

func (p *Page) valid(cmd string, ss ...state) bool {
	if p.err != nil {
		return false
	}

	for _, s := range ss {
		if p.state == s {
			return true
		}
	}

	p.err = fmt.Errorf("unexpected state %q for %q", p.state, cmd)
	return false
}

// Translate moves the origin of the coordinate system.
func (p *Page) Translate(x, y float64) {
	if !p.valid("Translate", stateGlobal) {
		return
	}
	_, p.err = fmt.Fprintln(p.w, 1, 0, 0, 1, coord(x), coord(y), "cm")
}

// SetLineWidth sets the line width.
func (p *Page) SetLineWidth(width float64) {
	if !p.valid("SetLineWidth", stateGlobal) {
		return
	}
	_, p.err = fmt.Fprintln(p.w, coord(width), "w")
}

type coord float64

func (x coord) String() string {
	// TODO(voss): think about this some more
	xInt := int(x)
	if math.Abs(float64(x)-float64(xInt)) < 1e-6 {
		return fmt.Sprintf("%d", xInt)
	}
	return fmt.Sprintf("%g", float64(x))
}

type state int

// See Figure 9 (p. 113) of PDF 32000-1:2008.
const (
	stateNone state = iota
	stateGlobal
	statePath
	stateText
	stateClipped
	stateShading
	stateImage
	stateExternal
)

func (s state) String() string {
	switch s {
	case stateNone:
		return "none"
	case stateGlobal:
		return "global"
	case statePath:
		return "path"
	case stateText:
		return "text"
	case stateClipped:
		return "clipped"
	case stateShading:
		return "shading"
	case stateImage:
		return "image"
	case stateExternal:
		return "external"
	default:
		return fmt.Sprintf("state(%d)", s)
	}
}
