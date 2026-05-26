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

package dict

import (
	"slices"
	"testing"
	"time"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/mapping"
)

// encodeAt returns an encoding that names the given codes as "A".
func encodeAt(codes ...byte) encoding.Simple {
	return func(code byte) string {
		if slices.Contains(codes, code) {
			return "A"
		}
		return ""
	}
}

func TestType1GlyphWidth(t *testing.T) {
	d := &Type1{Encoding: encodeAt(0x41)}
	d.Width[0x41] = 600

	if w, ok := d.GlyphWidth("A"); !ok || w != 0.6 {
		t.Errorf("GlyphWidth(A) = %v, %v; want 0.6, true", w, ok)
	}
	if _, ok := d.GlyphWidth("Z"); ok {
		t.Error("GlyphWidth(Z) = true; want false")
	}
}

func TestType1GlyphWidthSmallestCode(t *testing.T) {
	d := &Type1{Encoding: encodeAt(0x05, 0x41)}
	d.Width[0x05] = 300
	d.Width[0x41] = 600

	// the smaller code supplies the width
	if w, ok := d.GlyphWidth("A"); !ok || w != 0.3 {
		t.Errorf("GlyphWidth(A) = %v, %v; want 0.3, true", w, ok)
	}
}

func TestTrueTypeGlyphWidth(t *testing.T) {
	d := &TrueType{Encoding: encodeAt(0x41)}
	d.Width[0x41] = 600

	if w, ok := d.GlyphWidth("A"); !ok || w != 0.6 {
		t.Errorf("GlyphWidth(A) = %v, %v; want 0.6, true", w, ok)
	}
}

func TestType3GlyphWidth(t *testing.T) {
	d := &Type3{
		Encoding:   encodeAt(0x41),
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
	}
	d.Width[0x41] = 600

	// width is scaled by the font matrix
	if w, ok := d.GlyphWidth("A"); !ok || w != 0.6 {
		t.Errorf("GlyphWidth(A) = %v, %v; want 0.6, true", w, ok)
	}
}

func TestCIDFontType0GlyphWidthToUnicode(t *testing.T) {
	d := &CIDFontType0{
		ROS:          cidTextROS,
		CMap:         cidTextCMap(), // 0x41 -> CID 5
		ToUnicode:    cidTextToUnicode(),
		Width:        map[cmap.CID]float64{5: 600},
		DefaultWidth: 250,
	}

	// "A" -> code 0x41 -> CID 5 -> width 600
	if w, ok := d.GlyphWidth("A"); !ok || w != 0.6 {
		t.Errorf("GlyphWidth(A) = %v, %v; want 0.6, true", w, ok)
	}
	if _, ok := d.GlyphWidth("Z"); ok {
		t.Error("GlyphWidth(Z) = true; want false")
	}
}

func TestCIDFontType0GlyphWidthDefault(t *testing.T) {
	d := &CIDFontType0{
		ROS:          cidTextROS,
		CMap:         cidTextCMap(),
		ToUnicode:    cidTextToUnicode(),
		DefaultWidth: 250, // CID 5 is not in Width
	}

	if w, ok := d.GlyphWidth("A"); !ok || w != 0.25 {
		t.Errorf("GlyphWidth(A) = %v, %v; want 0.25, true", w, ok)
	}
}

func TestCIDFontType0GlyphWidthROS(t *testing.T) {
	reverse, err := mapping.GetTextToCIDMapping("Adobe", "Japan1")
	if err != nil {
		t.Fatal(err)
	}
	spaceCID, ok := reverse[" "]
	if !ok {
		t.Skip("Adobe-Japan1 has no space mapping")
	}

	d := &CIDFontType0{
		ROS:   &cid.SystemInfo{Registry: "Adobe", Ordering: "Japan1"},
		Width: map[cmap.CID]float64{spaceCID: 480},
	}

	// no ToUnicode: text resolves through the registry/ordering mapping
	if w, ok := d.GlyphWidth(" "); !ok || w != 0.48 {
		t.Errorf("GlyphWidth(space) = %v, %v; want 0.48, true", w, ok)
	}
}

func TestCIDFontType0GlyphWidthToUnicodeWins(t *testing.T) {
	reverse, err := mapping.GetTextToCIDMapping("Adobe", "Japan1")
	if err != nil {
		t.Fatal(err)
	}
	rosCID, ok := reverse[" "]
	if !ok {
		t.Skip("Adobe-Japan1 has no space mapping")
	}

	const tuCID cmap.CID = 9999
	if rosCID == tuCID {
		t.Skip("CID collision in test setup")
	}

	d := &CIDFontType0{
		ROS: &cid.SystemInfo{Registry: "Adobe", Ordering: "Japan1"},
		CMap: &cmap.File{
			Name:           "Test",
			ROS:            &cid.SystemInfo{Registry: "Adobe", Ordering: "Japan1"},
			CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0xFF}}},
			CIDSingles:     []cmap.Single{{Code: []byte{0x20}, Value: tuCID}},
		},
		ToUnicode: &cmap.ToUnicodeFile{
			CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0xFF}}},
			Singles:        []cmap.ToUnicodeSingle{{Code: []byte{0x20}, Value: " "}},
		},
		Width: map[cmap.CID]float64{tuCID: 700, rosCID: 480},
	}

	// ToUnicode (CID 9999, width 700) takes precedence over the ROS default
	if w, ok := d.GlyphWidth(" "); !ok || w != 0.7 {
		t.Errorf("GlyphWidth(space) = %v, %v; want 0.7, true", w, ok)
	}
}

func TestCIDFontType2GlyphWidth(t *testing.T) {
	d := &CIDFontType2{
		ROS:          cidTextROS,
		CMap:         cidTextCMap(),
		ToUnicode:    cidTextToUnicode(),
		Width:        map[cmap.CID]float64{5: 600},
		DefaultWidth: 250,
	}

	if w, ok := d.GlyphWidth("A"); !ok || w != 0.6 {
		t.Errorf("GlyphWidth(A) = %v, %v; want 0.6, true", w, ok)
	}
}

// TestCIDFontType0GlyphWidthHostile checks that GlyphWidth stays cheap even on
// a font crafted to trigger the old quadratic blow-up: a wide CID range plus a
// wide ToUnicode range.
func TestCIDFontType0GlyphWidthHostile(t *testing.T) {
	wide := charcode.CodeSpaceRange{{Low: []byte{0, 0, 0, 0}, High: []byte{0xFF, 0xFF, 0xFF, 0xFF}}}
	d := &CIDFontType0{
		ROS: cidTextROS,
		CMap: &cmap.File{
			Name:           "Evil",
			ROS:            cidTextROS,
			CodeSpaceRange: wide,
			CIDRanges: []cmap.Range{
				{First: []byte{0, 0, 0, 0}, Last: []byte{0xFF, 0xFF, 0xFF, 0xFF}, Value: 1},
			},
		},
		ToUnicode: &cmap.ToUnicodeFile{
			CodeSpaceRange: wide,
			Ranges: []cmap.ToUnicodeRange{
				{First: []byte{0, 0, 0, 0}, Last: []byte{0xFF, 0xFF, 0xFF, 0xFF}, Values: []string{"a"}},
			},
		},
	}

	done := make(chan struct{})
	go func() {
		for _, text := range []string{" ", "a", "A", "1", "中"} {
			d.GlyphWidth(text)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GlyphWidth did not finish on a hostile CMap")
	}
}
