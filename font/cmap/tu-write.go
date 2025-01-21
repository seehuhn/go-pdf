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

package cmap

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
)

// Embed adds the ToUnicode cmap to a PDF file.
func (info *ToUnicodeOld) Embed(w *pdf.Writer, ref pdf.Reference) error {
	stm, err := w.OpenStream(ref, nil, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	err = info.Write(stm)
	if err != nil {
		return fmt.Errorf("embedding ToUnicode cmap: %w", err)
	}
	err = stm.Close()
	if err != nil {
		return err
	}
	return nil
}

func (info *ToUnicodeOld) Write(w io.Writer) error {
	return toUnicodeTmpl.Execute(w, info)
}

var toUnicodeTmpl = template.Must(template.New("tounicode").Funcs(template.FuncMap{
	"B": func(x []byte) string {
		return fmt.Sprintf("<%02x>", x)
	},
	"SingleChunks": chunks[ToUnicodeSingleOld],
	"Single": func(cs charcode.CodeSpaceRange, s ToUnicodeSingleOld) string {
		var buf []byte
		buf = cs.Append(buf, s.Code)
		val := hexString(string(s.Value))
		return fmt.Sprintf("<%x> %s", buf, val)
	},
	"RangeChunks": chunks[ToUnicodeRangeOld],
	"Range": func(cs charcode.CodeSpaceRange, s ToUnicodeRangeOld) string {
		var first, last []byte
		first = cs.Append(first, s.First)
		last = cs.Append(last, s.Last)
		if len(s.Values) == 1 {
			return fmt.Sprintf("<%x> <%x> %s", first, last, hexString(string(s.Values[0])))
		}
		var repl []string
		for _, v := range s.Values {
			repl = append(repl, hexString(string(v)))
		}
		return fmt.Sprintf("<%x> <%x> [%s]", first, last, strings.Join(repl, " "))
	},
}).Parse(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapName /Adobe-Identity-UCS def
/CMapType 2 def
/CIDSystemInfo <<
/Registry (Adobe)
/Ordering (UCS)
/Supplement 0
>> def

{{with .CS -}}
{{len .}} begincodespacerange
{{range . -}}
{{B .Low}} {{B .High}}
{{end -}}
{{end -}}
endcodespacerange
{{$cs := .CS -}}

{{range SingleChunks .Singles -}}
{{len .}} beginbfchar
{{range . -}}
{{Single $cs .}}
{{end -}}
endbfchar
{{end -}}

{{range RangeChunks .Ranges -}}
{{len .}} beginbfrange
{{range . -}}
{{Range $cs .}}
{{end -}}
endbfrange
{{end -}}

endcmap
CMapName currentdict /CMap defineresource pop
end
end
`))
