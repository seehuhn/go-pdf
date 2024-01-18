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
)

// Read extracts a font from a PDF file.
func Read(r pdf.Getter, ref pdf.Object, name pdf.Name) (NewFont, error) {
	fontDicts, err := ExtractDicts(r, ref)
	if err != nil {
		return nil, err
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
		// cs = charcode.Simple
		// m = make(map[charcode.CharCode]type1.CID, 256)
		// for i := 0; i < 256; i++ {
		// 	m[charcode.CharCode(i)] = type1.CID(i)
		// }

		// // TODO(voss): somehow handle width information for the standard fonts

		// widther, err = newSimpleWidther(r, fontDicts)
		// if err != nil {
		// 	return nil, err
		// }
		res := &fromFile{
			Name:  name,
			Ref:   ref,
			WMode: 0,
			ToUni: toUnicode.GetMapping(),
		}
		return res, nil
	}
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

func (f *fromFile) Outlines() interface{} {
	panic("not implemented")
}
