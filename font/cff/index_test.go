package cff

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf/font/parser"
)

func TestIndex(t *testing.T) {
	blob := make([]byte, 1+127)
	for i := range blob {
		blob[i] = byte(i + 1)
	}

	for _, count := range []int{0, 2, 3, 517} {
		data := make([][]byte, count)
		for i := 0; i < count; i++ {
			d := i % 2
			data[i] = blob[d : d+127]
		}

		buf := &bytes.Buffer{}
		n, err := writeIndex(buf, data)
		if err != nil {
			t.Error(err)
			continue
		}
		if n != buf.Len() {
			t.Errorf("wrong output size for count=%d: %d != %d",
				count, n, buf.Len())
		}

		if count == 0 && n != 2 {
			t.Error("wrong length for empty INDEX")
		}

		r := bytes.NewReader(buf.Bytes())
		p := parser.New(r)
		err = p.SetRegion("CFF", 0, int64(n))
		if err != nil {
			t.Fatal(err)
		}

		out, err := readIndex(p)
		if err != nil {
			t.Error(err)
			continue
		}
		if len(out) != len(data) {
			t.Errorf("wrong length")
			continue
		}
		for i, blob := range out {
			if !bytes.Equal(blob, data[i]) {
				t.Errorf("wrong data")
				continue
			}
		}
	}
}
