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
	"seehuhn.de/go/pdf/font/cmap"
)

func (info *Info) Write(w io.Writer) error {
	tmpl := template.Must(template.New("CMap").Funcs(template.FuncMap{
		"PDF":          formatPDF,
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

func (info *Info) formatCharCode(code cmap.CharCode) (string, error) {
	for _, r := range info.CodeSpace {
		if code >= r.First && code <= r.Last {
			var format string
			if r.Last >= 1<<24 {
				format = "%08X"
			} else if r.Last >= 1<<16 {
				format = "%06X"
			} else if r.Last >= 1<<8 {
				format = "%04X"
			} else {
				format = "%02X"
			}

			return fmt.Sprintf("<"+format+">", code), nil
		}
	}
	return "", errors.New("code not in code space")
}

func formatText(s string) string {
	var text []string
	for _, x := range utf16.Encode([]rune(s)) {
		text = append(text, fmt.Sprintf("%04X", x))
	}
	return "<" + strings.Join(text, " ") + ">"
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

func formatPDF(obj pdf.Object) (string, error) {
	buf := &bytes.Buffer{}
	err := obj.PDF(buf)
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
/CMapName {{PDF .Name}} def
/CMapType 2 def
/CIDSystemInfo <<
  /Registry {{PDF .Registry}}
  /Ordering {{PDF .Ordering}}
  /Supplement {{.Supplement}}
>> def
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
