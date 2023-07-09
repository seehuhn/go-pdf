// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package builtin

import (
	"embed"

	"seehuhn.de/go/sfnt/afm"
)

// Afm returns the font metrics for one of the built-in pdf fonts.
// FontName must be one of the names listed in [FontNames].
func Afm(fontName string) (*afm.Info, error) {
	fd, err := afmData.Open("afm/" + fontName + ".afm")
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	res, err := afm.Read(fd)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// The names of the 14 built-in PDF fonts.
// These are the valid arguments for the Afm() function.
const (
	Courier              = "Courier"
	CourierBold          = "Courier-Bold"
	CourierBoldOblique   = "Courier-BoldOblique"
	CourierOblique       = "Courier-Oblique"
	Helvetica            = "Helvetica"
	HelveticaBold        = "Helvetica-Bold"
	HelveticaBoldOblique = "Helvetica-BoldOblique"
	HelveticaOblique     = "Helvetica-Oblique"
	TimesRoman           = "Times-Roman"
	TimesBold            = "Times-Bold"
	TimesBoldItalic      = "Times-BoldItalic"
	TimesItalic          = "Times-Italic"
	Symbol               = "Symbol"
	ZapfDingbats         = "ZapfDingbats"
)

// FontNames contains the names of the 14 built-in PDF fonts.
var FontNames = []string{
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

//go:embed afm/*.afm
var afmData embed.FS
