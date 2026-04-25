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

	// lastWasWhitespace tracks whether the most recently emitted character
	// was whitespace, so that an adjacent space can be collapsed.
	// lastWasNewline narrows that to specifically a newline, so that
	// adjacent newlines collapse but a "space then newline" run is kept
	// intact.  Both start true to suppress leading whitespace.
	lastWasWhitespace bool
	lastWasNewline    bool
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
		lastWasWhitespace:    true,
		lastWasNewline:       true,
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

	e.reader.TextEvent = func(event reader.TextEvent, _ float64) {
		switch event {
		case reader.TextEventSpace:
			e.writeSpace()
		case reader.TextEventNL:
			e.writeNewline()
		}
	}

	e.reader.Character = func(c font.Code) error {
		// suppress character output when inside ActualText region
		if e.actualTextStartDepth != -1 && len(e.reader.MarkedContentStack) >= e.actualTextStartDepth {
			return nil
		}

		text := c.Text
		if text == "" {
			currentFont := e.reader.State.GState.TextFont
			cidMapping, ok := e.extraTextCache[currentFont]
			if !ok {
				cidMapping = textextract.GlyphNameMapping(currentFont)
				e.extraTextCache[currentFont] = cidMapping
			}
			text = cidMapping[c.CID]
		}

		text = remapPUA(text)

		xDev, _ := e.reader.GetTextPositionDevice()
		if xDev < e.XRangeMin || xDev >= e.XRangeMax {
			return nil
		}

		e.writeText(text)
		return nil
	}
}

// writeSpace emits a space, collapsing it against any preceding whitespace.
func (e *TextExtractor) writeSpace() {
	if e.lastWasWhitespace {
		return
	}
	fmt.Fprint(e.writer, " ")
	e.lastWasWhitespace = true
	e.lastWasNewline = false
}

// writeNewline emits a newline.  Adjacent newlines collapse, but a newline
// after a trailing space is kept (the trailing space is preserved).
func (e *TextExtractor) writeNewline() {
	if e.lastWasNewline {
		return
	}
	fmt.Fprintln(e.writer)
	e.lastWasWhitespace = true
	e.lastWasNewline = true
}

// writeText emits character text from the content stream.  An empty text
// is ignored.  A single-space text collapses against preceding whitespace.
func (e *TextExtractor) writeText(text string) {
	if text == "" {
		return
	}
	if len(text) == 1 && text[0] == ' ' {
		e.writeSpace()
		return
	}
	fmt.Fprint(e.writer, text)
	last := text[len(text)-1]
	e.lastWasWhitespace = last == ' ' || last == '\n' || last == '\t'
	e.lastWasNewline = last == '\n'
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

	e.writeText(text)
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
	// reset per-page state.  extraTextCache is keyed by font.Instance and
	// CID, so it persists across pages without risk of stale data.
	e.actualTextStartDepth = -1
	e.lastWasWhitespace = true
	e.lastWasNewline = true
	return e.reader.ParsePage(pageDict, matrix.Identity)
}
