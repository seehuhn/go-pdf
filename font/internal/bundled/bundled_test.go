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

package bundled

import (
	"testing"

	"seehuhn.de/go/pdf/internal/stdmtx"
)

func TestReadAFM(t *testing.T) {
	for name := range stdmtx.Metrics {
		metrics, err := ReadAFM(name)
		if err != nil {
			t.Fatal(err)
		}
		if metrics.FontName != name {
			t.Errorf("%s: font name %q", name, metrics.FontName)
		}
		if _, ok := metrics.Glyphs["space"]; !ok {
			t.Errorf("%s: no space glyph", name)
		}
	}
}

func TestReadSfnt(t *testing.T) {
	for name := range stdmtx.Metrics {
		font, err := ReadSfnt(name)
		if err != nil {
			t.Fatal(err)
		}
		if font.NumGlyphs() == 0 {
			t.Errorf("%s: no glyphs", name)
		}
	}
}

// A name this library bundles no font for is an error rather than an empty
// font: the packages built on this one treat it as a broken installation.
func TestReadUnknown(t *testing.T) {
	if _, err := ReadAFM("NoSuchFont"); err == nil {
		t.Error("metrics which are not bundled were read without an error")
	}
	if _, err := ReadSfnt("NoSuchFont"); err == nil {
		t.Error("a font which is not bundled was read without an error")
	}
}

// TestAFMComplete checks that the AFM files can be used as they stand: they
// give an empty bounding box for the space, and the Latin fonts give ascent,
// descent and the fi and fl ligatures.
func TestAFMComplete(t *testing.T) {
	for name := range stdmtx.Metrics {
		metrics, err := ReadAFM(name)
		if err != nil {
			t.Fatal(err)
		}

		// whitespace glyphs have an empty bounding box
		if g := metrics.Glyphs["space"]; g == nil {
			t.Errorf("%s: no space glyph", name)
		} else if !g.BBox.IsZero() {
			t.Errorf("%s: space covers %v", name, g.BBox)
		}

		if name == "Symbol" || name == "ZapfDingbats" {
			continue
		}

		// ascent and descent are given
		if metrics.Ascent <= 0 || metrics.Descent >= 0 {
			t.Errorf("%s: ascent %v, descent %v", name, metrics.Ascent, metrics.Descent)
		}

		// the f-ligatures are given
		f := metrics.Glyphs["f"]
		if f == nil {
			t.Errorf("%s: no glyph f", name)
			continue
		}
		for _, pair := range [][2]string{{"i", "fi"}, {"l", "fl"}} {
			if got := f.Ligatures[pair[0]]; got != pair[1] {
				t.Errorf("%s: f followed by %s gives %q, want %q",
					name, pair[0], got, pair[1])
			}
		}
	}
}
