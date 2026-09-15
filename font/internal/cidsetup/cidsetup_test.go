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

package cidsetup

import (
	"testing"

	"seehuhn.de/go/postscript/cid"

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
		{"nil", nil, font.Horizontal, nil},
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
			gidToCID, enc, err := FromCMap(c.f, nil, info, 500)
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
			if _, _, err := FromCMap(c.f, nil, info, 500); err == nil {
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
			got, _, err := FromCMap(c.f, c.gidToCID, info, 500)
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
