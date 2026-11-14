// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"bytes"
	"math/rand/v2"
	"testing"

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

	// only the last rune is incremented, so the text must keep the prefix
	tu = &ToUnicodeFile{
		Ranges: []ToUnicodeRange{
			{First: []byte{0x10}, Last: []byte{0x12}, Values: []string{"ax"}},
		},
	}
	if code, ok := tu.CodeForText("by"); ok {
		t.Errorf("CodeForText(by) = % x, true; want false", code)
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

	code, ok := tu.CodeForText("x")
	// smallest code among all matches is the last range's (code 1)
	if !ok || !bytes.Equal(code, []byte{0x00, 0x01}) {
		t.Errorf("CodeForText(x) = % x, %v; want 00 01, true", code, ok)
	}
}

// TestCodeForTextWideRange checks that a code deep inside a wide range is
// found.  Enumerating the codes of the range would take time proportional to
// the position of the code.
func TestCodeForTextWideRange(t *testing.T) {
	tu := &ToUnicodeFile{
		Ranges: []ToUnicodeRange{
			{
				First:  []byte{0, 0, 0},
				Last:   []byte{0xFF, 0xFF, 0xFF},
				Values: []string{"a"},
			},
		},
	}

	pos := limits.MaxCMapMappings + 1000
	text, ok := generatedText("a", pos)
	if !ok {
		t.Fatal("test text is not valid")
	}
	code, ok := tu.CodeForText(text)
	want := []byte{byte(pos >> 16), byte(pos >> 8), byte(pos)}
	if !ok || !bytes.Equal(code, want) {
		t.Errorf("CodeForText = % x, %v; want % x, true", code, ok, want)
	}
}

// TestCodeForTextUnmappedPositions checks ranges whose generated text would
// contain a surrogate or a character beyond U+10FFFF.  The range does not
// map these codes, and mappings of lower precedence apply to them.
func TestCodeForTextUnmappedPositions(t *testing.T) {
	cases := []struct {
		name string
		tu   *ToUnicodeFile
		text string
		want []byte
	}{
		{
			name: "after the surrogates",
			tu: &ToUnicodeFile{Ranges: []ToUnicodeRange{
				{First: []byte{0x00, 0x00}, Last: []byte{0xFF, 0xFF}, Values: []string{"\uD7FE"}},
			}},
			text: "\uE000",
			want: []byte{0x08, 0x02},
		},
		{
			name: "U+FFFD itself",
			tu: &ToUnicodeFile{Ranges: []ToUnicodeRange{
				{First: []byte{0x00}, Last: []byte{0xFF}, Values: []string{"\uFFFB"}},
			}},
			text: "\uFFFD",
			want: []byte{0x02},
		},
		{
			name: "beyond Unicode",
			tu: &ToUnicodeFile{Ranges: []ToUnicodeRange{
				{First: []byte{0x00}, Last: []byte{0xFF}, Values: []string{"\U0010FFFE"}},
			}},
			text: "\uFFFD",
			want: nil,
		},
		{
			name: "parent mapping beyond Unicode",
			tu: &ToUnicodeFile{
				Ranges: []ToUnicodeRange{
					{First: []byte{0x00}, Last: []byte{0xFF}, Values: []string{"\U0010FFFF"}},
				},
				Parent: &ToUnicodeFile{Singles: []ToUnicodeSingle{
					{Code: []byte{0x05}, Value: "\uFFFD"},
				}},
			},
			text: "\uFFFD",
			want: []byte{0x05},
		},
		{
			name: "empty first value",
			tu: &ToUnicodeFile{Ranges: []ToUnicodeRange{
				{First: []byte{0x00, 0x00}, Last: []byte{0xFF, 0xFF}, Values: []string{""}},
			}},
			text: "",
			want: []byte{0x00, 0x00},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, ok := c.tu.CodeForText(c.text)
			if !bytes.Equal(code, c.want) || ok != (c.want != nil) {
				t.Errorf("CodeForText(%q) = % x, %v; want % x", c.text, code, ok, c.want)
			}
		})
	}
}

// TestCodeForTextBruteForce compares CodeForText with an exhaustive search
// over all one- and two-byte codes, for random ToUnicode CMaps.
func TestCodeForTextBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	bases := []rune{'a', 'z', 0xD7F0, 0xFFF8, 0x10FFF0}

	randCode := func() []byte {
		code := make([]byte, 1+rng.IntN(2))
		for i := range code {
			code[i] = byte(rng.IntN(8)) * 0x20
		}
		return code
	}
	randText := func() string {
		if rng.IntN(8) == 0 {
			return ""
		}
		return string(bases[rng.IntN(len(bases))] + rune(rng.IntN(16)))
	}
	randFile := func() *ToUnicodeFile {
		tu := &ToUnicodeFile{}
		for range rng.IntN(4) {
			tu.Singles = append(tu.Singles, ToUnicodeSingle{Code: randCode(), Value: randText()})
		}
		for range rng.IntN(4) {
			first := randCode()
			last := bytes.Clone(first)
			for i := range last {
				last[i] += byte(rng.IntN(0x40))
			}
			var values []string
			for range 1 + rng.IntN(3) {
				values = append(values, randText())
			}
			tu.Ranges = append(tu.Ranges, ToUnicodeRange{First: first, Last: last, Values: values})
		}
		return tu
	}

	for range 100 {
		tu := randFile()
		if rng.IntN(2) == 0 {
			tu.Parent = randFile()
		}

		want := make(map[string][]byte)
		for n := 1; n <= 2; n++ {
			code := make([]byte, n)
			for c := range 1 << (8 * n) {
				for i := range code {
					code[i] = byte(c >> (8 * (n - 1 - i)))
				}
				text, ok := tu.Lookup(code)
				if _, seen := want[text]; ok && !seen {
					want[text] = bytes.Clone(code)
				}
			}
		}

		targets := []string{"", "\uFFFD", "q"}
		for text := range want {
			targets = append(targets, text)
		}
		for _, text := range targets {
			code, ok := tu.CodeForText(text)
			if !bytes.Equal(code, want[text]) || ok != (want[text] != nil) {
				t.Fatalf("CodeForText(%q) = % x, %v; want % x\n%v",
					text, code, ok, want[text], tu)
			}
		}
	}
}
