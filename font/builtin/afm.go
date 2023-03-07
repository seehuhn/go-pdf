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
	"bufio"
	"embed"
	"strconv"
	"strings"

	"seehuhn.de/go/sfnt/funit"
	"seehuhn.de/go/sfnt/glyph"
)

// AfmInfo represent the font metrics and built-in character encoding
// of an Adobe Type 1 font.
type AfmInfo struct {
	FontName string

	IsFixedPitch bool
	IsDingbats   bool

	Ascent    funit.Int16
	Descent   funit.Int16 // negative
	CapHeight funit.Int16
	XHeight   funit.Int16

	Code         []int16 // code byte, or -1 if unmapped
	GlyphExtents []funit.Rect
	Widths       []funit.Int16
	GlyphName    []string

	Ligatures map[glyph.Pair]glyph.ID
	Kern      map[glyph.Pair]funit.Int16
}

// Afm returns the font metrics for one of the built-in pdf fonts.
// FontName must be one of the names listed in [FontNames].
func Afm(fontName string) (*AfmInfo, error) {
	fd, err := afmData.Open("afm/" + fontName + ".afm")
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	res := &AfmInfo{}

	type ligInfo struct {
		first, second, combined string
	}
	var nameLigs []*ligInfo

	type kernInfo struct {
		first, second string
		val           funit.Int16
	}
	var nameKern []*kernInfo

	nameToGid := make(map[string]glyph.ID)

	charMetrics := false
	kernPairs := false
	res.IsDingbats = fontName == "ZapfDingbats"
	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "EndCharMetrics") {
			charMetrics = false
			continue
		}
		if charMetrics {
			var name string
			var width funit.Int16
			var code int
			var BBox funit.Rect
			var ligTmp []*ligInfo

			keyVals := strings.Split(line, ";")
			for _, keyVal := range keyVals {
				ff := strings.Fields(keyVal)
				if len(ff) < 2 {
					continue
				}
				switch ff[0] {
				case "C":
					code, _ = strconv.Atoi(ff[1])
				case "WX":
					tmp, _ := strconv.Atoi(ff[1])
					width = funit.Int16(tmp)
				case "N":
					name = ff[1]
				case "B":
					conv := func(in string) funit.Int16 {
						x, _ := strconv.Atoi(in)
						return funit.Int16(x)
					}
					BBox.LLx = conv(ff[1])
					BBox.LLy = conv(ff[2])
					BBox.URx = conv(ff[3])
					BBox.URy = conv(ff[4])
				case "L":
					ligTmp = append(ligTmp, &ligInfo{
						second:   ff[1],
						combined: ff[2],
					})
				}
			}

			nameToGid[name] = glyph.ID(len(res.Code))

			res.Code = append(res.Code, int16(code))
			res.Widths = append(res.Widths, width)
			res.GlyphExtents = append(res.GlyphExtents, BBox)
			res.GlyphName = append(res.GlyphName, name)

			for _, lig := range ligTmp {
				lig.first = name
				nameLigs = append(nameLigs, lig)
			}

			continue
		}

		fields := strings.Fields(line)

		if fields[0] == "EndKernPairs" {
			kernPairs = false
			continue
		}
		if kernPairs {
			x, _ := strconv.Atoi(fields[3])
			kern := &kernInfo{
				first:  fields[1],
				second: fields[2],
				val:    funit.Int16(x),
			}
			nameKern = append(nameKern, kern)
			continue
		}

		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "FontName":
			res.FontName = fields[1]
		case "IsFixedPitch":
			res.IsFixedPitch = fields[1] == "true"
		case "CapHeight":
			x, _ := strconv.Atoi(fields[1])
			res.CapHeight = funit.Int16(x)
		case "XHeight":
			x, _ := strconv.Atoi(fields[1])
			res.XHeight = funit.Int16(x)
		case "Ascender":
			x, _ := strconv.Atoi(fields[1])
			res.Ascent = funit.Int16(x)
		case "Descender":
			x, _ := strconv.Atoi(fields[1])
			res.Descent = funit.Int16(x)
		case "StartCharMetrics":
			charMetrics = true
		case "StartKernPairs":
			kernPairs = true
		}
	}
	if err := scanner.Err(); err != nil {
		panic("corrupted afm data for " + fontName)
	}

	res.Ligatures = make(map[glyph.Pair]glyph.ID)
	for _, lig := range nameLigs {
		a, aOk := nameToGid[lig.first]
		b, bOk := nameToGid[lig.second]
		c, cOk := nameToGid[lig.combined]
		if aOk && bOk && cOk {
			res.Ligatures[glyph.Pair{Left: a, Right: b}] = c
		}
	}

	res.Kern = make(map[glyph.Pair]funit.Int16)
	for _, kern := range nameKern {
		a, aOk := nameToGid[kern.first]
		b, bOk := nameToGid[kern.second]
		if aOk && bOk && kern.val != 0 {
			res.Kern[glyph.Pair{Left: a, Right: b}] = kern.val
		}
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
