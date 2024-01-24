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

package type1

import (
	"sync"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/graphics"
)

// Builtin is one of the 14 built-in PDF fonts.
type Builtin string

// The 14 built-in PDF fonts.
const (
	Courier              Builtin = "Courier"
	CourierBold          Builtin = "Courier-Bold"
	CourierBoldOblique   Builtin = "Courier-BoldOblique"
	CourierOblique       Builtin = "Courier-Oblique"
	Helvetica            Builtin = "Helvetica"
	HelveticaBold        Builtin = "Helvetica-Bold"
	HelveticaBoldOblique Builtin = "Helvetica-BoldOblique"
	HelveticaOblique     Builtin = "Helvetica-Oblique"
	TimesRoman           Builtin = "Times-Roman"
	TimesBold            Builtin = "Times-Bold"
	TimesBoldItalic      Builtin = "Times-BoldItalic"
	TimesItalic          Builtin = "Times-Italic"
	Symbol               Builtin = "Symbol"
	ZapfDingbats         Builtin = "ZapfDingbats"
)

// All contains the 14 built-in PDF fonts.
var All = []Builtin{
	Courier,
	CourierBold,
	CourierBoldOblique,
	CourierOblique,
	Helvetica,
	HelveticaBold,
	HelveticaBoldOblique,
	HelveticaOblique,
	TimesRoman,
	TimesBold,
	TimesBoldItalic,
	TimesItalic,
	Symbol,
	ZapfDingbats,
}

// Embed implements the [font.Font] interface.
func (f Builtin) Embed(w pdf.Putter, resName pdf.Name) (font.Layouter, error) {
	info, err := getBuiltin(f)
	if err != nil {
		return nil, err
	}

	res := &embedded{
		Font: info,
		w:    w,
		Res: graphics.Res{
			Ref:     w.Alloc(),
			DefName: resName,
		},
		SimpleEncoder: encoding.NewSimpleEncoder(),
	}
	w.AutoClose(res)
	return res, nil
}

// GetGeometry implements the [font.Font] interface.
func (f Builtin) GetGeometry() *font.Geometry {
	info, _ := getBuiltin(f)
	return info.GetGeometry()
}

// Layout implements the [font.Font] interface.
func (f Builtin) Layout(s string) glyph.Seq {
	info, _ := getBuiltin(f)
	return info.Layout(s)
}

func getBuiltin(f Builtin) (*Font, error) {
	fontCacheLock.Lock()
	defer fontCacheLock.Unlock()

	if res, ok := fontCache[f]; ok {
		return res, nil
	}

	psFont, err := f.PSFont()
	if err != nil {
		return nil, err
	}
	res, err := New(psFont)
	if err != nil {
		return nil, err
	}

	fontCache[f] = res
	return res, nil
}

// PSFont returns the font metrics for one of the built-in pdf fonts.
func (f Builtin) PSFont() (*type1.Font, error) {
	L := loader.New()

	fd, err := L.Open(string(f), loader.FontTypeAFM)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	res, err := afm.Read(fd)
	if err != nil {
		return nil, err
	}

	res.FontInfo.FontName = string(f)

	return res, nil
}

// IsBuiltin returns true if the given font is one of the 14 built-in PDF fonts.
func IsBuiltin(f *type1.Font) bool {
	// TODO(voss): a font is one of the 14 standard fonts if the name is one of
	// the 14 standard names and no glyph data was given.

	b, err := Builtin(f.FontInfo.FontName).PSFont()
	if err != nil {
		return false
	}

	// TODO(voss): Is the following test too strict?
	for name, fi := range f.GlyphInfo {
		bi, ok := b.GlyphInfo[name]
		if !ok {
			return false
		}
		if fi.WidthX != bi.WidthX {
			return false
		}
	}

	return true
}

var (
	fontCache     = make(map[Builtin]*Font)
	fontCacheLock sync.Mutex
)
