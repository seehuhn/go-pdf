package pdf

import (
	"strings"
	"testing"
)

func TestReaderGoFuzz(t *testing.T) {
	// found by go-fuzz - check that the code doesn't panic
	cases := []string{
		"%PDF-1.0\n0 0obj<startxref8",
		"%PDF-1.0\n0 0obj(startxref8",
		"%PDF-1.0\n0 0obj<</Length -40>>stream\nstartxref8\n",
		"%PDF-1.0\n0 0obj<</ 0 0%startxref8",
	}
	for _, test := range cases {
		buf := strings.NewReader(test)
		_, _ = NewReader(buf, buf.Size(), nil)
	}
}
