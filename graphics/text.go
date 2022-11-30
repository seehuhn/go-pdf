package graphics

import (
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// BeginText starts a new text object.
func (p *Page) BeginText() {
	if !p.valid("BeginText", stateGlobal) {
		return
	}
	p.state = stateText
	_, p.err = fmt.Fprintln(p.w, "BT")
}

// EndText ends the current text object.
func (p *Page) EndText() {
	if !p.valid("EndText", stateText) {
		return
	}
	p.state = stateGlobal
	p.font = nil
	_, p.err = fmt.Fprintln(p.w, "ET")
}

// StartLine moves to the start of the next line of text.
func (p *Page) StartLine(x, y float64) {
	if !p.valid("StartLine", stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.w, p.coord(x), p.coord(y), "Td")
}

// StartNextLine moves to the start of the next line of text.
func (p *Page) StartNextLine(x, y float64) {
	if !p.valid("StartNextLine", stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.w, p.coord(x), p.coord(y), "TD")
}

// NewLine moves to the start of the next line of text.
func (p *Page) NewLine() {
	if !p.valid("NewLine", stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.w, "T*")
}

// SetFont sets the font and font size.
func (p *Page) SetFont(font *font.Font, size float64) {
	// TODO(voss): should this enter the font in the Resources dictionary?
	if !p.valid("SetFont", stateText) {
		return
	}
	p.font = font
	p.fontSize = size
	err := font.InstName.PDF(p.w)
	if err != nil {
		p.err = err
		return
	}
	_, p.err = fmt.Fprintln(p.w, "", size, "Tf")
}

// ShowString draws a string.
func (p *Page) ShowString(s string) {
	p.ShowStringAligned(s, 0, 0)
}

// ShowStringAligned draws a string and align it.
// The beginning of the string is shifted right by a*w+b, where w
// is the width of the string.
func (p *Page) ShowStringAligned(s string, a, b float64) {
	if !p.valid("ShowStrings", stateText) {
		return
	}
	if p.font == nil {
		p.err = errors.New("no font set")
		return
	}

	font := p.font
	gg := font.Typeset(s, p.fontSize)
	if len(gg) == 0 {
		return
	}

	q := 1000 / float64(font.UnitsPerEm)

	leftOffset := 1000 * b / p.fontSize
	if a != 0 {
		totalWidth := 0.0
		for _, g := range gg {
			totalWidth += float64(g.Advance)
		}
		leftOffset += a * totalWidth * q
	}

	var out pdf.Array
	var run pdf.String

	flush := func() {
		if len(run) > 0 {
			out = append(out, run)
			run = nil
		}
		if len(out) == 0 {
			return
		}
		if len(out) == 1 {
			if s, ok := out[0].(pdf.String); ok {
				if p.err != nil {
					return
				}
				p.err = s.PDF(p.w)
				if p.err != nil {
					return
				}
				_, p.err = fmt.Fprintln(p.w, " Tj")
				out = nil
				return
			}
		}

		if p.err != nil {
			return
		}
		p.err = out.PDF(p.w)
		if p.err != nil {
			return
		}
		_, p.err = fmt.Fprintln(p.w, " TJ")
		if p.err != nil {
			return
		}
		out = nil
	}

	xOffset := leftOffset
	for _, glyph := range gg {
		xOffset += float64(glyph.XOffset) * q

		xOffsetInt := pdf.Integer(math.Round(xOffset))
		if xOffsetInt != 0 {
			if len(run) > 0 {
				out = append(out, run)
				run = nil
			}
			out = append(out, -xOffsetInt)
			xOffset -= float64(xOffsetInt)
		}

		if newYPos := pdf.Integer(math.Round(float64(glyph.YOffset) * q)); newYPos != p.textRise {
			flush()
			p.textRise = newYPos
			if p.err != nil {
				return
			}
			p.err = p.textRise.PDF(p.w)
			if p.err != nil {
				return
			}
			_, p.err = fmt.Fprintln(p.w, " Ts")
		}

		gid := glyph.Gid
		if int(gid) >= len(font.Widths) {
			gid = 0
		}
		run = append(run, font.Enc(gid)...)

		xOffset += (float64(glyph.Advance) - float64(glyph.XOffset) - float64(font.Widths[gid])) * q
	}
	flush()
}
