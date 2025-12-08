// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package pdf

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var (
	_ Native = Array{}
	_ Native = Boolean(true)
	_ Native = Dict{}
	_ Native = Integer(0)
	_ Native = Name("name")
	_ Native = Real(0)
	_ Native = Reference(0)
	_ Native = (*Stream)(nil)
	_ Native = String(nil)
	_ Native = (*Placeholder)(nil)
)

func TestFormat(t *testing.T) {
	cases := []struct {
		in  Object
		out string
	}{
		{nil, "null"},
		{String("a"), "(a)"},
		{String("a (test version)"), "(a (test version))"},
		{String("a (test version"), "(a \\(test version)"},
		{String(""), "()"},
		{Array{Integer(1), nil, Integer(3)}, "[1 null 3]"},
	}
	for _, test := range cases {
		out := AsString(test.in)
		if out != test.out {
			t.Errorf("string wrongly formatted, expected %q but got %q",
				test.out, out)
		}
	}
}

func TestStringParse(t *testing.T) {
	type testCase struct {
		in  string
		out String
	}
	cases := []testCase{
		{`()`, String(nil)},
		{"(test string)", String("test string")},
		{`(hello)`, String("hello")},
		{`(he(ll)o)`, String("he(ll)o")},
		{`(he\)ll\(o)`, String("he)ll(o")},
		{"(hello\n)", String("hello\n")},
		{"(hello\r)", String("hello\r")},
		{"(hello\r\n)", String("hello\r\n")},
		{"(hello\n\r)", String("hello\n\r")},
		{"(hell\\\no)", String("hello")},
		{"(hell\\\ro)", String("hello")},
		{"(hell\\\r\no)", String("hello")},
		{`(h\145llo)`, String("hello")},
		{`(\0612)`, String("12")},
		{"<>", String(nil)},
		{"<68656c6c6f>", String("hello")},
		{"<68656C6C6F>", String("hello")},
		{"<68 65 6C 6C 6F>", String("hello")},
		{"<68656C70>", String("help")},
		{"<68656C7>", String("help")},
	}
	for i, test := range cases {
		out, err := ParseString([]byte(test.in))
		if err != nil {
			t.Errorf("%d %q: %s", i, test.in, err)
		} else if !bytes.Equal(out, test.out) {
			t.Errorf("wrong string: %q != %q", out, test.out)
		}
	}
}

func TestStringFormat(t *testing.T) {
	type testCase struct {
		in  String
		out string
	}
	cases := []testCase{
		{String(nil), "()"},
		{String("test string"), "(test string)"},
		{String("hello"), "(hello)"},
		{String("he(ll)o"), "(he(ll)o)"},
		{String("he((ll)o"), "(he(\\(ll)o)"},
		{String("he)ll(o"), "(he\\)ll\\(o)"},
		{String("hello\n"), "(hello\n)"},
		{String("hello\r"), "(hello\\r)"},
		{String("hello\r\n"), "(hello\\r\\n)"},
		{String("hello\n\r"), "(hello\\n\\r)"},
	}
	buf := &bytes.Buffer{}
	for i, test := range cases {
		buf.Reset()
		err := Format(buf, 0, test.in)
		if err != nil {
			t.Errorf("%d: %q: %s", i, test.in, err)
		} else if buf.String() != test.out {
			t.Errorf("%d: wrong string: %q != %q", i, buf.String(), test.out)
		}
	}
}

func FuzzString(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("ABC"))
	f.Add([]byte("()"))
	f.Add([]byte(")("))
	f.Add([]byte("(((()))"))
	f.Add([]byte("\\\\\\(\\)"))
	f.Add([]byte(""))
	f.Add([]byte{0, 1, 2})
	f.Add([]byte{0xFF, 0x00})
	f.Fuzz(func(t *testing.T, data []byte) {
		s1 := String(data)
		enc := AsString(s1)
		s2, err := ParseString([]byte(enc))
		if err != nil {
			t.Error(err)
		} else if !bytes.Equal(s1, s2) {
			t.Errorf("wrong string: %q -> %q -> %q", s1, enc, s2)
		}
	})
}

func TestDict_NilValue(t *testing.T) {
	d := Dict{
		"good": Name("value"),
		"bad":  nil,
	}
	buf := &bytes.Buffer{}
	err := Format(buf, 0, d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "good") {
		panic("test is broken")
	}
	if strings.Contains(buf.String(), "bad") {
		t.Error("nil entry in dict")
	}
}

