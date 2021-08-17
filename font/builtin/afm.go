// seehuhn.de/go/pdf - support for reading and writing PDF files
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

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/names"
)

//go:embed afm/*.afm
var afmData embed.FS

type afmFont struct {
	FontName string
	Ascent   float64
	Descent  float64

	Code        []byte
	GlyphExtent []font.Rect
	Width       []int
	Chars       []rune

	Ligatures map[font.GlyphPair]font.GlyphID
	Kern      map[font.GlyphPair]int
}

func readAfmFont(fname string) (*afmFont, error) {
	fd, err := afmData.Open("afm/" + fname + ".afm")
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	res := &afmFont{}

	type ligInfo struct {
		first, second, combined string
	}
	var nameLigs []*ligInfo

	type kernInfo struct {
		first, second string
		val           int
	}
	var nameKern []*kernInfo

	nameToGid := make(map[string]font.GlyphID)

	// prepend an artificial entry for .notdef, so that CMap works
	res.Code = append(res.Code, 0)
	res.Width = append(res.Width, 250) // TODO(voss): what is the correct width?
	res.GlyphExtent = append(res.GlyphExtent, font.Rect{})
	res.Chars = append(res.Chars, 0)

	charMetrics := false
	kernPairs := false
	dingbats := fname == "ZapfDingbats"
	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "EndCharMetrics") {
			charMetrics = false
			continue
		}
		if charMetrics {
			var name string
			var width int
			var code int
			var BBox font.Rect
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
					width, _ = strconv.Atoi(ff[1])
				case "N":
					name = ff[1]
				case "B":
					if len(ff) != 5 {
						panic("corrupted afm data for " + fname)
					}
					BBox.LLx, _ = strconv.Atoi(ff[1])
					BBox.LLy, _ = strconv.Atoi(ff[2])
					BBox.URx, _ = strconv.Atoi(ff[3])
					BBox.URy, _ = strconv.Atoi(ff[4])
				case "L":
					if len(ff) != 3 {
						panic("corrupted afm data for " + fname)
					}
					ligTmp = append(ligTmp, &ligInfo{
						second:   ff[1],
						combined: ff[2],
					})
				default:
					panic(ff[0] + " not implemented")
				}
			}

			rr := names.ToUnicode(name, dingbats)
			if len(rr) != 1 {
				panic("not implemented")
			}
			r := rr[0]

			nameToGid[name] = font.GlyphID(len(res.Code))

			res.Code = append(res.Code, byte(code))
			res.Width = append(res.Width, width)
			res.GlyphExtent = append(res.GlyphExtent, BBox)
			res.Chars = append(res.Chars, r)

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
			if len(fields) != 4 || fields[0] != "KPX" {
				panic("unsupported KernPair " + line)
			}
			kern := &kernInfo{
				first:  fields[1],
				second: fields[2],
			}
			kern.val, _ = strconv.Atoi(fields[3])
			nameKern = append(nameKern, kern)
			continue
		}

		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MetricsSets":
			if fields[1] != "0" {
				panic("unsupported writing direction")
			}
		case "FontName":
			res.FontName = fields[1]
		case "CapHeight":
			// x, err := strconv.ParseFloat(fields[1], 64)
			// if err != nil {
			// 	panic("corrupted afm data for " + fname)
			// }
			// builtin.CapHeight = x
		case "XHeight":
			// x, err := strconv.ParseFloat(fields[1], 64)
			// if err != nil {
			// 	panic("corrupted afm data for " + fname)
			// }
			// builtin.XHeight = x
		case "Ascender":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fname)
			}
			res.Ascent = x
		case "Descender":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fname)
			}
			res.Descent = x
		case "CharWidth":
			panic("not implemented")
		case "StartCharMetrics":
			charMetrics = true
		case "StartKernPairs":
			kernPairs = true
		case "StartTrackKern":
			panic("not implemented")
		}
	}
	if err := scanner.Err(); err != nil {
		panic("corrupted afm data for " + fname)
	}

	// TODO(voss): only use kerning/ligatures for proportional fonts

	res.Ligatures = make(map[font.GlyphPair]font.GlyphID)
	for _, lig := range nameLigs {
		a, aOk := nameToGid[lig.first]
		b, bOk := nameToGid[lig.second]
		c, cOk := nameToGid[lig.combined]
		if aOk && bOk && cOk {
			res.Ligatures[font.GlyphPair{a, b}] = c
		}
	}

	res.Kern = make(map[font.GlyphPair]int)
	for _, kern := range nameKern {
		a, aOk := nameToGid[kern.first]
		b, bOk := nameToGid[kern.second]
		if aOk && bOk && kern.val != 0 {
			res.Kern[font.GlyphPair{a, b}] = kern.val
		}
	}

	return res, nil
}
