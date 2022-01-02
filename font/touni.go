// seehuhn.de/go/pdf - a library for reading and writing PDF files
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

package font

// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf
// https://www.adobe.com/content/dam/acom/en/devnet/acrobat/pdfs/5411.ToUnicode.pdf

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf16"

	"seehuhn.de/go/pdf"
)

// SimpleMapping describes the unicode text corresponding to a character code
// in a simple font.
type SimpleMapping struct {
	CharCode byte
	Text     []rune
}

// WriteToUnicodeSimple writes the ToUnicode stream for a simple font.
// This modifies mm.
func WriteToUnicodeSimple(w *pdf.Writer, ordering string, mm []SimpleMapping, toUnicodeRef *pdf.Reference) error {
	data := &toUnicodeData{
		Registry:   "seehuhn.de",
		Ordering:   ordering,
		Supplement: 0,
		CodeSpace:  []string{"<00><FF>"},
	}

	sort.Slice(mm, func(i, j int) bool { return mm[i].CharCode < mm[j].CharCode })

	canDeltaRange := make([]bool, len(mm))
	step := make([]byte, len(mm))
	var prevDelta int
	var prevCharCode byte
	for i, m := range mm {
		delta := int(m.Text[0]) - int(m.CharCode)
		charCode := m.CharCode
		if i > 0 {
			canDeltaRange[i] = delta == prevDelta && len(m.Text) == 1
			step[i] = charCode - prevCharCode
		}
		prevDelta = delta
		prevCharCode = charCode
	}

	pos := 0
	for pos < len(mm) {
		next := pos + 1
		for next < len(mm) && canDeltaRange[next] {
			next++
		}
		if next > pos+1 {
			bf := bfRange{
				From:     []byte{mm[pos].CharCode},
				To:       []byte{mm[next-1].CharCode},
				FromText: [][]rune{mm[pos].Text},
			}
			data.Ranges = append(data.Ranges, bf)
			pos = next
			continue
		}

		next = pos + 1
		for next < len(mm) && step[next] < 2 {
			next++
		}
		if next > pos+1 {
			var repl [][]rune
			for i := pos; i < next; i++ {
				if i > pos && step[i] > 1 {
					for j := 0; j < int(step[i]-1); j++ {
						repl = append(repl, []rune{'\uFFFD'})
					}
				}
				repl = append(repl, mm[i].Text)
			}
			bf := bfRange{
				From:     []byte{mm[pos].CharCode},
				To:       []byte{mm[next-1].CharCode},
				FromText: repl,
			}
			data.Ranges = append(data.Ranges, bf)
			pos = next
			continue
		}

		data.Chars = append(data.Chars, bfChar{
			Code: []byte{mm[pos].CharCode},
			Text: mm[pos].Text,
		})
		pos++
	}

	return writeToUnicodeStream(w, data, toUnicodeRef)
}

// CIDMapping describes the unicode text corresponding to a character code
// in a CIDFont.
type CIDMapping struct {
	CharCode uint16
	Text     []rune
}

// WriteToUnicodeCID writes the ToUnicode stream for a CIDFont.
// This modifies mm.
func WriteToUnicodeCID(w *pdf.Writer, mm []CIDMapping, toUnicodeRef *pdf.Reference) error {
	data := &toUnicodeData{
		Registry:   "Adobe",
		Ordering:   "UCS",
		Supplement: 0,
		CodeSpace:  []string{"<0000><FFFF>"},
	}

	sort.Slice(mm, func(i, j int) bool { return mm[i].CharCode < mm[j].CharCode })

	canDeltaRange := make([]bool, len(mm))
	step := make([]uint16, len(mm))
	var prevDelta int
	var prevCharCode uint16
	for i, m := range mm {
		delta := int(m.Text[0]) - int(m.CharCode)
		charCode := m.CharCode
		if i > 0 {
			canDeltaRange[i] = delta == prevDelta && len(m.Text) == 1
			step[i] = charCode - prevCharCode
		}
		prevDelta = delta
		prevCharCode = charCode
	}

	pos := 0
	for pos < len(mm) {
		next := pos + 1
		for next < len(mm) && canDeltaRange[next] {
			next++
		}
		if next > pos+1 {
			start := mm[pos].CharCode
			end := mm[next-1].CharCode
			bf := bfRange{
				From:     []byte{byte(start >> 8), byte(start)},
				To:       []byte{byte(end >> 8), byte(end)},
				FromText: [][]rune{mm[pos].Text},
			}
			data.Ranges = append(data.Ranges, bf)
			pos = next
			continue
		}

		next = pos + 1
		for next < len(mm) && step[next] < 2 {
			next++
		}
		if next > pos+1 {
			var repl [][]rune
			for i := pos; i < next; i++ {
				if i > pos && step[i] > 1 {
					for j := 0; j < int(step[i]-1); j++ {
						repl = append(repl, []rune{'\uFFFD'})
					}
				}
				repl = append(repl, mm[i].Text)
			}
			from := mm[pos].CharCode
			to := mm[next-1].CharCode
			bf := bfRange{
				From:     []byte{byte(from >> 8), byte(from)},
				To:       []byte{byte(to >> 8), byte(to)},
				FromText: repl,
			}
			data.Ranges = append(data.Ranges, bf)
			pos = next
			continue
		}

		code := mm[pos].CharCode
		data.Chars = append(data.Chars, bfChar{
			Code: []byte{byte(code >> 8), byte(code)},
			Text: mm[pos].Text,
		})
		pos++
	}

	return writeToUnicodeStream(w, data, toUnicodeRef)
}

