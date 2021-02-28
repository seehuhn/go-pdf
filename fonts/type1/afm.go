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

	"seehuhn.de/go/pdf/fonts"
)

//go:embed afm/*.afm
var afmData embed.FS

type afmMap struct {
	sync.Mutex

	data map[string]*fonts.Font
}

// BuiltIn returns information about one of the built-in PDF fonts.
func BuiltIn(fontName string, encoding fonts.Encoding, ptSize float64) *fonts.Font {
	raw := afm.Lookup(fontName, encoding)
	if raw == nil {
		return nil
	}

	// Units in an afm file are in 1/1000 of the scale of the font being
	// formatted. Multiplying with the scale factor gives values in 1000*bp.
	q := ptSize / 1000

	f := &fonts.Font{
		FontName:  raw.FontName,
		FullName:  raw.FullName,
		FontSize:  ptSize,
		CapHeight: raw.CapHeight * q,
		XHeight:   raw.XHeight * q,
		Ascender:  raw.Ascender * q,
		Descender: raw.Descender * q,
		Encoding:  encoding,
		Width:     make(map[byte]float64),
		BBox:      make(map[byte]*fonts.Rect),
		Ligatures: raw.Ligatures,
		Kerning:   raw.Kerning,
	}
	for c, w := range raw.Width {
		f.Width[c] = w * q
	}
	for c, box := range raw.BBox {
		f.BBox[c] = &fonts.Rect{
			LLx: box.LLx * q,
			LLy: box.LLy * q,
			URx: box.URx * q,
			URy: box.URy * q,
		}
	}

	return f
}

func (m *afmMap) Lookup(fontName string, encoding fonts.Encoding) *fonts.Font {
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

	f = &fonts.Font{
		Encoding:  encoding,
		Width:     make(map[byte]float64),
		BBox:      make(map[byte]*fonts.Rect),
		Ligatures: make(map[fonts.GlyphPair]byte),
		Kerning:   make(map[fonts.GlyphPair]float64),
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
			BBox := &fonts.Rect{}
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
			f.FontName = fields[1]
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
		f.Ligatures[fonts.GlyphPair{a, b}] = c
	}

	for _, kern := range kerning {
		a, aOk := byName[kern.first]
		b, bOk := byName[kern.second]
		if !aOk || !bOk || kern.val == 0 {
			continue
		}
		f.Kerning[fonts.GlyphPair{a, b}] = kern.val
	}

	return f
}

var afm = &afmMap{
	data: make(map[string]*fonts.Font),
}
