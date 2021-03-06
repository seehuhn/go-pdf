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
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
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
		{String("\000"), "<00>"},
		{Array{Integer(1), nil, Integer(3)}, "[1 null 3]"},
	}
	for _, test := range cases {
		out := format(test.in)
		if out != test.out {
			t.Errorf("string wrongly formatted, expected %q but got %q",
				test.out, out)
		}
	}
}

func TestTextString(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"\000\011\n\f\r",
		"ein Bär",
		"o țesătură",
		"中文",
		"日本語",
	}
	for _, test := range cases {
		enc := TextString(test)
		out := enc.AsTextString()
		if out != test {
			t.Errorf("wrong text: %q != %q", out, test)
		}
	}
}

func TestDateString(t *testing.T) {
	PST := time.FixedZone("PST", -8*60*60)
	cases := []time.Time{
		time.Date(1998, 12, 23, 19, 52, 0, 0, PST),
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2020, 12, 24, 16, 30, 12, 0, time.FixedZone("", 90*60)),
	}
	for _, test := range cases {
		enc := Date(test)
		out, err := enc.AsDate()
		if err != nil {
			t.Error(err)
		} else if !test.Equal(out) {
			fmt.Println(test, string(enc), out)
			t.Errorf("wrong time: %s != %s", out, test)
		}
	}
}

func TestStream(t *testing.T) {
	dataIn := "\nbinary stream data\000123\n   "
	rIn := strings.NewReader(dataIn)
	stream := &Stream{
		Dict: map[Name]Object{
			"Length": Integer(len(dataIn)),
		},
		R: rIn,
	}
	rOut, err := stream.Decode(nil)
	if err != nil {
		t.Fatal(err)
	}
	dataOut, err := io.ReadAll(rOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(dataOut) != dataIn {
		t.Errorf("wrong result:\n  %q\n  %q", dataIn, dataOut)
	}
}

func format(x Object) string {
	buf := &bytes.Buffer{}
	if x == nil {
		buf.WriteString("null")
	} else {
		_ = x.PDF(buf)
	}
	return buf.String()
}
