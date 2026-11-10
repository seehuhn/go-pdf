// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package text

import (
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// Show draws text inside a single BT/ET text object.  The arguments are
// processed in order and may be of type [M], [F], [Leading], string,
// [pdf.String], [color.Color], [NL], [RecordPos], or the result of [Wrap]
// or [WrapWith].  Show panics on any other argument type, or if a wrapped
// text is shown while the current font does not implement [font.Layouter].
// Show does nothing if b.Err is set.
func Show(b *builder.Builder, args ...any) {
	if b.Err != nil {
		return
	}

	leading := b.State.GState.TextLeading
	leadingSet := b.State.GState.Set&graphics.StateTextLeading != 0

	b.TextBegin()
	for _, arg := range args {
		switch v := arg.(type) {
		case M:
			b.TextFirstLine(v.X, v.Y)
		case F:
			b.TextSetFont(v.Font, v.Size)
			leading = 0
			if l, ok := v.Font.(font.Layouter); ok {
				leading = l.GetGeometry().Leading * v.Size
			}
			if leading <= 0 {
				leading = math.Round(v.Size*15) / 10
			}
			leadingSet = false
			if v.Color != nil {
				b.SetFillColor(v.Color)
			}
		case Leading:
			leading = float64(v)
			leadingSet = true
			b.TextSetLeading(leading)
		case string:
			b.TextShow(v)
		case pdf.String:
			b.TextShowRaw(v)
		case color.Color:
			b.SetFillColor(v)
		case nl:
			if !leadingSet {
				b.TextSecondLine(0, -leading)
				leadingSet = true
			} else {
				b.TextNextLine()
			}
		case *wrap:
			for line := range v.Lines(b.State.GState.TextFont.(font.Layouter), b.State.GState.TextFontSize) {
				b.TextShowGlyphs(line)
				if !leadingSet {
					b.TextSecondLine(0, -leading)
					leadingSet = true
				} else {
					b.TextNextLine()
				}
			}
		case RecordPos:
			x, y := b.State.GState.GetTextPositionUser()
			if v.UserX != nil {
				*v.UserX = x
			}
			if v.UserY != nil {
				*v.UserY = y
			}
		default:
			panic(fmt.Sprintf("unexpected argument type %T", v))
		}
	}
	b.TextEnd()
}

type nl struct{}

// NL starts a new line, using the current leading.
var NL = nl{}

// M moves to the start of a new line, offset by (X, Y) from the start of
// the current line.  Since every call to [Show] starts a new text object,
// the first M in a call, if no NL or wrapped text comes before it, gives
// an absolute position in user space.
type M struct {
	X, Y float64
}

// F sets the font, the font size and, if Color is non-nil, the fill colour.
// It also resets the leading to the font's line spacing, or to 1.5 times
// the font size if the font gives none.
type F struct {
	Font  font.Instance
	Size  float64
	Color color.Color
}

// Leading sets the distance between baselines, until the next [F].
// Without an F or Leading earlier in the call, [Show] uses the leading of
// the graphics state.
type Leading float64

// RecordPos stores the current text position, in user space coordinates,
// into the non-nil fields.
type RecordPos struct {
	UserX, UserY *float64
}
