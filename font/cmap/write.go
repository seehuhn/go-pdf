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
	"text/template"

	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript"
)

func (info *FileOld) Write(w io.Writer) error {
	return cmapTmpl.Execute(w, info)
}

var cmapTmpl = template.Must(template.New("cmap").Funcs(template.FuncMap{
	"PS": func(s string) string {
		x := postscript.String(s)
		return x.PS()
	},
	"PN": func(s string) string {
		x := postscript.Name(s)
		return x.PS()
	},
	"B": func(x []byte) string {
		return fmt.Sprintf("<%02x>", x)
	},
	"SingleChunks": chunks[SingleOld],
	"Single": func(cs charcode.CodeSpaceRange, s SingleOld) string {
		var buf []byte
		buf = cs.Append(buf, s.Code)
		return fmt.Sprintf("<%x> %d", buf, s.Value)
	},
	"RangeChunks": chunks[RangeOld],
	"Range": func(cs charcode.CodeSpaceRange, s RangeOld) string {
		var first, last []byte
		first = cs.Append(first, s.First)
		last = cs.Append(last, s.Last)
		return fmt.Sprintf("<%x> <%x> %d", first, last, s.Value)
	},
}).Parse(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
{{if .UseCMap -}}
{{PN .UseCMap}} usecmap
{{end -}}

{{if .ROS -}}
/CIDSystemInfo 3 dict dup begin
/Registry {{PS .ROS.Registry}} def
/Ordering {{PS .ROS.Ordering}} def
/Supplement {{.ROS.Supplement}} def
end def
{{end -}}
/CMapName {{PN .Name}} def
/CMapType 1 def
/WMode {{.WMode}} def
{{with .CSFile -}}
{{len .}} begincodespacerange
{{range . -}}
{{B .Low}} {{B .High}}
{{end -}}
{{end -}}
endcodespacerange
{{$cs := .CodeSpaceRange -}}

{{range SingleChunks .Singles -}}
{{len .}} begincidchar
{{range . -}}
{{Single $cs .}}
{{end -}}
endcidchar
{{end -}}

{{range RangeChunks .Ranges -}}
{{len .}} begincidrange
{{range . -}}
{{Range $cs .}}
{{end -}}
endcidrange
{{end -}}

endcmap
CMapName currentdict /CMap defineresource pop
end
end
`))
