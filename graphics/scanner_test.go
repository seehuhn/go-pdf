package graphics

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
		s.Scan(strings.NewReader(tc.in))(func(op string, args []pdf.Object) bool {
			actual = append(actual, testOutput{op, slices.Clone(args)})
			return true
		})

		if d := cmp.Diff(tc.expected, actual); d != "" {
			t.Errorf("%d: unexpected output (-want +got):\n%s", testNo, d)
		}
	}
}
