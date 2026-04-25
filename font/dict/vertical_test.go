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
	"testing"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
)

// makeVerticalCMap returns a CMap that maps single-byte codes 1..255 to the
// CIDs with the same numeric value, in vertical writing mode.
func makeVerticalCMap() *cmap.File {
	return &cmap.File{
		Name:           "Test-V",
		ROS:            &cid.SystemInfo{Registry: "Adobe", Ordering: "Identity", Supplement: 0},
		WMode:          font.Vertical,
		CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0xFF}}},
		CIDRanges: []cmap.Range{
			{First: []byte{0x01}, Last: []byte{0xFF}, Value: 1},
		},
	}
}

// TestVerticalAdvancePopulationDefault verifies that a vertical font without
// per-glyph VMetrics yields the default DeltaY in font.Code.VerticalAdvance.
func TestVerticalAdvancePopulationDefault(t *testing.T) {
	d := &CIDFontType2{
		PostScriptName:  "Test",
		ROS:             &cid.SystemInfo{Registry: "Adobe", Ordering: "Identity", Supplement: 0},
		CMap:            makeVerticalCMap(),
		Width:           map[cid.CID]float64{1: 500},
		DefaultWidth:    1000,
		DefaultVMetrics: DefaultVMetrics{OffsY: 880, DeltaY: -1000},
	}

	f := d.MakeFont()
	for c := range f.Codes(pdf.String{0x01}) {
		if c.Width != 0.5 {
			t.Errorf("Width = %v, want 0.5", c.Width)
		}
		if c.VerticalAdvance != -1 {
			t.Errorf("VerticalAdvance = %v, want -1", c.VerticalAdvance)
		}
	}
}

// TestVerticalAdvancePopulationPerGlyph verifies that VMetrics overrides
// DefaultVMetrics for matching CIDs.
func TestVerticalAdvancePopulationPerGlyph(t *testing.T) {
	d := &CIDFontType2{
		PostScriptName:  "Test",
		ROS:             &cid.SystemInfo{Registry: "Adobe", Ordering: "Identity", Supplement: 0},
		CMap:            makeVerticalCMap(),
		Width:           map[cid.CID]float64{1: 500, 2: 500},
		DefaultWidth:    1000,
		DefaultVMetrics: DefaultVMetricsDefault,
		VMetrics: map[cid.CID]VMetrics{
			2: {DeltaY: -1200, OffsX: 0, OffsY: 880},
		},
	}

	f := d.MakeFont()
	got := make(map[cid.CID]float64)
	for c := range f.Codes(pdf.String{0x01, 0x02}) {
		got[c.CID] = c.VerticalAdvance
	}

	if got[1] != -1 {
		t.Errorf("CID 1 (default) VerticalAdvance = %v, want -1", got[1])
	}
	if got[2] != -1.2 {
		t.Errorf("CID 2 (override) VerticalAdvance = %v, want -1.2", got[2])
	}
}

// TestHorizontalCodeNoVerticalAdvance verifies that horizontal-mode fonts
// leave VerticalAdvance unset (zero) even when VMetrics is configured.
func TestHorizontalCodeNoVerticalAdvance(t *testing.T) {
	cm := makeVerticalCMap()
	cm.WMode = font.Horizontal

	d := &CIDFontType2{
		PostScriptName: "Test",
		ROS:            &cid.SystemInfo{Registry: "Adobe", Ordering: "Identity", Supplement: 0},
		CMap:           cm,
		Width:          map[cid.CID]float64{1: 500},
		DefaultWidth:   1000,
		VMetrics: map[cid.CID]VMetrics{
			1: {DeltaY: -1200, OffsX: 0, OffsY: 880},
		},
	}

	f := d.MakeFont()
	for c := range f.Codes(pdf.String{0x01}) {
		if c.VerticalAdvance != 0 {
			t.Errorf("horizontal-mode VerticalAdvance = %v, want 0", c.VerticalAdvance)
		}
	}
}