func TestDict_SortedKeys(t *testing.T) {
	tests := []struct {
		name string
		d    Dict
		want []Name
	}{
		{
			name: "Empty dictionary",
			d:    Dict{},
			want: []Name{},
		},
		{
			name: "Only Type",
			d: Dict{
				"Type": Integer(1),
			},
			want: []Name{"Type"},
		},
		{
			name: "Only Subtype",
			d: Dict{
				"Subtype": Integer(1),
			},
			want: []Name{"Subtype"},
		},
		{
			name: "Type and Subtype",
			d: Dict{
				"Type":    Integer(1),
				"Subtype": Integer(2),
			},
			want: []Name{"Type", "Subtype"},
		},
		{
			name: "Type, Subtype, and others",
			d: Dict{
				"Type":    Integer(1),
				"Subtype": Integer(2),
				"Z":       Integer(3),
				"A":       Integer(4),
			},
			want: []Name{"Type", "Subtype", "A", "Z"},
		},
		{
			name: "Only others",
			d: Dict{
				"Z": Integer(1),
				"A": Integer(2),
				"M": Integer(3),
			},
			want: []Name{"A", "M", "Z"},
		},
		{
			name: "Mixed case with missing special keys",
			d: Dict{
				"Subtype": Integer(1),
				"Z":       Integer(2),
				"A":       Integer(3),
			},
			want: []Name{"Subtype", "A", "Z"},
		},
		{
			name: "Case sensitivity test",
			d: Dict{
				"type":    Integer(1),
				"Type":    Integer(2),
				"subtype": Integer(3),
				"Subtype": Integer(4),
				"a":       Integer(5),
				"A":       Integer(6),
			},
			want: []Name{"Type", "Subtype", "A", "a", "subtype", "type"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.d.SortedKeys()
			if !slices.Equal(got, tt.want) {
				t.Errorf("Dict.SortedKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStreamRead(t *testing.T) {
	dataIn := "\nbinary stream data\000123\n   "
	rIn := strings.NewReader(dataIn)
	stream := &Stream{
		Dict: map[Name]Object{
			"Length": Integer(len(dataIn)),
		},
		R: rIn,
	}
	dataOut, err := ReadAll(nil, stream)
	if err != nil {
		t.Fatal(err)
	}
	if string(dataOut) != dataIn {
		t.Errorf("wrong result:\n  %q\n  %q", dataIn, dataOut)
	}
}

func TestPlaceholder(t *testing.T) {
	const testVal = 12345

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.pdf")

	w, err := Create(tmpFile, V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = w.Alloc() // pretend we have pages

	length := NewPlaceholder(w, 5)
	testRef := w.Alloc()
	err = w.Put(testRef, Dict{
		"Test":   Boolean(true),
		"Length": length,
	})
	if err != nil {
		t.Fatal(err)
	}

	if length.ref != 0 {
		t.Error("failed to detect that file is seekable")
	}

	err = length.Set(Integer(testVal))
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// try to read back the file

	r, err := Open(tmpFile, nil)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := GetDict(r, testRef)
	if err != nil {
		t.Fatal(err)
	}

	lengthOut, err := GetInteger(r, obj["Length"])
	if err != nil {
		t.Fatal(err)
	}

	if lengthOut != testVal {
		t.Errorf("wrong /Length: %d vs %d", lengthOut, testVal)
	}
}

func TestEqual(t *testing.T) {
	objects := []Object{
		nil,
		Integer(1),
		Integer(2),
		Real(1.5),
		Real(2.5),
		Boolean(true),
		Boolean(false),
		Name(""),
		Name("B"),
		String(nil),
		String(""),
		String("world"),
		Operator(""),
		Operator("TJ"),
		NewReference(1, 0),
		NewReference(2, 0),
		Array(nil),
		Array{},
		Array{Integer(1)},
		Array{Integer(1), Integer(2)},
		Dict(nil),
		Dict{},
		Dict{"X": Integer(1)},
		Dict{"X": Integer(2)},
		&Stream{},
		&Stream{},
	}

	for i, a := range objects {
		for j, b := range objects {
			got := Equal(a, b)
			want := i == j
			if got != want {
				t.Errorf("Equal(objects[%d], objects[%d]) = %t, want %t", i, j, got, want)
			}
		}
	}
}

func TestNearlyEqual(t *testing.T) {
	eps := 1e-6

	// nil vs empty are equal for NearlyEqual
	if !NearlyEqual(String(nil), String(""), eps) {
		t.Error("NearlyEqual: String(nil) should equal String(\"\")")
	}
	if !NearlyEqual(Array(nil), Array{}, eps) {
		t.Error("NearlyEqual: Array(nil) should equal Array{}")
	}
	if !NearlyEqual(Dict(nil), Dict{}, eps) {
		t.Error("NearlyEqual: Dict(nil) should equal Dict{}")
	}

	// numeric tolerance
	if !NearlyEqual(Real(3.0), Real(3.0+1e-9), eps) {
		t.Error("NearlyEqual: Real(3.0) should equal Real(3.0+1e-9)")
	}
	if !NearlyEqual(Integer(3), Real(3.0), eps) {
		t.Error("NearlyEqual: Integer(3) should equal Real(3.0)")
	}
	if !NearlyEqual(Real(3.0), Integer(3), eps) {
		t.Error("NearlyEqual: Real(3.0) should equal Integer(3)")
	}
	if NearlyEqual(Integer(3), Integer(4), eps) {
		t.Error("NearlyEqual: Integer(3) should not equal Integer(4)")
	}

	// different types still differ
	if NearlyEqual(Integer(1), Name("1"), eps) {
		t.Error("NearlyEqual: Integer(1) should not equal Name(\"1\")")
	}
}
