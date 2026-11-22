// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package encoding

import (
	"fmt"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/postscript/type1/names"
)

func TestType1Encoding(t *testing.T) {
	type mapping struct {
		code  byte
		value string
	}
	type testCase struct {
		name           string
		encoding       pdf.Object
		nonSymbolicExt bool
		mappings       []mapping
	}
	cases := []testCase{
		{
			name:     "nil encoding",
			encoding: nil,
			mappings: []mapping{
				{code: 0, value: UseBuiltin},
				{code: 1, value: UseBuiltin},
				{code: 255, value: UseBuiltin},
			},
		},
		{
			name:     "MacRomanEncoding",
			encoding: pdf.Name("MacRomanEncoding"),
			mappings: []mapping{
				{code: 0o101, value: "A"},
				{code: 0o256, value: "AE"},
				{code: 0o331, value: "Ydieresis"},
			},
		},
		{
			name:     "WinAnsiEncoding",
			encoding: pdf.Name("WinAnsiEncoding"),
			mappings: []mapping{
				{code: 0o101, value: "A"},
				{code: 0o306, value: "AE"},
				{code: 0o237, value: "Ydieresis"},
			},
		},
		{
			name:     "MacExpertEncoding",
			encoding: pdf.Name("MacExpertEncoding"),
			mappings: []mapping{
				{code: 0o276, value: "AEsmall"},
				{code: 0o207, value: "Aacutesmall"},
				{code: 0o342, value: "zerosuperior"},
			},
		},
		{
			name:           "dict/nil/true",
			encoding:       pdf.Dict{},
			nonSymbolicExt: true,
			mappings: []mapping{ // standard encoding
				{code: 0o101, value: "A"},
				{code: 0o341, value: "AE"},
				{code: 0o331, value: ".notdef"},
			},
		},
		{
			name:           "dict/nil/false",
			encoding:       pdf.Dict{},
			nonSymbolicExt: false,
			mappings: []mapping{ // built-in encoding
				{code: 0o101, value: UseBuiltin},
				{code: 0o341, value: UseBuiltin},
				{code: 0o331, value: UseBuiltin},
			},
		},
		{
			name: "dict/MacRomanEncoding",
			encoding: pdf.Dict{
				"BaseEncoding": pdf.Name("MacRomanEncoding"),
			},
			mappings: []mapping{
				{code: 0o101, value: "A"},
				{code: 0o256, value: "AE"},
				{code: 0o331, value: "Ydieresis"},
			},
		},
		{
			name: "dict/WinAnsiEncoding",
			encoding: pdf.Dict{
				"BaseEncoding": pdf.Name("WinAnsiEncoding"),
			},
			mappings: []mapping{
				{code: 0o101, value: "A"},
				{code: 0o306, value: "AE"},
				{code: 0o237, value: "Ydieresis"},
			},
		},
		{
			name: "dict/MacExpertEncoding",
			encoding: pdf.Dict{
				"BaseEncoding": pdf.Name("MacExpertEncoding"),
			},
			mappings: []mapping{
				{code: 0o276, value: "AEsmall"},
				{code: 0o207, value: "Aacutesmall"},
				{code: 0o342, value: "zerosuperior"},
			},
		},
		{
			name: "differences",
			encoding: pdf.Dict{
				"BaseEncoding": pdf.Name("MacRomanEncoding"),
				"Differences": pdf.Array{
					pdf.Integer(0o101), pdf.Name("B"), pdf.Name("A"),
					pdf.Integer(0o177), pdf.Name("silly"),
				},
			},
			mappings: []mapping{
				{code: 0o101, value: "B"},
				{code: 0o102, value: "A"},
				{code: 0o103, value: "C"},
				{code: 0o177, value: "silly"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			enc, err := ExtractSimple(pdf.NewCursor(mock.Getter), c.encoding, c.nonSymbolicExt)
			if err != nil {
				t.Fatal(err)
			}
			if enc == nil {
				t.Fatal("expected encoding, got nil")
			}
			for _, m := range c.mappings {
				if got, _ := enc(m.code); got != m.value {
					t.Errorf("encoding mismatch at %d: got %q, want %q", m.code, got, m.value)
				}
			}
		})
	}
}

func TestType1RoundTrip(t *testing.T) {
	var cases = []Simple{
		Builtin,
		WinAnsi,
		MacRoman,
		MacExpert,
		Standard,
		func(code byte) (string, bool) {
			switch code {
			case 30:
				return "Gandalf", true
			case 31:
				return "Elrond", true
			case 32:
				return "Galadriel", true
			case 100:
				return "Gimli", true
			case 101:
				return "Frodo", true
			case 102:
				return "Sam", true
			default:
				return WinAnsi(code)
			}
		},
		func(code byte) (string, bool) {
			switch code {
			case 0:
				return "Gandalf", true
			case 2:
				return "Elrond", true
			case 4:
				return "Galadriel", true
			case 126:
				return "Gimli", true
			case 128:
				return "Frodo", true
			case 130:
				return "Sam", true
			default:
				return Builtin(code)
			}
		},
	}
	for i, enc1 := range cases {
		for _, nonSymbolicExt := range []bool{true, false} {
			t.Run(fmt.Sprintf("%d/%v", i, nonSymbolicExt), func(t *testing.T) {
				obj, err := enc1.AsPDFSimple(nonSymbolicExt, 0)
				if err == errInvalidEncoding {
					t.Skip("encoding cannot be represented as PDF object")
				}
				if err != nil {
					t.Fatal(err)
				}

				enc2, err := ExtractSimple(pdf.NewCursor(mock.Getter), obj, nonSymbolicExt)
				if err != nil {
					t.Fatal(err)
				}

				// unmapped codes may gain names implied by the base encoding
				for code := range 256 {
					want, ok1 := enc1(byte(code))
					got, ok2 := enc2(byte(code))
					if ok1 && (got != want || !ok2) {
						t.Errorf("encoding mismatch at %d: got (%q, %t), want (%q, true)", code, got, ok2, want)
						break
					}
				}
			})
		}
	}
}

// TestType1EmptyName checks that AsPDFSimple rejects the empty glyph name.
func TestType1EmptyName(t *testing.T) {
	enc := Simple(func(code byte) (string, bool) {
		if code == 65 {
			return "", true
		}
		return WinAnsi(code)
	})
	for _, baseIsStd := range []bool{true, false} {
		if _, err := enc.AsPDFSimple(baseIsStd, 0); err == nil {
			t.Errorf("baseIsStd=%t: expected error for empty glyph name", baseIsStd)
		}
	}
}

// TestUseBuiltin checks that the UseBuiltin value is not a valid glyph name.
func TestUseBuiltin(t *testing.T) {
	s := names.ToUnicode(UseBuiltin, "")
	if s != "" {
		t.Errorf("UseBuiltin should not be a valid glyph name, got %q", s)
	}
}

// TestType3EmptyName checks that the empty name in a Type 3 Differences
// array is a glyph name, and that the names after it keep their codes.
func TestType3EmptyName(t *testing.T) {
	obj := pdf.Dict{
		"Differences": pdf.Array{pdf.Integer(65), pdf.Name(""), pdf.Name("B")},
	}
	enc, err := ExtractType3(pdf.NewCursor(mock.Getter), obj, false)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		code byte
		name string
		ok   bool
	}{
		{65, "", true},
		{66, "B", true},
		{67, "", false},
	}
	for _, c := range cases {
		if name, ok := enc(c.code); name != c.name || ok != c.ok {
			t.Errorf("code %d: got (%q, %t), want (%q, %t)", c.code, name, ok, c.name, c.ok)
		}
	}
}

