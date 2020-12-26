package pdflib

import "testing"

func TestString(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"a", "(a)"},
		{"", "()"},
		{"\000", "<00>"},
	}
	for _, test := range cases {
		out := format(String(test.in))
		if out != test.out {
			t.Errorf("string wrongly formatted, expected %q but got %q",
				test.out, out)
		}
	}
}
