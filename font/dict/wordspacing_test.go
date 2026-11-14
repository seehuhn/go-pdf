// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict

import (
	"testing"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
)

// TestWordSpacingSingleByteOnly checks that word spacing applies to code 32
// only when it is a valid single-byte code.
func TestWordSpacingSingleByteOnly(t *testing.T) {
	ros := &cid.SystemInfo{Registry: "Adobe", Ordering: "Identity", Supplement: 0}
	twoByte := &cmap.File{
		Name:           "Test-H",
		ROS:            ros,
		CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00, 0x00}, High: []byte{0xFF, 0xFF}}},
		CIDRanges:      []cmap.Range{{First: []byte{0x00, 0x00}, Last: []byte{0xFF, 0xFF}, Value: 0}},
	}
	oneByte := &cmap.File{
		Name:           "Test-H",
		ROS:            ros,
		CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0xFF}}},
		CIDRanges:      []cmap.Range{{First: []byte{0x00}, Last: []byte{0xFF}, Value: 0}},
	}

	type maker func(*cmap.File) font.Instance
	makers := map[string]maker{
		"type0": func(cm *cmap.File) font.Instance {
			return (&CIDFontType0{PostScriptName: "Test", ROS: ros, CMap: cm}).MakeFont()
		},
		"type2": func(cm *cmap.File) font.Instance {
			return (&CIDFontType2{PostScriptName: "Test", ROS: ros, CMap: cm}).MakeFont()
		},
	}
	for name, mk := range makers {
		t.Run(name, func(t *testing.T) {
			// a lone trailing 0x20 in a two-byte-only code space is invalid
			for c := range mk(twoByte).Codes(pdf.String{0x00, 0x41, 0x20}) {
				if c.UseWordSpacing {
					t.Errorf("word spacing for invalid code")
				}
			}

			var got bool
			for c := range mk(oneByte).Codes(pdf.String{0x20}) {
				got = c.UseWordSpacing
			}
			if !got {
				t.Error("no word spacing for valid single-byte code 32")
			}
		})
	}
}
