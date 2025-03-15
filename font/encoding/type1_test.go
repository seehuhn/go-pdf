// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package encoding

import (
	"fmt"
	"testing"

	"seehuhn.de/go/pdf"
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
			enc, err := ExtractType1(r, c.encoding, c.nonSymbolicExt)
			if err != nil {
				t.Fatal(err)
			}
			if enc == nil {
				t.Fatal("expected encoding, got nil")
			}
			for _, m := range c.mappings {
				if got := enc(m.code); got != m.value {
					t.Errorf("encoding mismatch at %d: got %q, want %q", m.code, got, m.value)
				}
			}
		})
	}
}

func TestType1Roundtrip(t *testing.T) {
	var cases = []Simple{
		Builtin,
		WinAnsi,
		MacRoman,
		MacExpert,
		Standard,
		func(code byte) string {
			switch code {
			case 30:
				return "Gandalf"
			case 31:
				return "Elrond"
			case 32:
				return "Galadriel"
			case 100:
				return "Gimli"
			case 101:
				return "Frodo"
			case 102:
				return "Sam"
			default:
				return WinAnsi(code)
			}
		},
		func(code byte) string {
			switch code {
			case 0:
				return "Gandalf"
			case 2:
				return "Elrond"
			case 4:
				return "Galadriel"
			case 126:
				return "Gimli"
			case 128:
				return "Frodo"
			case 130:
				return "Sam"
			default:
				return Builtin(code)
			}
		},
	}
	for i, enc1 := range cases {
		for _, nonSymbolicExt := range []bool{true, false} {
			t.Run(fmt.Sprintf("%d/%v", i, nonSymbolicExt), func(t *testing.T) {
				obj, err := enc1.AsPDFType1(nonSymbolicExt, 0)
				if err == errInvalidEncoding {
					t.Skip("encoding cannot be represented as PDF object")
				}
				if err != nil {
					t.Fatal(err)
				}

				enc2, err := ExtractType1(&MockGetter{}, obj, nonSymbolicExt)
				if err != nil {
					t.Fatal(err)
				}

				for code := range 256 {
					if got, want := enc1(byte(code)), enc2(byte(code)); got != want {
						t.Errorf("encoding mismatch at %d: got %q, want %q", code, got, want)
						break
					}
				}
			})
		}
	}
}
