package tounicode

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"strconv"
	"strings"
	"unicode/utf16"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/sfnt/glyph"
)

func (info *Info) Write(w io.Writer) error {
	data := &toUnicodeData{}

	panic("not implemented")

	err := toUnicodeTmpl.Execute(w, data)
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

func hex(idx glyph.ID) string {
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
