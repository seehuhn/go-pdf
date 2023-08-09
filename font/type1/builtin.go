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

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
)

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

func (f Builtin) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	info, err := getBuiltin(f)
	if err != nil {
		return nil, err
	}

	res := &embedded{
		Font: info,
		w:    w,
		Resource: pdf.Resource{
			Ref:  w.Alloc(),
			Name: resName,
		},
		SimpleEncoder: cmap.NewSimpleEncoder(),
	}

	w.AutoClose(res)

	return res, nil
}

func (f Builtin) GetGeometry() *font.Geometry {
	info, _ := getBuiltin(f)
	return info.GetGeometry()
}

func (f Builtin) Layout(s string, ptSize float64) glyph.Seq {
	info, _ := getBuiltin(f)
	return info.Layout(s, ptSize)
}

func getBuiltin(f Builtin) (*Font, error) {
	fontCacheLock.Lock()
	defer fontCacheLock.Unlock()

	if res, ok := fontCache[f]; ok {
		return res, nil
	}

	psFont, err := f.Afm()
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

var (
	fontCache     = make(map[Builtin]*Font)
	fontCacheLock sync.Mutex
)
