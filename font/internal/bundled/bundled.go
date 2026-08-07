// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// Package bundled serves the Type 1 fonts bundled with this library to the
// packages which name them.  It reads each font at most once, hands out
// instances cloned from the result, and carries the repairs the bundled
// metrics need.
package bundled

import (
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/postscript/afm"
	pstype1 "seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf/font/loader"
)

// builtin locates the font files bundled with this library.
var builtin = loader.NewFontLoader()

// Read reads the Type 1 font program and the metrics bundled under the given
// PostScript name.  Both are returned as they stand in the files; see
// [FixUpMetrics] for the repairs the metrics need.
func Read(name string) (*pstype1.Font, *afm.Metrics, error) {
	fontData, err := builtin.Open(name, loader.FontTypeType1)
	if err != nil {
		return nil, nil, err
	}
	defer fontData.Close()

	psFont, err := pstype1.Read(fontData)
	if err != nil {
		return nil, nil, err
	}

	afmData, err := builtin.Open(name, loader.FontTypeAFM)
	if err != nil {
		return nil, nil, err
	}
	defer afmData.Close()

	metrics, err := afm.Read(afmData)
	if err != nil {
		return nil, nil, err
	}

	return psFont, metrics, nil
}

// FixUpMetrics repairs the metrics of the bundled fonts, which give wrong
// values for some entries and omit others.
//
// A caller which restricts the glyph set must do so before calling this, so
// that the repairs see only the glyphs which survive: the values inferred here
// depend on which glyphs are present.
func FixUpMetrics(metrics *afm.Metrics) {
	// Some of the fonts wrongly have a non-zero bounding box for some of the
	// whitespace glyphs.  We fix this here.
	//
	// Revisit this, once
	// https://github.com/ArtifexSoftware/urw-base35-fonts/issues/48
	// is resolved.
	for _, name := range []string{"space", "uni00A0", "uni2002"} {
		if g, ok := metrics.Glyphs[name]; ok {
			g.BBox = rect.Rect{}
		}
	}

	// Ascent and descent are missing from our .afm files.  We infer values for
	// these from glyph metrics.
	for _, name := range []string{"d", "bracketleft", "bar"} {
		if glyph, ok := metrics.Glyphs[name]; ok {
			y := glyph.BBox.URy
			if y > metrics.Ascent {
				metrics.Ascent = y
			}
		}
	}
	for _, name := range []string{"p", "bracketleft", "bar"} {
		if glyph, ok := metrics.Glyphs[name]; ok {
			y := glyph.BBox.LLy
			if y < metrics.Descent {
				metrics.Descent = y
			}
		}
	}

	// We add the standard ligatures here, just in case.
	if !metrics.IsFixedPitch {
		type lig struct {
			left, right, result string
		}
		all := []lig{
			{"f", "f", "ff"},
			{"f", "i", "fi"},
			{"f", "l", "fl"},
			{"ff", "i", "ffi"},
			{"ff", "l", "ffl"},
		}
		for _, l := range all {
			_, leftOk := metrics.Glyphs[l.left]
			_, rightOk := metrics.Glyphs[l.right]
			_, resOk := metrics.Glyphs[l.result]
			if leftOk && rightOk && resOk {
				if len(metrics.Glyphs[l.left].Ligatures) == 0 {
					metrics.Glyphs[l.left].Ligatures = make(map[string]string)
				}
				metrics.Glyphs[l.left].Ligatures[l.right] = l.result
			}
		}
	}
}
