// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package encoding

import (
	"fmt"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/type1/names"
)

type MockGetter struct{}

func (m *MockGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	return nil, nil
}

func (m *MockGetter) GetMeta() *pdf.MetaInfo {
	return nil
}

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
	r := &MockGetter{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			enc, err := ExtractSimple(pdf.NewCursor(r), c.encoding, c.nonSymbolicExt)
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

				enc2, err := ExtractSimple(pdf.NewCursor(&MockGetter{}), obj, nonSymbolicExt)
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
