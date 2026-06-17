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

package cmap

import (
	"bytes"
	"testing"
	"time"

	"seehuhn.de/go/pdf/internal/limits"
)

func TestCodeForTextSingles(t *testing.T) {
	tu := &ToUnicodeFile{
		Singles: []ToUnicodeSingle{
			{Code: []byte{0x41}, Value: "A"},
			{Code: []byte{0x05}, Value: "A"},
			{Code: []byte{0x42}, Value: "B"},
		},
	}

	// smallest code wins when several map to the same text
	code, ok := tu.CodeForText("A")
	if !ok || !bytes.Equal(code, []byte{0x05}) {
		t.Errorf("CodeForText(A) = % x, %v; want 05, true", code, ok)
	}

	code, ok = tu.CodeForText("B")
	if !ok || !bytes.Equal(code, []byte{0x42}) {
		t.Errorf("CodeForText(B) = % x, %v; want 42, true", code, ok)
	}

	if _, ok := tu.CodeForText("Z"); ok {
		t.Error("CodeForText(Z) = true; want false")
	}
}

func TestCodeForTextRange(t *testing.T) {
	// base "x" increments the last rune: 0x10->"x", 0x11->"y", 0x12->"z"
	tu := &ToUnicodeFile{
		Ranges: []ToUnicodeRange{
			{First: []byte{0x10}, Last: []byte{0x12}, Values: []string{"x"}},
		},
	}

	code, ok := tu.CodeForText("y")
	if !ok || !bytes.Equal(code, []byte{0x11}) {
		t.Errorf("CodeForText(y) = % x, %v; want 11, true", code, ok)
	}
}

func TestCodeForTextChildShadowsParent(t *testing.T) {
	// the child overrides code 0x20: effectively 0x20 renders "B", not "A"
	parent := &ToUnicodeFile{
		Singles: []ToUnicodeSingle{{Code: []byte{0x20}, Value: "A"}},
	}
	tu := &ToUnicodeFile{
		Singles: []ToUnicodeSingle{{Code: []byte{0x20}, Value: "B"}},
		Parent:  parent,
	}

	// the shadowed parent entry must not be returned for "A"
	if code, ok := tu.CodeForText("A"); ok {
		t.Errorf("CodeForText(A) = % x, true; want false (shadowed)", code)
	}
	// the child's own mapping is returned for "B"
	if code, ok := tu.CodeForText("B"); !ok || !bytes.Equal(code, []byte{0x20}) {
		t.Errorf("CodeForText(B) = % x, %v; want 20, true", code, ok)
	}
}

func TestCodeForTextParentFallback(t *testing.T) {
	// the child does not define 0x30, so the parent's mapping is effective
	parent := &ToUnicodeFile{
		Singles: []ToUnicodeSingle{{Code: []byte{0x30}, Value: "A"}},
	}
	tu := &ToUnicodeFile{
		Singles: []ToUnicodeSingle{{Code: []byte{0x20}, Value: "B"}},
		Parent:  parent,
	}

	if code, ok := tu.CodeForText("A"); !ok || !bytes.Equal(code, []byte{0x30}) {
		t.Errorf("CodeForText(A) = % x, %v; want 30, true", code, ok)
	}
}

func TestCodeForTextShorterPreferred(t *testing.T) {
	tu := &ToUnicodeFile{
		Singles: []ToUnicodeSingle{
			{Code: []byte{0x00, 0x41}, Value: "A"},
			{Code: []byte{0x41}, Value: "A"},
		},
	}
	code, ok := tu.CodeForText("A")
	if !ok || !bytes.Equal(code, []byte{0x41}) {
		t.Errorf("CodeForText(A) = % x, %v; want 41 (shorter), true", code, ok)
	}
}

// TestCodeForTextManyRanges checks that the search stays linear when many
// ranges each map a code to the queried text in descending code order, the
// shape that would make a per-candidate Lookup quadratic.
func TestCodeForTextManyRanges(t *testing.T) {
	const n = 50000
	ranges := make([]ToUnicodeRange, n)
	for i := range ranges {
		// descending codes: range i covers code n-i
		c := n - i
		first := []byte{byte(c >> 8), byte(c)}
		ranges[i] = ToUnicodeRange{First: first, Last: first, Values: []string{"x"}}
	}
	tu := &ToUnicodeFile{Ranges: ranges}

	done := make(chan struct{})
	go func() {
		code, ok := tu.CodeForText("x")
		// smallest code among all matches is the last range's (code 1)
		if !ok || !bytes.Equal(code, []byte{0x00, 0x01}) {
			t.Errorf("CodeForText(x) = % x, %v; want 00 01, true", code, ok)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("CodeForText did not finish promptly with many ranges")
	}
}

// TestCodeForTextBudget checks that the reverse scan in CodeForText stops at
// limits.MaxCMapMappings, independent of how wide the range is. The range's
// values auto-increment from "a", so the code at enumeration index i carries
// the text rune('a'+i); the code a little past the budget therefore has a
// known, unique text that a bounded scan never reaches. A scan that ignored the
// budget would walk the whole range and report it. This is a deterministic
// stand-in for a wall-clock "finishes in time" check, which is sensitive to
// machine load.
func TestCodeForTextBudget(t *testing.T) {
	n := limits.MaxCMapMappings
	// A 3-byte range spans 256^3 ≈ 16.8M codes, far more than the budget. Codes
	// are enumerated last-byte-fastest, so index i is reached after i steps.
	tu := &ToUnicodeFile{
		Ranges: []ToUnicodeRange{
			{
				First:  []byte{0, 0, 0},
				Last:   []byte{0xFF, 0xFF, 0xFF},
				Values: []string{"a"},
			},
		},
	}

	// text of a code just past the budget; n stays well below the Unicode
	// maximum, so the incremented value is a valid, unique rune.
	target := nextString("a", n+1000)
	if _, ok := tu.CodeForText(target); ok {
		t.Errorf("CodeForText found a code past the %d-entry budget", n)
	}
}
