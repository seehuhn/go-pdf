package truetype

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestWidths(t *testing.T) {
	type A = pdf.Array
	type I = pdf.Integer

	cases := []struct {
		in  []int
		out A
	}{
		// test sequence detection
		{
			in: []int{1, 2, 3, 9, 9, 9, 9, 9, 9, 4, 5, 6},
			out: A{
				I(0), A{I(1), I(2), I(3)},
				I(3), I(8), I(9),
				I(9), A{I(4), I(5), I(6)},
			},
		},
		{
			in:  []int{},
			out: nil,
		},
		{
			in: []int{1, 1, 1, 1, 1, 2, 3, 4},
			out: A{
				I(0), I(4), I(1),
				I(5), A{I(2), I(3), I(4)},
			},
		},
		{
			in: []int{2, 1, 4, 1, 1, 1, 1, 1},
			out: A{
				I(0), A{I(2), I(1), I(4)},
				I(3), I(7), I(1),
			},
		},
		{
			in: []int{1, 1, 1, 1, 1},
			out: A{
				I(0), I(4), I(1),
			},
		},

		// test default widths
		{
			in: []int{1, 0, 2},
			out: A{
				I(0), A{I(1), I(0), I(2)},
			},
		},
		{
			in: []int{0, 1, 0, 2, 0},
			out: A{
				I(1), A{I(1), I(0), I(2)},
			},
		},
		{
			in: []int{1, 0, 0, 2},
			out: A{
				I(0), A{I(1)},
				I(3), A{I(2)},
			},
		},
		{
			in: []int{0, 0, 0, 0, 1, 0, 0, 2, 0, 0, 0},
			out: A{
				I(4), A{I(1)},
				I(7), A{I(2)},
			},
		},
	}
	for i, test := range cases {
		W := encodeWidths(test.in, 0)
		buf := &bytes.Buffer{}
		W.PDF(buf)
		fmt.Println(i, buf.String())
		if !reflect.DeepEqual(W, test.out) {
			t.Error(i, "wrong result "+buf.String())
		}
	}
}
