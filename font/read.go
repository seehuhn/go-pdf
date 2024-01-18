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
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/sfnt/glyph"
)

// Read extracts a font from a PDF file.
func Read(r pdf.Getter, ref pdf.Object, name pdf.Name) (NewFont, error) {
	fontDicts, err := ExtractDicts(r, ref)
	if err != nil {
		return nil, err
	}

	switch fontDicts.Type {
	case Builtin: // built-in fonts
	case CFFComposite: // CFF font data without wrapper (composite font)
	case CFFSimple: // CFF font data without wrapper (simple font)
		// cff.ExtractSimple(r, fontDicts)
	case MMType1: // Multiple Master type 1 fonts
	case OpenTypeCFFComposite: // CFF fonts in an OpenType wrapper (composite font)
	case OpenTypeCFFSimple: // CFF font data in an OpenType wrapper (simple font)
	case OpenTypeGlyfComposite: // OpenType fonts with glyf outline (composite font)
	case OpenTypeGlyfSimple: // OpenType fonts with glyf outline (simple font)
	case TrueTypeComposite: // TrueType fonts (composite font)
	case TrueTypeSimple: // TrueType fonts (simple font)
	case Type1: // Type 1 fonts
	case Type3: // Type 3 fonts
	}

	// TODO(voss): always override the code space range?
	toUnicode, err := cmap.ExtractToUnicode(r, fontDicts.FontDict["ToUnicode"], nil)
	if err != nil {
		return nil, pdf.Wrap(err, "ToUnicode")
	}

	if fontDicts.Type.IsComposite() {
		// 	cmapInfo, err := cmap.Extract(r, fontDicts.FontDict["Encoding"])
		// 	if err != nil {
		// 		return nil, pdf.Wrap(err, "Encoding")
		// 	}

		// 	// TODO(voss): read this information from cmapInfo instead of
		// 	// expanding the cmap into a map?
		// 	cs = cmapInfo.CodeSpaceRange
		// 	m = cmapInfo.GetMapping()

		// 	writingMode = cmapInfo.WMode

		// 	widther, err = newCIDWidther(r, cmapInfo, fontDicts)
		// 	if err != nil {
		// 		return nil, err
		// 	}
		res := &fromFile{
			Name:  name,
			Ref:   ref,
			WMode: 0, // TODO(voss)
			ToUni: toUnicode.GetMapping(),
		}
		return res, nil
	} else {
		// widther, err = newSimpleWidther(r, fontDicts)
		// if err != nil {
		// 	return nil, err
		// }
		res := &fromFile{
			Name:  name,
			Ref:   ref,
			ToUni: toUnicode.GetMapping(),
		}
		return res, nil
	}
}

type fromFileSimple struct {
	fromFile
}

func (f *fromFileSimple) CodeToGID(byte) glyph.ID {
	panic("not implemented")
}

func (f *fromFileSimple) GIDToCode(glyph.ID, []rune) byte {
	panic("not implemented")
}

func (f *fromFileSimple) CodeToWidth(byte) float64 {
	panic("not implemented")
}

type fromFile struct {
	Name  pdf.Name
	Ref   pdf.Object
	WMode int
	ToUni map[charcode.CharCode][]rune
}

func (f *fromFile) DefaultName() pdf.Name {
	return f.Name
}

func (f *fromFile) PDFObject() pdf.Object {
	return f.Ref
}

func (f *fromFile) WritingMode() int {
	return f.WMode
}

func (f *fromFile) AsText(pdf.String) []rune {
	panic("not implemented")
}
