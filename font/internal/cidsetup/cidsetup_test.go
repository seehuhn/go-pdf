// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cidsetup

import (
	"bytes"
	"math"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/parser"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func predefined(t *testing.T, name string) *cmap.File {
	t.Helper()
	f, err := cmap.Predefined(name)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestFromCMap(t *testing.T) {
	info := makefont.TrueType()

	cases := []struct {
		name  string
		f     *cmap.File
		wMode font.WritingMode
		// ros is the collection the CIDs belong to, nil for free CIDs
		ros *cid.SystemInfo
	}{
		{"Identity-H", cmap.IdentityH, font.Horizontal, nil},
		{"Identity-V", predefined(t, "Identity-V"), font.Vertical, nil},
		{"UTF8H", cmap.UTF8H, font.Horizontal, nil},
		{"UTF8V", cmap.UTF8V, font.Vertical, nil},
		{"UniJIS-UCS2-H", predefined(t, "UniJIS-UCS2-H"), font.Horizontal,
			&cid.SystemInfo{Registry: "Adobe", Ordering: "Japan1", Supplement: 4}},
		{"UniJIS-UCS2-HW-V", predefined(t, "UniJIS-UCS2-HW-V"), font.Vertical,
			&cid.SystemInfo{Registry: "Adobe", Ordering: "Japan1", Supplement: 4}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gidToCID, enc, err := FromCMap(c.f, nil, info)
			if err != nil {
				t.Fatal(err)
			}
			if got := enc.WritingMode(); got != c.wMode {
				t.Errorf("writing mode %v, want %v", got, c.wMode)
			}
			ros := gidToCID.ROS()
			if c.ros == nil {
				// free CIDs belong to a collection of the font's own
				if ros.Registry == "Adobe" {
					t.Errorf("CIDs belong to %v, want a collection of the font's own", ros)
				}
			} else if ros.Registry != c.ros.Registry || ros.Ordering != c.ros.Ordering {
				t.Errorf("CIDs belong to %v, want %v", ros, c.ros)
			}
		})
	}
}

func TestFromCMapErrors(t *testing.T) {
	info := makefont.TrueType()

	cases := []struct {
		name string
		f    *cmap.File
	}{
		{"template with UCS-2 code space", &cmap.File{CodeSpaceRange: charcode.UCS2}},
		{"no CIDSystemInfo", &cmap.File{
			CodeSpaceRange: charcode.UCS2,
			CIDRanges:      []cmap.Range{{First: []byte{0, 0}, Last: []byte{0xFF, 0xFF}}},
		}},
		{"unknown collection", &cmap.File{
			ROS:            &cid.SystemInfo{Registry: "Test", Ordering: "Unknown"},
			CodeSpaceRange: charcode.UCS2,
			CIDRanges:      []cmap.Range{{First: []byte{0, 0}, Last: []byte{0xFF, 0xFF}}},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := FromCMap(c.f, nil, info); err == nil {
				t.Error("no error")
			}
		})
	}
}

// privateCMap returns a CMap over a character collection the library knows
// nothing about, with a 1-byte code space mapping 'A'..'Z' to CIDs 100..125.
func privateCMap() *cmap.File {
	return &cmap.File{
		Name:           "Test-Private-H",
		ROS:            &cid.SystemInfo{Registry: "Test", Ordering: "Private"},
		CodeSpaceRange: charcode.Simple,
		CIDRanges:      []cmap.Range{{First: []byte{'A'}, Last: []byte{'Z'}, Value: 100}},
	}
}

func TestFromCMapGIDToCID(t *testing.T) {
	info := makefont.TrueType()
	private := cmap.NewGIDToCIDFromMap(&cid.SystemInfo{Registry: "Test", Ordering: "Private"}, nil)
	japan := cmap.NewGIDToCIDFromMap(&cid.SystemInfo{Registry: "Adobe", Ordering: "Japan1"}, nil)
	own := cmap.NewGIDToCIDSequential()

	cases := []struct {
		name     string
		f        *cmap.File
		gidToCID cmap.GIDToCID
		wantErr  bool
	}{
		{"private collection", privateCMap(), private, false},
		{"known collection", predefined(t, "UniJIS-UCS2-H"), japan, false},
		{"Identity-H", predefined(t, "Identity-H"), own, false},
		{"template", cmap.UTF8H, own, false},
		{"wrong collection", predefined(t, "UniJIS-UCS2-H"), private, true},
		{"private collection without mapping", privateCMap(), nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _, err := FromCMap(c.f, c.gidToCID, info)
			if c.wantErr {
				if err == nil {
					t.Error("no error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != c.gidToCID {
				t.Error("the supplied GIDToCID was not used")
			}
		})
	}
}

// TestFromCMapNotdefMetrics checks that CID 0 gets the metrics of glyph 0,
// even though the Japan1 collection implies U+FFFD for it, which the font
// has a glyph for.
func TestFromCMapNotdefMetrics(t *testing.T) {
	info, err := sfnt.Read(bytes.NewReader(goregular.TTF), parser.NewBudget(int64(len(goregular.TTF))))
	if err != nil {
		t.Fatal(err)
	}
	gidToCID, enc, err := FromCMap(predefined(t, "Adobe-Japan1-7"), nil, info)
	if err != nil {
		t.Fatal(err)
	}
	if gid := gidToCID.GID(0); gid != 0 {
		t.Errorf("CID 0 selects glyph %d", gid)
	}
	wantWidth, wantV := math.Round(info.GlyphWidthPDF(0)), GlyphVMetrics(info, 0)
	if w, v := enc.CID0Metrics(); w != wantWidth || v != wantV {
		t.Errorf("CID 0 has metrics %g %v, want %g %v", w, v, wantWidth, wantV)
	}
}

// TestFromCMapNotdefContract checks that a GIDToCID which does not map CID 0
// and glyph 0 to each other is rejected.
func TestFromCMapNotdefContract(t *testing.T) {
	info := makefont.TrueType()
	ros := &cid.SystemInfo{Registry: "Test", Ordering: "Private"}
	f := &cmap.File{
		Name: "Test-Private-H",
		ROS:  ros,
		CodeSpaceRange: charcode.CodeSpaceRange{
			{Low: []byte{0x00}, High: []byte{0x7F}},
		},
		CIDRanges: []cmap.Range{{First: []byte{0x20}, Last: []byte{0x7E}, Value: 1}},
	}
	cases := []struct {
		name string
		g2c  map[glyph.ID]cid.CID
		bad  bool
	}{
		{"valid", map[glyph.ID]cid.CID{0: 0, 5: 7}, false},
		{"glyph 0 omitted", map[glyph.ID]cid.CID{5: 7}, false},
		{"other glyph is CID 0", map[glyph.ID]cid.CID{5: 0}, true},
		{"glyph 0 is other CID", map[glyph.ID]cid.CID{0: 3}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := cmap.NewGIDToCIDFromMap(ros, c.g2c)
			_, _, err := FromCMap(f, g, info)
			if (err != nil) != c.bad {
				t.Errorf("got error %v, want error: %v", err, c.bad)
			}

			// the same check applies to a CMap without mappings
			_, _, err = FromCMap(cmap.UTF8H, cmap.NewGIDToCIDFromMap(g.ROS(), c.g2c), info)
			if (err != nil) != c.bad {
				t.Errorf("template: got error %v, want error: %v", err, c.bad)
			}
		})
	}
}
