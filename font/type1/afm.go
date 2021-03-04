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

package type1

import (
	"bufio"
	"embed"
	"strconv"
	"strings"
	"sync"

	"seehuhn.de/go/pdf/font"
)

// BuiltIn returns information about one of the built-in PDF fonts.
func BuiltIn(fontName string, encoding font.Encoding) *font.Font {
	return afm.Lookup(fontName, encoding)
}

var afm = &afmMap{
	data: make(map[string]*font.Font),
}

type afmMap struct {
	sync.Mutex

	data map[string]*font.Font
}

func (m *afmMap) Lookup(fontName string, encoding font.Encoding) *font.Font {
	m.Lock()
	defer m.Unlock()

	f, ok := m.data[fontName]
	if ok {
		return f
	}
	fd, err := afmData.Open("afm/" + fontName + ".afm")
	if err != nil {
		return nil
	}

	dingbats := fontName == "ZapfDingbats"

	f = &font.Font{
		Encoding:  encoding,
		Width:     make(map[byte]float64),
		BBox:      make(map[byte]*font.Rect),
		Ligatures: make(map[font.GlyphPair]byte),
		Kerning:   make(map[font.GlyphPair]float64),
	}
	byName := make(map[string]byte)
	type ligInfo struct {
		first, second, combined string
	}
	var ligatures []*ligInfo
	type kernInfo struct {
		first, second string
		val           float64
	}
	var kerning []*kernInfo

	charMetrics := false
	kernPairs := false
	scanner := bufio.NewScanner(fd)
glyphLoop:
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
			var charCode byte
			var width float64
			BBox := &font.Rect{}
			var ligTmp []*ligInfo

			keyVals := strings.Split(line, ";")
			for _, keyVal := range keyVals {
				ff := strings.Fields(keyVal)
				if len(ff) < 2 {
					continue
				}
				switch ff[0] {
				case "C":
					// TODO(voss): is this needed?
					// glyphIdx, _ = strconv.Atoi(ff[1])
				case "WX":
					width, _ = strconv.ParseFloat(ff[1], 64)
				case "N":
					name = ff[1]
					r := decodeGlyphName(name, dingbats)
					if len(r) != 1 {
						panic("not implemented")
					}
					c, ok := encoding.Encode(r[0])
					if !ok {
						continue glyphLoop
					}
					charCode = c
				case "B":
					if len(ff) != 5 {
						panic("corrupted afm data for " + fontName)
					}
					BBox.LLx, _ = strconv.ParseFloat(ff[1], 64)
					BBox.LLy, _ = strconv.ParseFloat(ff[2], 64)
					BBox.URx, _ = strconv.ParseFloat(ff[3], 64)
					BBox.URy, _ = strconv.ParseFloat(ff[4], 64)
				case "L":
					if len(ff) != 3 {
						panic("corrupted afm data for " + fontName)
					}
					ligTmp = append(ligTmp, &ligInfo{
						second:   ff[1],
						combined: ff[2],
					})
				default:
					panic(ff[0] + " not implemented")
				}
			}
			byName[name] = charCode
			if _, present := f.Width[charCode]; present {
				panic("duplicate character (invalid encoding)")
			}
			f.Width[charCode] = width
			f.BBox[charCode] = BBox
			for _, lig := range ligTmp {
				lig.first = name
				ligatures = append(ligatures, lig)
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
			kern.val, _ = strconv.ParseFloat(fields[3], 64)
			kerning = append(kerning, kern)
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
			f.BaseFont = fields[1]
		case "FullName":
			f.FullName = fields[1]
		case "CapHeight":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fontName)
			}
			f.CapHeight = x
		case "XHeight":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fontName)
			}
			f.XHeight = x
		case "Ascender":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fontName)
			}
			f.Ascender = x
		case "Descender":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fontName)
			}
			f.Descender = x
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
		panic("corrupted afm data for " + fontName)
	}

	for _, lig := range ligatures {
		a, aOk := byName[lig.first]
		b, bOk := byName[lig.second]
		c, cOk := byName[lig.combined]
		if !aOk || !bOk || !cOk {
			continue
		}
		f.Ligatures[font.GlyphPair{a, b}] = c
	}

	for _, kern := range kerning {
		a, aOk := byName[kern.first]
		b, bOk := byName[kern.second]
		if !aOk || !bOk || kern.val == 0 {
			continue
		}
		f.Kerning[font.GlyphPair{a, b}] = kern.val
	}

	return f
}

//go:embed afm/*.afm
var afmData embed.FS
