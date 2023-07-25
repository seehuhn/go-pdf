// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package tounicode

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/template"

	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
)

// Embed writes the ToUnicode CMap as a stream object to the given PDF file.
func (info *Info) Embed(ref pdf.Reference, w pdf.Putter) error {
	cmapStream, err := w.OpenStream(ref, nil, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	err = info.Write(cmapStream)
	if err != nil {
		return err
	}
	err = cmapStream.Close()
	if err != nil {
		return err
	}

	return nil
}

// Write writes the ToUnicode CMap to the given writer.
func (info *Info) Write(w io.Writer) error {
	if info.ROS != nil {
		if !isValidVCString(info.ROS.Registry) {
			return errors.New("invalid registry")
		}
		if !isValidVCString(info.ROS.Ordering) {
			return errors.New("invalid ordering")
		}
	}

	tmpl := template.Must(template.New("CMap").Funcs(template.FuncMap{
		"PDF":          pdf.Format,
		"PDFString":    formatPDFString,
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

func (info *Info) formatCharCode(code type1.CID) (string, error) {
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

func formatText(xx []uint16) string {
	var text []string
	for _, x := range xx {
		text = append(text, fmt.Sprintf("%04X", x))
	}
	return "<" + strings.Join(text, " ") + ">"
}

func (info *Info) formatSingle(s Single) (string, error) {
	code, err := info.formatCharCode(s.Code)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s %s", code, formatText(s.UTF16)), nil
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

	if len(r.UTF16) == 1 {
		return fmt.Sprintf("%s %s %s", a, b, formatText(r.UTF16[0])), nil
	}

	var texts []string
	for _, t := range r.UTF16 {
		texts = append(texts, formatText(t))
	}
	return fmt.Sprintf("%s %s [%s]", a, b, strings.Join(texts, " ")), nil
}

func formatPDFString(s string) (string, error) {
	return pdf.Format(pdf.TextString(s)), nil
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
{{if .Name -}}
/CMapName {{PDF .Name}} def
{{end -}}
/CMapType 2 def
{{if .ROS -}}
/CIDSystemInfo <<
  /Registry {{PDFString .ROS.Registry}}
  /Ordering {{PDFString .ROS.Ordering}}
  /Supplement {{.ROS.Supplement}}
>> def
{{end -}}
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
