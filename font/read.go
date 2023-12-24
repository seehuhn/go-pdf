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
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/type1"
)

type fromFile struct {
	Name   pdf.Name
	Object pdf.Object
	charcode.CodeSpaceRange
	M     map[charcode.CharCode]type1.CID
	WMode int // 0 = horizontal, 1 = vertical
	DW    float64
	W     map[type1.CID]float64
}

// Read reads a font from a PDF file.
func Read(r pdf.Getter, obj pdf.Object, name pdf.Name) (NewFont, error) {
	fontDicts, err := ExtractDicts(r, obj)
	if err != nil {
		return nil, err
	}

	if name == "" {
		name, _ = fontDicts.FontDict["Name"].(pdf.Name)
	}

	var cs charcode.CodeSpaceRange
	var m map[charcode.CharCode]type1.CID
	writingMode := 0
	var dw float64
	w := make(map[type1.CID]float64)
	if fontDicts.Type.IsComposite() {
		cmapInfo, err := cmap.Extract(r, fontDicts.FontDict["Encoding"])
		if err != nil {
			return nil, pdf.Wrap(err, "Encoding")
		}

		// TODO(voss): read this information from cmapInfo instead of
		// expanding the cmap into a map?
		cs = cmapInfo.CS
		m = cmapInfo.GetMapping()

		writingMode = cmapInfo.WMode

		w, dw, err = DecodeWidthsComposite(r, fontDicts.CIDFontDict["W"])
		if err != nil {
			return nil, pdf.Wrap(err, "W, DW")
		}
	} else {
		cs = charcode.Simple
		m = make(map[charcode.CharCode]type1.CID, 256)
		for i := 0; i < 256; i++ {
			m[charcode.CharCode(i)] = type1.CID(i)
		}

		// TODO(voss): handle width information for the standard fonts

		firstChar, err := pdf.GetInteger(r, fontDicts.FontDict["FirstChar"])
		if err != nil {
			return nil, pdf.Wrap(err, "FirstChar")
		}
		lastChar, err := pdf.GetInteger(r, fontDicts.FontDict["LastChar"])
		if err != nil {
			return nil, pdf.Wrap(err, "LastFirst")
		}
		var dw float64
		if fontDicts.FontDescriptor != nil {
			dw = float64(fontDicts.FontDescriptor.MissingWidth)
		}
		ww, err := pdf.GetArray(r, fontDicts.FontDict["Widths"])
		if err != nil {
			return nil, pdf.Wrap(err, "Widths")
		} else if len(ww) != int(lastChar)-int(firstChar)+1 {
			return nil, &pdf.MalformedFileError{
				Err: errors.New("malformed Widths"),
			}
		}
		for i, wi := range ww {
			x, err := pdf.GetNumber(r, wi)
			if err != nil {
				return nil, err
			}
			if float64(x) != dw {
				w[type1.CID(i)+type1.CID(firstChar)] = float64(x)
			}
		}
	}

	res := &fromFile{
		Object:         obj,
		Name:           name,
		CodeSpaceRange: cs,
		M:              m,
		WMode:          writingMode,
		DW:             dw,
		W:              w,
	}
	return res, nil
}

func (f *fromFile) DefaultName() pdf.Name {
	return f.Name
}

func (f *fromFile) PDFObject() pdf.Object {
	return f.Object
}

func (f *fromFile) WritingMode() int {
	return f.WMode
}

func (f *fromFile) SplitString(s pdf.String) []type1.CID {
	var res []type1.CID
	for len(s) > 0 {
		c, n := f.CodeSpaceRange.Decode(s)
		s = s[n:]
		if c >= 0 {
			// TODO(voss): what to do if c is not in the cmap?
			res = append(res, f.M[c])
		}
	}
	return res
}

func (f *fromFile) GlyphWidth(c type1.CID) float64 {
	w, ok := f.W[c]
	if !ok {
		w = f.DW
	}
	return w
}
