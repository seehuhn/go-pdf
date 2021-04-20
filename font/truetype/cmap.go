// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package truetype

// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf
// https://www.adobe.com/content/dam/acom/en/devnet/acrobat/pdfs/5411.ToUnicode.pdf

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"text/template"
	"unicode/utf16"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

type cmapInfo struct {
	Name         string
	Registry     string
	Ordering     string
	Supplement   int
	SkipComments bool
	Chars        []cidChar
	Ranges       []cidRange
}

type cidChar struct {
	Code font.GlyphID
	CID  rune
}

type cidRange struct {
	From, To font.GlyphID
	FromCID  rune
}

func (info *cmapInfo) FillRanges(cmap map[font.GlyphID]rune) {
	var all []font.GlyphID
	for idx := range cmap {
		all = append(all, idx)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i] < all[j]
	})
	if len(all) == 0 {
		panic("empty cmap")
	}

	first := true
	var start, lastIn font.GlyphID
	var lastOut rune
	flush := func() {
		if start < lastIn {
			info.Ranges = append(info.Ranges, cidRange{
				From:    start,
				To:      lastIn,
				FromCID: lastOut - rune(lastIn-start),
			})
		} else {
			info.Chars = append(info.Chars, cidChar{
				Code: lastIn,
				CID:  lastOut,
			})
		}
	}
	for _, r := range all {
		c := cmap[r]
		if first {
			start = r
			lastIn = r
			lastOut = c
			first = false
		} else {
			if r != lastIn+1 || c != lastOut+1 || lastIn%256 == 255 || lastOut%256 == 255 {
				flush()
				start = r
			}
			lastIn = r
			lastOut = c
		}
	}
	flush()
}

func formatPDFString(args ...interface{}) (string, error) {
	var s pdf.String
	for _, arg := range args {
		switch x := arg.(type) {
		case string:
			s = append(s, x...)
		case []byte:
			s = append(s, x...)
		case byte:
			s = append(s, x)
		case rune:
			s = append(s, string(x)...)
		case int:
			s = append(s, strconv.Itoa(x)...)
		default:
			return "", errors.New("invalid argument type for {{PDFString ...}}")
		}
	}
	buf := &bytes.Buffer{}
	err := s.PDF(buf)
	return buf.String(), err
}

func formatPDFName(args ...interface{}) (string, error) {
	var name pdf.Name
	for _, arg := range args {
		switch x := arg.(type) {
		case string:
			name = name + pdf.Name(x)
		default:
			return "", errors.New("invalid argument type for {{PDFName ...}}")
		}
	}
	buf := &bytes.Buffer{}
	err := name.PDF(buf)
	return buf.String(), err
}

func hex(idx font.GlyphID) string {
	return fmt.Sprintf("<%x>", []byte{byte(idx >> 8), byte(idx)})
}

func runehex(r rune) string {
	x := utf16.Encode([]rune{r})
	var buf []byte
	for _, xi := range x {
		buf = append(buf, byte(xi>>8), byte(xi))
	}
	return fmt.Sprintf("<%x>", buf)
}

const chunkSize = 100

func charChunks(x []cidChar) [][]cidChar {
	var res [][]cidChar
	for len(x) >= chunkSize {
		res = append(res, x[:chunkSize])
		x = x[chunkSize:]
	}
	if len(x) > 0 {
		res = append(res, x)
	}
	return res
}

func rangeChunks(x []cidRange) [][]cidRange {
	var res [][]cidRange
	for len(x) >= chunkSize {
		res = append(res, x[:chunkSize])
		x = x[chunkSize:]
	}
	if len(x) > 0 {
		res = append(res, x)
	}
	return res
}

var cMapTmpl = template.Must(template.New("CMap").Funcs(template.FuncMap{
	"PDFString":   formatPDFString,
	"PDFName":     formatPDFName,
	"hex":         hex,
	"runehex":     runehex,
	"charChunks":  charChunks,
	"rangeChunks": rangeChunks,
}).Parse(
	`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo 3 dict dup begin
/Registry {{PDFString .Registry}} def
/Ordering {{PDFString .Ordering}} def
/Supplement {{.Supplement}} def
end def
/CMapName {{PDFName .Name}} def
/CMapType 2 def
/WMode 0 def
1 begincodespacerange
<0000><FFFF>
endcodespacerange
{{range charChunks .Chars -}}
{{len .}} beginbfchar
{{range . -}}
{{hex .Code}} {{runehex .CID}}
{{end -}}
endbfchar
{{end -}}

{{range rangeChunks .Ranges -}}
{{len .}} beginbfrange
{{range . -}}
{{hex .From}}{{hex .To}}{{runehex .FromCID}}
{{end -}}
endbfrange
{{end -}}

endcmap
CMapName currentdict /CMap defineresource pop
end
end
`))