// TestType3NoNames checks that a Differences array which names no glyphs
// gives an encoding which maps no codes.
func TestType3NoNames(t *testing.T) {
	obj := pdf.Dict{"Differences": pdf.Array{pdf.Integer(65)}}
	enc, err := ExtractType3(pdf.NewCursor(mock.Getter), obj, false)
	if err != nil {
		t.Fatal(err)
	}
	for code := range 256 {
		if _, ok := enc(byte(code)); ok {
			t.Errorf("code %d is mapped", code)
		}
	}
}

// TestType3RoundTrip checks that a Type 3 encoding read from a dictionary,
// written with AsPDFType3 and read again, maps every code the same way.
func TestType3RoundTrip(t *testing.T) {
	cases := []pdf.Object{
		pdf.Dict{"Differences": pdf.Array{pdf.Integer(65), pdf.Name(""), pdf.Name("B")}},
		pdf.Dict{"Differences": pdf.Array{
			pdf.Integer(0), pdf.Name("a"), pdf.Name("b"),
			pdf.Integer(254), pdf.Name("y"), pdf.Name("z"), pdf.Name("dropped"),
		}},
		pdf.Dict{"Differences": pdf.Array{pdf.Integer(65)}},

		// malformed: Encoding and Differences are required for Type 3 fonts
		nil,
		pdf.Dict{},
		pdf.Dict{"Differences": pdf.Integer(1)},
		pdf.Name("WinAnsiEncoding"),
		pdf.Array{pdf.Integer(65), pdf.Name("A")},
		pdf.Dict{
			"Type":        pdf.Name("Font"),
			"Differences": pdf.Array{pdf.Integer(65), pdf.Name("A")},
		},
	}
	for _, obj := range cases {
		c := pdf.NewCursor(mock.Getter)
		enc1, err := ExtractType3(c, obj, false)
		if err != nil {
			t.Fatal(err)
		}
		written, err := enc1.AsPDFType3(0)
		if err != nil {
			t.Fatal(err)
		}
		enc2, err := ExtractType3(c, written, false)
		if err != nil {
			t.Fatal(err)
		}
		for code := range 256 {
			name1, ok1 := enc1(byte(code))
			name2, ok2 := enc2(byte(code))
			if name1 != name2 || ok1 != ok2 {
				t.Errorf("%v: code %d: got (%q, %t), want (%q, %t)",
					obj, code, name2, ok2, name1, ok1)
			}
		}
	}
}

