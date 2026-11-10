// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/cid"
)

func TestIdentity(t *testing.T) {
	cases := []struct {
		cmap  *File
		name  string
		wMode font.WritingMode
	}{
		{IdentityH, "Identity-H", font.Horizontal},
		{IdentityV, "Identity-V", font.Vertical},
	}
	for _, c := range cases {
		if c.cmap == nil {
			t.Fatalf("%s: nil", c.name)
		}
		if c.cmap.Name != c.name {
			t.Errorf("%s: wrong name %q", c.name, c.cmap.Name)
		}
		if c.cmap.WMode != c.wMode {
			t.Errorf("%s: wrong writing mode %v", c.name, c.cmap.WMode)
		}
		p, err := Predefined(c.name)
		if err != nil {
			t.Fatal(err)
		}
		if p != c.cmap {
			t.Errorf("%s: differs from Predefined", c.name)
		}

		twoByte := charcode.CodeSpaceRange{{Low: []byte{0x00, 0x00}, High: []byte{0xFF, 0xFF}}}
		codec, err := c.cmap.Codec()
		if err != nil {
			t.Fatal(err)
		}
		if cs := codec.CodeSpaceRange(); !cs.Equivalent(twoByte) {
			t.Errorf("%s: wrong code space %v", c.name, cs)
		}
		for _, code := range []uint16{0x0000, 0x0001, 0x00FF, 0x0100, 0x1234, 0xFFFF} {
			got := c.cmap.LookupCID([]byte{byte(code >> 8), byte(code)})
			if got != cid.CID(code) {
				t.Errorf("%s: code %04X maps to CID %d", c.name, code, got)
			}
		}
	}
}
