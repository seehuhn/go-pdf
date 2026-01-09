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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/reader"
)

// TextExtractor extracts text from PDF pages with optional ActualText support.
type TextExtractor struct {
	reader *reader.Reader
	writer io.Writer

	UseActualText bool
	XRangeMin     float64
	XRangeMax     float64

	extraTextCache       map[font.Instance]map[cid.CID]string
	spaceWidth           map[font.Instance]float64
	actualTextStartDepth int // -1 if not in ActualText region
}

// New creates a new TextExtractor that writes to w.
func New(doc pdf.Getter, w io.Writer) *TextExtractor {
	e := &TextExtractor{
		reader:               reader.New(doc),
		writer:               w,
		XRangeMin:            math.Inf(-1),
		XRangeMax:            math.Inf(1),
		extraTextCache:       make(map[font.Instance]map[cid.CID]string),
		spaceWidth:           make(map[font.Instance]float64),
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
		if !e.UseActualText {
			return nil
		}

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
			fontSpaceWidth, ok := e.spaceWidth[e.reader.TextFont]
			if !ok {
				fontSpaceWidth = getSpaceWidth(e.reader.TextFont)
				e.spaceWidth[e.reader.TextFont] = fontSpaceWidth
			}
			if arg > 0.3*fontSpaceWidth {
				fmt.Fprint(e.writer, " ")
			}
		case reader.TextEventNL, reader.TextEventMove:
			fmt.Fprintln(e.writer)
		}
	}

	e.reader.Character = func(cid cid.CID, text string) error {
		// suppress character output when inside ActualText region
		if e.actualTextStartDepth != -1 && len(e.reader.MarkedContentStack) >= e.actualTextStartDepth {
			return nil
		}

		if text == "" {
			currentFont := e.reader.TextFont
			cidMapping, ok := e.extraTextCache[currentFont]
			if !ok {
				cidMapping = getExtraMapping(currentFont)
				e.extraTextCache[currentFont] = cidMapping
			}
			text = cidMapping[cid]
		}

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

	actualTextObj, err := mc.Properties.Get("ActualText")
	if err != nil {
		return
	}

	text, err := pdf.GetTextString(e.reader.R, actualTextObj.AsPDF(0))
	if err != nil {
		return
	}

	fmt.Fprint(e.writer, text)
	e.actualTextStartDepth = len(e.reader.MarkedContentStack)
}

func (e *TextExtractor) handleActualTextEnd() {
	if len(e.reader.MarkedContentStack) < e.actualTextStartDepth {
		e.actualTextStartDepth = -1
	}
}

// ExtractPage extracts text from a page dictionary.
func (e *TextExtractor) ExtractPage(pageDict pdf.Dict) error {
	return e.reader.ParsePage(pageDict, matrix.Identity)
}
