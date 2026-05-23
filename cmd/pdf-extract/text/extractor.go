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

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/textextract"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/reader"
)

// TextExtractor extracts text from PDF pages.
type TextExtractor struct {
	reader *reader.Reader
	x      *pdf.Extractor
	writer io.Writer

	XRangeMin float64
	XRangeMax float64

	extraTextCache map[font.Instance]map[cid.CID]string

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
	x := pdf.NewExtractor(doc)
	e := &TextExtractor{
		reader:            reader.New(x),
		x:                 x,
		writer:            w,
		XRangeMin:         math.Inf(-1),
		XRangeMax:         math.Inf(1),
		extraTextCache:    make(map[font.Instance]map[cid.CID]string),
		lastWasWhitespace: true,
		lastWasNewline:    true,
	}

	e.setupCallbacks()
	return e
}

func (e *TextExtractor) setupCallbacks() {
	e.reader.ActualText = func(event reader.ActualTextEvent, text string) error {
		if event == reader.ActualTextBegin {
			e.writeText(text)
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
		// inside an ActualText region the replacement text has already
		// been emitted; suppress per-glyph text
		if e.reader.InActualText() {
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

		xDev, _ := e.reader.State.GState.GetTextPositionDevice()
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
	e.lastWasWhitespace = true
	e.lastWasNewline = true

	pg, err := pdf.ExtractorGet(e.x, nil, pageDict, page.Decode)
	if err != nil {
		return err
	}
	return e.reader.ProcessPage(pg)
}
