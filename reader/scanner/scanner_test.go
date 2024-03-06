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
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
)

func TestScanner(t *testing.T) {
	type testOutput struct {
		Op   string
		Args []pdf.Object
	}
	type testCase struct {
		in       string
		expected []testOutput
	}

	testCases := []testCase{
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

	for testNo, tc := range testCases {
		var actual []testOutput

		s := NewScanner()
		err := s.Scan(strings.NewReader(tc.in))(func(op string, args []pdf.Object) error {
			actual = append(actual, testOutput{op, slices.Clone(args)})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		if d := cmp.Diff(tc.expected, actual); d != "" {
			t.Errorf("%d: unexpected output (-want +got):\n%s", testNo, d)
		}
	}
}