func writeToUnicodeStream(w *pdf.Writer, data *toUnicodeData, toUnicodeRef *pdf.Reference) error {
	compress := &pdf.FilterInfo{
		Name:  pdf.Name("LZWDecode"),
		Parms: pdf.Dict{"EarlyChange": pdf.Integer(0)},
	}
	if w.Version >= pdf.V1_2 {
		compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
	}
	cmapStream, _, err := w.OpenStream(nil, toUnicodeRef, compress)
	if err != nil {
		return err
	}
	err = toUnicodeTmpl.Execute(cmapStream, data)
	if err != nil {
		return err
	}
	err = cmapStream.Close()
	if err != nil {
		return err
	}
	return nil
}

type toUnicodeData struct {
	Registry     string
	Ordering     string
	Supplement   int
	SkipComments bool
	CodeSpace    []string
	Chars        []bfChar
	Ranges       []bfRange
}

type bfChar struct {
	Code pdf.String
	Text []rune
}

func (bfc bfChar) String() string {
	var text []byte
	for _, x := range utf16.Encode(bfc.Text) {
		text = append(text, byte(x>>8), byte(x))
	}
	return fmt.Sprintf("<%02X> <%02X>", []byte(bfc.Code), text)
}

type bfRange struct {
	From, To pdf.String
	FromText [][]rune
}

func (bfr bfRange) String() string {
	if len(bfr.FromText) == 1 {
		var text []byte
		for _, x := range utf16.Encode(bfr.FromText[0]) {
			text = append(text, byte(x>>8), byte(x))
		}
		return fmt.Sprintf("<%02X> <%02X> <%02X>",
			[]byte(bfr.From), []byte(bfr.To), text)
	}

	var texts []string
	for _, in := range bfr.FromText {
		var text []byte
		for _, x := range utf16.Encode(in) {
			text = append(text, byte(x>>8), byte(x))
		}
		texts = append(texts, fmt.Sprintf("<%02X>", text))
	}
	repl := strings.Join(texts, " ")
	return fmt.Sprintf("<%02X> <%02X> [%s]",
		[]byte(bfr.From), []byte(bfr.To), repl)
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

func hex(idx GlyphID) string {
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

func charChunks(x []bfChar) [][]bfChar {
	var res [][]bfChar
	for len(x) >= chunkSize {
		res = append(res, x[:chunkSize])
		x = x[chunkSize:]
	}
	if len(x) > 0 {
		res = append(res, x)
	}
	return res
}

func rangeChunks(x []bfRange) [][]bfRange {
	var res [][]bfRange
	for len(x) >= chunkSize {
		res = append(res, x[:chunkSize])
		x = x[chunkSize:]
	}
	if len(x) > 0 {
		res = append(res, x)
	}
	return res
}

var toUnicodeTmpl = template.Must(template.New("CMap").Funcs(template.FuncMap{
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
/CMapName {{printf "%s-%s-%03d" .Registry .Ordering .Supplement | PDFName}} def
/CMapType 2 def
/WMode 0 def
{{len .CodeSpace}} begincodespacerange
{{range .CodeSpace -}}
{{.}}
{{end -}}
endcodespacerange
{{range charChunks .Chars -}}
{{len .}} beginbfchar
{{range . -}}
{{.}}
{{end -}}
endbfchar
{{end -}}

{{range rangeChunks .Ranges -}}
{{len .}} beginbfrange
{{range . -}}
{{.}}
{{end -}}
endbfrange
{{end -}}

endcmap
CMapName currentdict /CMap defineresource pop
end
end
`))
