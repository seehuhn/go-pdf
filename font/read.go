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
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/type1"
)

// FromFile represents a font read from a PDF file.
type FromFile struct {
	Name pdf.Name
	Ref  pdf.Object
	charcode.CodeSpaceRange
	M     map[charcode.CharCode]type1.CID // TODO(voss): remove
	WMode int                             // 0 = horizontal, 1 = vertical

	AllWidther
}

// Read extracts a font from a PDF file.
//
// TODO(voss): return NewFont instead?
// TODO(voss): can we get away without the name argument?
func Read(r pdf.Getter, ref pdf.Object, name pdf.Name) (*FromFile, error) {
	fontDicts, err := ExtractDicts(r, ref)
	if err != nil {
		return nil, err
	}

	if name == "" {
		name, _ = fontDicts.FontDict["Name"].(pdf.Name)
	}

	var cs charcode.CodeSpaceRange
	var m map[charcode.CharCode]type1.CID
	writingMode := 0
	var widther AllWidther
	if fontDicts.Type.IsComposite() {
		cmapInfo, err := cmap.Extract(r, fontDicts.FontDict["Encoding"])
		if err != nil {
			return nil, pdf.Wrap(err, "Encoding")
		}

		// TODO(voss): read this information from cmapInfo instead of
		// expanding the cmap into a map?
		cs = cmapInfo.CodeSpaceRange
		m = cmapInfo.GetMapping()

		writingMode = cmapInfo.WMode

		widther, err = newCIDWidther(r, cmapInfo, fontDicts)
		if err != nil {
			return nil, err
		}
	} else {
		cs = charcode.Simple
		m = make(map[charcode.CharCode]type1.CID, 256)
		for i := 0; i < 256; i++ {
			m[charcode.CharCode(i)] = type1.CID(i)
		}

		// TODO(voss): somehow handle width information for the standard fonts

		widther, err = newSimpleWidther(r, fontDicts)
		if err != nil {
			return nil, err
		}
	}

	res := &FromFile{
		Ref:            ref,
		Name:           name,
		CodeSpaceRange: cs,
		M:              m,
		WMode:          writingMode,
		AllWidther:     widther,
	}
	return res, nil
}

// DefaultName implements the [NewFont] interface.
func (f *FromFile) DefaultName() pdf.Name {
	return f.Name
}

// PDFObject implements the [NewFont] interface.
func (f *FromFile) PDFObject() pdf.Object {
	return f.Ref
}

// WritingMode implements the [NewFont] interface.
func (f *FromFile) WritingMode() int {
	return f.WMode
}

type simpleWidther struct {
	W []float64
}

func newSimpleWidther(r pdf.Getter, fontInfo *Dicts) (*simpleWidther, error) {
	firstChar, err := pdf.GetInteger(r, fontInfo.FontDict["FirstChar"])
	if err != nil {
		return nil, pdf.Wrap(err, "FirstChar")
	} else if firstChar < 0 || firstChar > 255 {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("invalid FirstChar=%d", firstChar),
		}
	}
	lastChar, err := pdf.GetInteger(r, fontInfo.FontDict["LastChar"])
	if err != nil {
		return nil, pdf.Wrap(err, "LastChar")
	} else if lastChar < 0 || lastChar > 255 {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("invalid LastChar=%d", lastChar),
		}
	}
	var dw float64
	if fontInfo.FontDescriptor != nil {
		dw = float64(fontInfo.FontDescriptor.MissingWidth)
	}
	ww, err := pdf.GetArray(r, fontInfo.FontDict["Widths"])
	if err != nil {
		return nil, pdf.Wrap(err, "Widths")
	} else if len(ww) != int(lastChar)-int(firstChar)+1 {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("malformed Widths array"),
		}
	}

	w := make([]float64, 256)
	for i := range w {
		if i < int(firstChar) || i > int(lastChar) {
			w[i] = dw
			continue
		}

		wi := i - int(firstChar)
		x, err := pdf.GetNumber(r, ww[wi])
		if err != nil {
			return nil, pdf.Wrap(err, fmt.Sprintf("Widths[%d]", wi))
		}
		w[i] = float64(x)
	}
	return &simpleWidther{W: w}, nil
}

func (w *simpleWidther) AllWidths(s pdf.String) func(yield func(w float64, isSpace bool) bool) bool {
	return func(yield func(w float64, isSpace bool) bool) bool {
		for _, c := range s {
			if !yield(w.W[c], c == 0x20) {
				return false
			}
		}
		return true
	}
}

func (w *simpleWidther) GlyphWidth(c type1.CID) float64 {
	return w.W[c]
}

type compositeWidther struct {
	charcode.CodeSpaceRange
	M  map[charcode.CharCode]type1.CID
	DW float64
	W  map[type1.CID]float64
}

func newCIDWidther(r pdf.Getter, cmap *cmap.Info, fontInfo *Dicts) (*compositeWidther, error) {
	dw := 100.0
	if val, ok := fontInfo.CIDFontDict["DW"]; ok {
		x, err := pdf.GetNumber(r, val)
		if err != nil {
			return nil, pdf.Wrap(err, "CIDFontDict.DW")
		}
		dw = float64(x)
	}
	w, err := DecodeWidthsComposite(r, fontInfo.CIDFontDict["W"], dw)
	if err != nil {
		return nil, pdf.Wrap(err, "CIDFontDict.W")
	}
	return &compositeWidther{
		CodeSpaceRange: cmap.CodeSpaceRange,
		M:              cmap.GetMapping(),
		DW:             float64(dw),
		W:              w,
	}, nil
}

func (w *compositeWidther) AllWidths(s pdf.String) func(yield func(w float64, isSpace bool) bool) bool {
	return func(yield func(w float64, isSpace bool) bool) bool {
		return w.CodeSpaceRange.AllCodes(s)(func(c pdf.String, valid bool) bool {
			if !valid {
				return yield(w.DW, false)
			}
			code, k := w.CodeSpaceRange.Decode(c)
			if k != len(c) {
				panic("internal error")
			}
			return yield(w.W[w.M[code]], len(c) == 1 && c[0] == 0x20)
		})
	}
}

func (w *compositeWidther) GlyphWidth(c type1.CID) float64 {
	return w.W[c]
}
