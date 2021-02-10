package fonts

import (
	"testing"
	"unicode"
)

func TestBuiltinEncodings(t *testing.T) {
	encodings := []Encoding{
		StandardEncoding,
		MacRomanEncoding,
		WinAnsiEncoding,
		PDFDocEncoding,
	}
	for i, enc := range encodings {
		for j := 0; j < 256; j++ {
			c := byte(j)

			r := enc.Decode(c)
			if r == unicode.ReplacementChar {
				continue
			}
			c2, ok := enc.Encode(r)
			if !ok {
				t.Errorf("Encoding failed: %d %d->%04x->xxx", i, c, r)
			} else if c2 != c {
				t.Errorf("Encoding failed: %d %d->%04x->%d", i, c, r, c2)
			}
		}

		for r := rune(0); r < 65536; r++ {
			c, ok := enc.Encode(r)
			if !ok {
				continue
			}
			r2 := enc.Decode(c)
			if r2 == unicode.ReplacementChar {
				t.Errorf("Decoding failed: %d %04x->%d->xxx", i, r, c)
			} else if r2 != r {
				t.Errorf("Decoding failed: %d %04x->%d->%04x", i, r, c, r2)
			}
		}
	}
}
