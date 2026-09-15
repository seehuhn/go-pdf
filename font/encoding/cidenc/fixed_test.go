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

package cidenc

import (
	"slices"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/cid"
)

// TestFixedChildWins checks that a code the CMap remaps encodes the child's
// CID and decodes back to it, and that the parent's CID for that code is
// left without a code rather than emitting one which decodes differently.
func TestFixedChildWins(t *testing.T) {
	f, err := cmap.Predefined("UniJIS-UCS2-HW-H")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := NewFromCMap(f, 500)
	if err != nil {
		t.Fatal(err)
	}

	code := []byte{0x00, 'A'}
	child := f.LookupCID(code)
	parent := f.Parent.LookupCID(code)
	if child == parent {
		t.Fatal("test CMap does not remap ASCII")
	}

	c, err := enc.Encode(child, "A", 500)
	if err != nil {
		t.Fatal(err)
	}
	var s pdf.String
	s = enc.Codec().AppendCode(s, c)
	for got := range enc.Codes(s) {
		if got.CID != child {
			t.Errorf("code decodes to CID %d, want %d", got.CID, child)
		}
	}

	if _, err := enc.Encode(parent, "A", 500); err == nil {
		t.Errorf("shadowed CID %d was encoded", parent)
	}
}

// TestFixedSmallestCode checks that a CID which several codes map to is
// always written as the smallest of them.
func TestFixedSmallestCode(t *testing.T) {
	f, err := cmap.Predefined("UniJIS-UCS2-H")
	if err != nil {
		t.Fatal(err)
	}
	codec, err := f.Codec()
	if err != nil {
		t.Fatal(err)
	}

	// find a CID with more than one code
	codes := make(map[cid.CID][]charcode.Code)
	for code, c := range f.All(codec) {
		codes[c] = append(codes[c], code)
	}
	var shared cid.CID
	var want charcode.Code
	for c, cc := range codes {
		if len(cc) > 1 {
			shared, want = c, slices.Min(cc)
			break
		}
	}
	if len(codes[shared]) < 2 {
		t.Fatal("test CMap has no CID with several codes")
	}

	// several encoders are built, since map order varies between runs
	for range 10 {
		enc, err := NewFromCMap(f, 500)
		if err != nil {
			t.Fatal(err)
		}
		got, err := enc.Encode(shared, "x", 500)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("CID %d written as code %d, want %d", shared, got, want)
		}
	}
}