// TestSimpleMalformedRoundTrip checks that a malformed /Encoding entry of a
// simple font can be read, and that a read-write-read cycle via AsPDFSimple
// maps every code the same way.
func TestSimpleMalformedRoundTrip(t *testing.T) {
	cases := []pdf.Object{
		pdf.Dict{
			"Type":        pdf.Name("Font"),
			"Differences": pdf.Array{pdf.Integer(65), pdf.Name("B")},
		},
		pdf.Dict{
			"Type":         pdf.Name("Font"),
			"BaseEncoding": pdf.Name("WinAnsiEncoding"),
		},
		pdf.Name("NoSuchEncoding"),
		pdf.Array{pdf.Integer(65), pdf.Name("B")},
		pdf.Dict{"Differences": pdf.Integer(1)},
	}
	for _, obj := range cases {
		for _, nonSymbolicExt := range []bool{false, true} {
			c := pdf.NewCursor(mock.Getter)
			enc1, err := ExtractSimple(c, obj, nonSymbolicExt)
			if err != nil {
				t.Fatal(err)
			}
			written, err := enc1.AsPDFSimple(nonSymbolicExt, 0)
			if err != nil {
				t.Fatal(err)
			}
			enc2, err := ExtractSimple(c, written, nonSymbolicExt)
			if err != nil {
				t.Fatal(err)
			}
			for code := range 256 {
				name1, ok1 := enc1(byte(code))
				name2, ok2 := enc2(byte(code))
				if name1 != name2 || ok1 != ok2 {
					t.Errorf("%v, %t: code %d: got (%q, %t), want (%q, %t)",
						obj, nonSymbolicExt, code, name2, ok2, name1, ok1)
				}
			}
		}
	}
}

// TestType3WriteEmpty checks that an encoding which maps no codes is written
// with an empty, non-null Differences array.
func TestType3WriteEmpty(t *testing.T) {
	var enc Simple = func(byte) (string, bool) { return "", false }
	obj, err := enc.AsPDFType3(0)
	if err != nil {
		t.Fatal(err)
	}
	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("got %T, want pdf.Dict", obj)
	}
	diff, ok := dict["Differences"].(pdf.Array)
	if !ok || diff == nil || len(diff) != 0 {
		t.Errorf("got Differences %#v, want empty array", dict["Differences"])
	}
}
