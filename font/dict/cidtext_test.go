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
	"time"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
)

// testROS has no entry in the built-in registry/ordering text mappings, so the
// only text comes from the ToUnicode CMap.
var cidTextROS = &cid.SystemInfo{Registry: "Test", Ordering: "Random"}

func cidTextCMap() *cmap.File {
	return &cmap.File{
		Name:           "Test",
		ROS:            cidTextROS,
		CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0xFF}}},
		CIDSingles: []cmap.Single{
			{Code: []byte{0x41}, Value: 5},
			{Code: []byte{0x42}, Value: 6},
		},
	}
}

func cidTextToUnicode() *cmap.ToUnicodeFile {
	return &cmap.ToUnicodeFile{
		CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0xFF}}},
		Singles: []cmap.ToUnicodeSingle{
			{Code: []byte{0x41}, Value: "A"},
		},
	}
}

// TestCIDFontType0Text pins the text extracted for each code: code 0x41 has a
// ToUnicode entry, code 0x42 has a CID but no ToUnicode and no registry text.
func TestCIDFontType0Text(t *testing.T) {
	d := &CIDFontType0{
		ROS:             cidTextROS,
		CMap:            cidTextCMap(),
		ToUnicode:       cidTextToUnicode(),
		DefaultVMetrics: DefaultVMetricsDefault,
	}

	inst := d.MakeFont()
	got := map[byte]string{}
	for code := range inst.Codes(pdf.String{0x41, 0x42}) {
		got[byte(code.CID)] = code.Text
	}
	if got[5] != "A" {
		t.Errorf("code 0x41: got text %q, want %q", got[5], "A")
	}
	if got[6] != "" {
		t.Errorf("code 0x42: got text %q, want empty", got[6])
	}
}

// TestCIDFontType2Text pins the same behavior for CIDFontType2.
func TestCIDFontType2Text(t *testing.T) {
	d := &CIDFontType2{
		ROS:             cidTextROS,
		CMap:            cidTextCMap(),
		ToUnicode:       cidTextToUnicode(),
		DefaultVMetrics: DefaultVMetricsDefault,
	}

	inst := d.MakeFont()
	got := map[byte]string{}
	for code := range inst.Codes(pdf.String{0x41, 0x42}) {
		got[byte(code.CID)] = code.Text
	}
	if got[5] != "A" {
		t.Errorf("code 0x41: got text %q, want %q", got[5], "A")
	}
	if got[6] != "" {
		t.Errorf("code 0x42: got text %q, want empty", got[6])
	}
}

// TestCIDFontType0TextHugeRange checks that building a font and extracting text
// is cheap even when the CMap declares a wide range: text is looked up lazily
// per code rather than materialized for every code in the range.
func TestCIDFontType0TextHugeRange(t *testing.T) {
	cm := &cmap.File{
		Name:           "Evil",
		ROS:            cidTextROS,
		CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0, 0, 0, 0}, High: []byte{0xFF, 0xFF, 0xFF, 0xFF}}},
		CIDRanges: []cmap.Range{
			{First: []byte{0, 0, 0, 0}, Last: []byte{0xFF, 0xFF, 0xFF, 0xFF}, Value: 1},
		},
	}
	d := &CIDFontType0{
		ROS:             cidTextROS,
		CMap:            cm,
		DefaultVMetrics: DefaultVMetricsDefault,
	}

	done := make(chan struct{})
	go func() {
		inst := d.MakeFont()
		for range inst.Codes(pdf.String{0, 0, 0, 0}) {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("MakeFont/Codes did not finish on a wide CMap range")
	}
}
