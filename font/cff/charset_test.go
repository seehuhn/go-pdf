package cff

import (
	"bytes"
	"fmt"
	"testing"

	"seehuhn.de/go/pdf/font/parser"
)

func TestCharsetDecode(t *testing.T) {
	cases := []struct {
		blob   []byte
		nGlyph int
		first  sid
		last   sid
	}{
		{[]byte{0, 0, 1, 0, 3, 0, 15}, 4, 1, 15},
		{[]byte{1, 0, 2, 13}, 15, 2, 2 + 13},
		{[]byte{2, 0, 3, 2, 1}, 1 + 2*256 + 2, 3, 3 + 2*256 + 1},
	}

	for i, test := range cases {
		fmt.Println("test", i)

		p := parser.New(bytes.NewReader(test.blob))
		err := p.SetRegion("CFF", 0, int64(len(test.blob)))
		if err != nil {
			t.Fatal(err)
		}
		names, err := readCharset(p, test.nGlyph)
		if err != nil {
			t.Fatal(err)
		}

		if len(names) != test.nGlyph {
			t.Errorf("expected %d glyphs, got %d", test.nGlyph, len(names))
		}

		out, err := encodeCharset(names)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(out, test.blob) {
			t.Errorf("expected %v, got %v", test.blob, out)
		}
	}
}

func TestCharsetRoundtrip(t *testing.T) {
	n1 := make([]int32, 100)
	for i := range n1 {
		n1[i] = int32(2 * i)
	}

	n2 := make([]int32, 400)
	for i := range n2 {
		sid := int32(i)
		if i >= 300 && i <= 301 {
			sid = 1000 + 2*int32(i)
		} else if i > 300 {
			sid += 10
		}
		n2[i] = sid
	}

	n3 := make([]int32, 1200)
	for i := range n3 {
		sid := int32(i)
		if i == 600 {
			sid = 2000
		} else if i > 600 {
			sid += 10
		}
		n3[i] = sid
	}

	for i, names := range [][]int32{n1, n2, n3} {
		data, err := encodeCharset(names)
		if err != nil {
			t.Error(err)
			continue
		}
		if data[0] != byte(i) {
			t.Errorf("expected format %d, got %d", i, data[0])
		}

		p := parser.New(bytes.NewReader(data))
		err = p.SetRegion("CFF", 0, int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}

		fmt.Printf("% x\n", data)

		out, err := readCharset(p, len(names))
		if err != nil {
			t.Fatal(err)
		}

		if len(out) != len(names) {
			t.Errorf("expected %d glyphs, got %d", len(names), len(out))
		}
		for i, sid := range names {
			if out[i] != sid {
				t.Errorf("expected %d, got %d", sid, out[i])
			}
		}
	}
}
