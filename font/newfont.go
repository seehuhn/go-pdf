// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package font

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt/glyph"
)

type NewFont interface {
	DefaultName() pdf.Name // return "" to choose names automatically
	PDFObject() pdf.Object // value to use in the resource dictionary
	WritingMode() int      // 0 = horizontal, 1 = vertical
	AsText(pdf.String) []rune
	// Outlines() interface{}
}

type NewFontLayouter interface {
	NewFont
	Layout(s string) glyph.Seq
	FontMatrix() []float64
}

type NewFontSimple interface {
	NewFont
	CodeToGID(byte) glyph.ID
	GIDToCode(glyph.ID, []rune) byte
	CodeToWidth(byte) float64 // scaled PDF text space units
}

type NewFontComposite interface {
	NewFont
	CS() charcode.CodeSpaceRange
	CodeToCID(pdf.String) type1.CID
	AppendCode(pdf.String, type1.CID) pdf.String
	GID(type1.CID) glyph.ID
	CID(glyph.ID, []rune) type1.CID
	CIDToWidth(type1.CID) float64
}
