// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package scanner

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
)

type testOutput struct {
	Op   string
	Args []pdf.Object
}

type testCase struct {
	in       string
	expected []testOutput
}

var testCases = []testCase{
	{"1 2 3", nil},
	{"1 test 2", []testOutput{{"test", []pdf.Object{pdf.Integer(1)}}}},
	{"1 2.3 m",
		[]testOutput{{"m", []pdf.Object{pdf.Integer(1), pdf.Real(2.3)}}}},
	{"1 2 m 3 4 l",
		[]testOutput{
			{"m", []pdf.Object{pdf.Integer(1), pdf.Integer(2)}},
			{"l", []pdf.Object{pdf.Integer(3), pdf.Integer(4)}}}},
	{".1 x", []testOutput{{"x", []pdf.Object{pdf.Real(0.1)}}}},
	{"+.1 x", []testOutput{{"x", []pdf.Object{pdf.Real(0.1)}}}},
	{"-.1 x", []testOutput{{"x", []pdf.Object{pdf.Real(-0.1)}}}},
	{"true test", []testOutput{{"test", []pdf.Object{pdf.Boolean(true)}}}},
	{"false test", []testOutput{{"test", []pdf.Object{pdf.Boolean(false)}}}},
	{"null test", []testOutput{{"test", []pdf.Object{nil}}}},
	{"<< /a 1 /b 2 >> test",
		[]testOutput{
			{"test", []pdf.Object{pdf.Dict{"a": pdf.Integer(1), "b": pdf.Integer(2)}}}},
	},
	{"[ 1 2 3 ] test",
		[]testOutput{
			{"test", []pdf.Object{pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)}}}}},
	{"(hello world) test",
		[]testOutput{
			{"test", []pdf.Object{pdf.String("hello world")}}}},
	{`(\n\r\t\b\f\)\(\\\123) test`,
		[]testOutput{
			{"test", []pdf.Object{pdf.String("\n\r\t\b\f)(\\S")}}}},
	{"<68656c6c6f20776F726C64> test",
		[]testOutput{
			{"test", []pdf.Object{pdf.String("hello world")}}}},
	{"<5> test",
		[]testOutput{
			{"test", []pdf.Object{pdf.String("P")}}}},
	{"/he#6c#6Co test",
		[]testOutput{
			{"test", []pdf.Object{pdf.Name("hello")}}}},
	{"1 % comment\n2 test",
		[]testOutput{
			{"test", []pdf.Object{pdf.Integer(1), pdf.Integer(2)}}}},
	{`' " W*`, []testOutput{{"'", nil}, {"\"", nil}, {"W*", nil}}},
}

func TestScanner(t *testing.T) {
	for testNo, tc := range testCases {
		var actual []testOutput

		s := NewScanner()
		s.SetInput(strings.NewReader(tc.in))
		for s.Scan() {
			op := s.Operator()
			actual = append(actual, testOutput{op.Name, slices.Clone(op.Args)})
		}
		if err := s.Error(); err != nil {
			t.Fatal(err)
		}

		if d := cmp.Diff(tc.expected, actual); d != "" {
			t.Errorf("%d: unexpected output (-want +got):\n%s", testNo, d)
		}
	}
}

// FuzzScanner makes sure that the scanner does not panic on fuzzed input. For
// each input, the scanner is run, the output is written back to a buffer, and
// the buffer is scanned again.  It is an error if the result of the second
// scan differs from the result of the first scan.
func FuzzScanner(f *testing.F) {
	for _, tc := range testCases {
		f.Add(tc.in)
	}
	f.Fuzz(func(t *testing.T, in string) {
		buf := &bytes.Buffer{}

		s := NewScanner()
		s.SetInput(strings.NewReader(in))
		for s.Scan() {
			op := s.Operator()
			for i, arg := range op.Args {
				if i > 0 {
					buf.WriteString(" ")
				}
				err := pdf.Format(buf, pdf.OptContentStream, arg)
				if err != nil {
					t.Fatal(err)
				}
			}
			if len(op.Args) > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(op.Name)
			buf.WriteString("\n")
		}
		if err := s.Error(); err != nil {
			return
		}

		out1 := buf.String()

		buf.Reset()
		s.Reset()
		s.SetInput(strings.NewReader(out1))
		for s.Scan() {
			op := s.Operator()
			for i, arg := range op.Args {
				if i > 0 {
					buf.WriteString(" ")
				}
				err := pdf.Format(buf, pdf.OptContentStream, arg)
				if err != nil {
					t.Fatal(err)
				}
			}
			if len(op.Args) > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(op.Name)
			buf.WriteString("\n")
		}
		if err := s.Error(); err != nil {
			t.Fatal(err)
		}
		out2 := buf.String()

		if out1 != out2 {
			t.Fatalf("output differs: %q -> %q -> %q", in, out1, out2)
		}
	})
}
