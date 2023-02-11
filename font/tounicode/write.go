package tounicode

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/template"
	"unicode/utf16"

	"seehuhn.de/go/pdf"
)

func (info *Info) Write(w io.Writer) error {
	tmpl := template.Must(template.New("CMap").Funcs(template.FuncMap{
		"PDFString":    formatPDFString,
		"PDFName":      formatPDFName,
		"SingleChunks": singleChunks,
		"Single":       info.formatSingle,
		"RangeChunks":  rangeChunks,
		"Range":        info.formatRange,
	}).Parse(toUnicodeTmpl))
	err := tmpl.Execute(w, info)
	if err != nil {
		return err
	}

	return nil
}

func (info *Info) formatCharCode(code CharCode) (string, error) {
	for _, r := range info.CodeSpace {
		if code >= r.First && code <= r.Last {
			var format string
			if r.Last >= 1<<24 {
				format = "%08x"
			} else if r.Last >= 1<<16 {
				format = "%06x"
			} else if r.Last >= 1<<8 {
				format = "%04x"
			} else {
				format = "%02x"
			}

			return fmt.Sprintf("<"+format+">", code), nil
		}
	}
	return "", errors.New("code not in code space")
}

func formatText(s string) string {
	var text []byte
	for _, x := range utf16.Encode([]rune(s)) {
		text = append(text, byte(x>>8), byte(x))
	}
	return fmt.Sprintf("<%02X>", text)
}

func (info *Info) formatSingle(s Single) (string, error) {
	code, err := info.formatCharCode(s.Code)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s %s", code, formatText(s.Text)), nil
}

func (info *Info) formatRange(r Range) (string, error) {
	a, err := info.formatCharCode(r.First)
	if err != nil {
		return "", err
	}
	b, err := info.formatCharCode(r.Last)
	if err != nil {
		return "", err
	}

	if len(r.Text) == 1 {
		return fmt.Sprintf("%s %s %s", a, b, formatText(r.Text[0])), nil
	}

	var texts []string
	for _, t := range r.Text {
		texts = append(texts, formatText(t))
	}
	return fmt.Sprintf("%s %s [%s]", a, b, strings.Join(texts, " ")), nil
}

func formatPDFString(s pdf.String) (string, error) {
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

const chunkSize = 100

func singleChunks(x []Single) [][]Single {
	var res [][]Single
	for len(x) >= chunkSize {
		res = append(res, x[:chunkSize])
		x = x[chunkSize:]
	}
	if len(x) > 0 {
		res = append(res, x)
	}
	return res
}

func rangeChunks(x []Range) [][]Range {
	var res [][]Range
	for len(x) >= chunkSize {
		res = append(res, x[:chunkSize])
		x = x[chunkSize:]
	}
	if len(x) > 0 {
		res = append(res, x)
	}
	return res
}

var toUnicodeTmpl = `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapType 2 def
/CMapName {{printf "%s-%s-%03d" .Registry .Ordering .Supplement | PDFName}} def
/CIDSystemInfo <<
/Registry {{PDFString .Registry}} def
/Ordering {{PDFString .Ordering}} def
/Supplement {{.Supplement}} def
>> def
/WMode 0 def
{{len .CodeSpace}} begincodespacerange
{{range .CodeSpace -}}
{{.}}
{{end -}}
endcodespacerange
{{range SingleChunks .Singles -}}
{{len .}} beginbfchar
{{range . -}}
{{Single .}}
{{end -}}
endbfchar
{{end -}}

{{range RangeChunks .Ranges -}}
{{len .}} beginbfrange
{{range . -}}
{{Range .}}
{{end -}}
endbfrange
{{end -}}

endcmap
CMapName currentdict /CMap defineresource pop
end
end
`
