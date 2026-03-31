// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package text

import (
	"fmt"
	"io"
	"math"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/textextract"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/property"
	"seehuhn.de/go/pdf/reader"
)

// TextExtractor extracts text from PDF pages.
type TextExtractor struct {
	reader *reader.Reader
	writer io.Writer

	XRangeMin float64
	XRangeMax float64

	extraTextCache       map[font.Instance]map[cid.CID]string
	actualTextStartDepth int // -1 if not in ActualText region
	prevY                float64
	prevYValid           bool
}

// New creates a new TextExtractor that writes to w.
func New(doc pdf.Getter, w io.Writer) *TextExtractor {
	e := &TextExtractor{
		reader:               reader.New(pdf.NewExtractor(doc)),
		writer:               w,
		XRangeMin:            math.Inf(-1),
		XRangeMax:            math.Inf(1),
		extraTextCache:       make(map[font.Instance]map[cid.CID]string),
		actualTextStartDepth: -1,
	}

	e.setupCallbacks()
	return e
}

func (e *TextExtractor) setupCallbacks() {
	// ActualText handling uses depth-based suppression:
	// - When first ActualText encountered: emit replacement text, save stack depth
	// - While stack depth >= saved depth: suppress all character output
	// - When stack depth < saved depth: exit region, reset to -1
	// This naturally handles nested ActualText (outer wins) and nested content.
	e.reader.MarkedContent = func(event reader.MarkedContentEvent, mc *graphics.MarkedContent) error {
		switch event {
		case reader.MarkedContentBegin:
			e.handleActualTextBegin(mc)
		case reader.MarkedContentEnd:
			e.handleActualTextEnd()
		}

		return nil
	}

	e.reader.TextEvent = func(op reader.TextEvent, arg float64) {
		switch op {
		case reader.TextEventSpace:
			fmt.Fprint(e.writer, " ")
		case reader.TextEventNL:
			fmt.Fprintln(e.writer)
			e.prevYValid = false
		case reader.TextEventMove:
			if e.reader.State.GState.TextFont == nil {
				fmt.Fprintln(e.writer)
				e.prevYValid = false
				break
			}
			_, y := e.reader.GetTextPositionDevice()
			if e.prevYValid && math.Abs(y-e.prevY) < 0.5 {
				fmt.Fprint(e.writer, " ")
			} else {
				fmt.Fprintln(e.writer)
			}
			e.prevY = y
			e.prevYValid = true
		}
	}

	e.reader.Character = func(cid cid.CID, text string) error {
		// suppress character output when inside ActualText region
		if e.actualTextStartDepth != -1 && len(e.reader.MarkedContentStack) >= e.actualTextStartDepth {
			return nil
		}

		if text == "" {
			currentFont := e.reader.State.GState.TextFont
			cidMapping, ok := e.extraTextCache[currentFont]
			if !ok {
				cidMapping = textextract.GlyphNameMapping(currentFont)
				e.extraTextCache[currentFont] = cidMapping
			}
			text = cidMapping[cid]
		}

		text = remapPUA(text)

		xDev, _ := e.reader.GetTextPositionDevice()
		if xDev >= e.XRangeMin && xDev < e.XRangeMax {
			fmt.Fprint(e.writer, text)
		}
		return nil
	}
}

func (e *TextExtractor) handleActualTextBegin(mc *graphics.MarkedContent) {
	// already in ActualText region - inner ActualText is suppressed
	if e.actualTextStartDepth != -1 {
		return
	}

	if mc.Properties == nil {
		return
	}

	at, err := property.ListGet(mc.Properties, property.ExtractActualText)
	if err != nil {
		return
	}
	text := at.Text

	fmt.Fprint(e.writer, text)
	e.actualTextStartDepth = len(e.reader.MarkedContentStack)
}

func (e *TextExtractor) handleActualTextEnd() {
	if len(e.reader.MarkedContentStack) < e.actualTextStartDepth {
		e.actualTextStartDepth = -1
	}
}

// remapPUA replaces Private Use Area codepoints (U+F020–U+F0FF) with their
// Unicode equivalents.  Some PDF generators (notably older Microsoft tools)
// map Symbol font characters to this PUA range instead of real Unicode.
// The low byte of each PUA codepoint corresponds to the Symbol encoding
// position.
func remapPUA(text string) string {
	needsRemap := false
	for _, r := range text {
		if r >= 0xF020 && r <= 0xF0FF {
			needsRemap = true
			break
		}
	}
	if !needsRemap {
		return text
	}

	var buf []rune
	for _, r := range text {
		if r >= 0xF020 && r <= 0xF0FF {
			glyphName := pdfenc.Symbol.Encoding[r-0xF000]
			if glyphName != ".notdef" {
				replacement := names.ToUnicode(glyphName, "")
				if replacement != "" {
					buf = append(buf, []rune(replacement)...)
					continue
				}
			}
		}
		buf = append(buf, r)
	}
	return string(buf)
}

// ExtractPage extracts text from a page dictionary.
func (e *TextExtractor) ExtractPage(pageDict pdf.Dict) error {
	e.prevYValid = false
	return e.reader.ParsePage(pageDict, matrix.Identity)
}
