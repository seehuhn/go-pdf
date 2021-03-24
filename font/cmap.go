package font

// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"text/template"

	"seehuhn.de/go/pdf"
)

func WriteCMap(w io.Writer, name string, baseName string, cmap map[rune]int) error {
	info := &cmapInfo{
		Name:       name,
		Registry:   "JV",
		Ordering:   baseName,
		Supplement: 0,
		Type:       1,
	}
	info.FillRanges(cmap)
	return writeCMap(w, info)
}

type cmapInfo struct {
	Name       string
	Registry   string
	Ordering   string
	Supplement int
	Type       int
	Chars      []cidChar
	Ranges     []cidRange
}

type cidChar struct {
	Code rune
	CID  int
}

type cidRange struct {
	From, To rune
	FromCID  int
}

func (info *cmapInfo) FillRanges(cmap map[rune]int) {
	var all []rune
	for r := range cmap {
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i] < all[j]
	})
	if len(all) == 0 {
		panic("empty cmap")
	}

	first := true
	var start, lastIn rune
	var lastOut int
	flush := func() {
		if start < lastIn {
			info.Ranges = append(info.Ranges, cidRange{
				From:    start,
				To:      lastIn,
				FromCID: lastOut - int(lastIn-start),
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
			if r != lastIn+1 || c != lastOut+1 || r == 0x80 || r == 0x0800 || r == 0x10000 {
				flush()
				start = r
			}
			lastIn = r
			lastOut = c
		}
	}
	flush()
}

func writeCMap(w io.Writer, info *cmapInfo) error {
	return cMapTmpl.Execute(w, info)
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

func hex(r rune) string {
	return fmt.Sprintf("<%x>", []byte(string(r)))
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
	"charChunks":  charChunks,
	"rangeChunks": rangeChunks,
}).Parse(
	`%!PS-Adobe-3.0 Resource-CMap
%%DocumentNeededResources: ProcSet (CIDInit)
%%IncludeResource: ProcSet (CIDInit)
%%BeginResource: CMap {{PDFString .Name}}
%%Title: {{PDFString .Name " " .Registry " " .Ordering " " .Supplement}}
%%Version: 1.0
%%EndComments
/CIDInit /ProcSet findresource begin
10 dict begin
begincmap
/CIDSystemInfo 3 dict dup begin
/Registry {{PDFString .Registry}} def
/Ordering {{PDFString .Ordering}} def
/Supplement {{.Supplement}} def
end def
/CMapName {{PDFName .Name}} def
/CMapVersion 1.0 def
/CMapType {{.Type}} def
/WMode 0 def
4 begincodespacerange
{{hex 0x00}} {{hex 0x7F}}
{{hex 0x80}} {{hex 0x07FF}}
{{hex 0x0800}} {{hex 0xFFFF}}
{{hex 0x10000}} {{hex 0x10FFFF}}
endcodespacerange
{{range charChunks .Chars -}}
{{len .}} begincidchar
{{range . -}}
{{hex .Code}} {{.CID}}
{{end -}}
endcidchar
{{end -}}

{{range rangeChunks .Ranges -}}
{{len .}} begincidrange
{{range . -}}
{{hex .From}} {{hex .To}} {{.FromCID}}
{{end -}}
endcidrange
{{end -}}

endcmap
CMapName currentdict /CMap defineresource pop
end
end
%%EndResource
%%EOF
`))
