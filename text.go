package pdf

import (
	"unicode/utf16"

	"seehuhn.de/go/pdf/fonts"
)

func isUTF16(s string) bool {
	return len(s) >= 2 && s[0] == 0xFE && s[1] == 0xFF
}

func utf16Decode(s String) string {
	var u []uint16
	for i := 0; i < len(s)-1; i += 2 {
		u = append(u, uint16(s[i])<<8|uint16(s[i+1]))
	}
	return string(utf16.Decode(u))
}

func pdfDocDecode(s String) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 || fonts.PDFDocEncoding.Decode(s[i]) != rune(s[i]) {
			goto Decode
		}
	}
	return string(s)

Decode:
	r := make([]rune, len(s))
	for i := 0; i < len(s); i++ {
		r[i] = fonts.PDFDocEncoding.Decode(s[i])
	}
	return string(r)
}
