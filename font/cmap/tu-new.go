// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"text/template"
	"unicode/utf16"

	"seehuhn.de/go/postscript"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
)

type ToUnicodeInfo struct {
	charcode.CodeSpaceRange

	Singles []ToUnicodeSingle
	Ranges  []ToUnicodeRange

	Parent *ToUnicodeInfo // This corresponds to the UseCMap entry in the PDF spec.
}

// ToUnicodeSingle specifies that character code Code represents the given unicode string.
type ToUnicodeSingle struct {
	Code  []byte
	Value []rune
}

func (s ToUnicodeSingle) String() string {
	return fmt.Sprintf("% 02x: %q", s.Code, s.Value)
}

// ToUnicodeRange describes a range of character codes.
type ToUnicodeRange struct {
	First  []byte
	Last   []byte
	Values [][]rune
}

func (r ToUnicodeRange) String() string {
	ss := make([]string, len(r.Values))
	for i, v := range r.Values {
		ss[i] = string(v)
	}
	return fmt.Sprintf("% 02x-% 02x: %q", r.First, r.Last, ss)
}

// MakeName returns a unique name for the ToUnicodeInfo object.
//
// TODO(voss): reconsider once
// https://github.com/pdf-association/pdf-issues/issues/344 is resoved.
func (info *ToUnicodeInfo) MakeName() pdf.Name {
	h := md5.New()
	info.writeBinary(h, 3)
	return pdf.Name(fmt.Sprintf("GoPDF-%x-UTF16", h.Sum(nil)))
}

func (info *ToUnicodeInfo) writeBinary(h io.Writer, maxGen int) {
	// This function is only used to write to a hash.Hash.
	// Therefore we don't need to check for write errors.

	if maxGen <= 0 {
		return
	}

	var buf [binary.MaxVarintLen64]byte
	writeInt := func(x int) {
		k := binary.PutUvarint(buf[:], uint64(x))
		h.Write(buf[:k])
	}
	writeBytes := func(b []byte) {
		writeInt(len(b))
		h.Write(b)
	}
	writeRunes := func(rr []rune) {
		writeInt(len(rr))
		for _, r := range rr {
			writeInt(int(r))
		}
	}

	writeInt(len(info.CodeSpaceRange))
	for _, r := range info.CodeSpaceRange {
		writeBytes(r.Low)
		writeBytes(r.High)
	}

	writeInt(len(info.Singles))
	for _, s := range info.Singles {
		writeBytes(s.Code)
		writeRunes(s.Value)
	}

	writeInt(len(info.Ranges))
	for _, r := range info.Ranges {
		writeBytes(r.First)
		writeBytes(r.Last)
		writeInt(len(r.Values))
		for _, values := range r.Values {
			writeRunes(values)
		}
	}

	if info.Parent != nil {
		info.Parent.writeBinary(h, maxGen-1)
	}
}

func ExtractToUnicodeNew(r pdf.Getter, obj pdf.Object) (*ToUnicodeInfo, error) {
	cycle := pdf.NewCycleChecker()
	return safeExtractToUnicode(r, cycle, obj)
}

func safeExtractToUnicode(r pdf.Getter, cycle *pdf.CycleChecker, obj pdf.Object) (*ToUnicodeInfo, error) {
	if err := cycle.Check(obj); err != nil {
		return nil, err
	}

	stmObj, err := pdf.GetStream(r, obj)
	if err != nil {
		return nil, err
	}

	err = pdf.CheckDictType(r, stmObj.Dict, "CMap")
	if err != nil {
		return nil, err
	}

	body, err := pdf.DecodeStream(r, stmObj, 0)
	if err != nil {
		return nil, err
	}

	res, err := readToUnicode(body)
	if err != nil {
		return nil, err
	}

	parent := stmObj.Dict["UseCMap"]
	if parent != nil {
		parentInfo, err := safeExtractToUnicode(r, cycle, parent)
		if err != nil && !pdf.IsMalformed(err) {
			return nil, err
		}
		res.Parent = parentInfo
	}

	return res, nil
}

func readToUnicode(r io.Reader) (*ToUnicodeInfo, error) {
	raw, err := postscript.ReadCMap(r)
	if err != nil {
		return nil, err
	}

	if tp, _ := raw["CMapType"].(postscript.Integer); !(tp == 0 || tp == 2) {
		return nil, pdf.Errorf("invalid CMapType: %d", tp)
	}

	res := &ToUnicodeInfo{}

	codeMap := raw["CodeMap"].(*postscript.CMapInfo)

	for _, entry := range codeMap.CodeSpaceRanges {
		if len(entry.Low) != len(entry.High) || len(entry.Low) == 0 {
			continue
		}
		res.CodeSpaceRange = append(res.CodeSpaceRange,
			charcode.Range{Low: entry.Low, High: entry.High})
	}

	for _, entry := range codeMap.BfChars {
		if len(entry.Src) == 0 {
			continue
		}
		rr, _ := toRunes(entry.Dst)
		if rr == nil {
			continue
		}
		res.Singles = append(res.Singles, ToUnicodeSingle{
			Code:  entry.Src,
			Value: rr,
		})
	}
	for _, entry := range codeMap.BfRanges {
		if len(entry.Low) != len(entry.High) || len(entry.Low) == 0 {
			continue
		}

		switch r := entry.Dst.(type) {
		case postscript.String:
			rr, _ := toRunes(r)
			if rr == nil {
				continue
			}
			res.Ranges = append(res.Ranges, ToUnicodeRange{
				First:  entry.Low,
				Last:   entry.High,
				Values: [][]rune{rr},
			})
		case postscript.Array:
			values := make([][]rune, 0, len(r))
			for _, v := range r {
				rr, _ := toRunes(v)
				if rr != nil {
					values = append(values, rr)
				} else {
					values = append(values, brokenReplacement)
				}
			}
			res.Ranges = append(res.Ranges, ToUnicodeRange{
				First:  entry.Low,
				Last:   entry.High,
				Values: values,
			})
		}
	}

	return res, nil
}

func toRunes(obj postscript.Object) ([]rune, error) {
	dst, ok := obj.(postscript.String)
	if !ok || len(dst)%2 != 0 {
		return nil, fmt.Errorf("invalid ToUnicode CMap")
	}
	buf := make([]uint16, 0, len(dst)/2)
	for i := 0; i < len(dst); i += 2 {
		buf = append(buf, uint16(dst[i])<<8|uint16(dst[i+1]))
	}
	return utf16.Decode(buf), nil
}

func (c *ToUnicodeInfo) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	rosRef, _, err := pdf.ResourceManagerEmbed(rm, toUnicodeROS)
	if err != nil {
		return nil, zero, err
	}

	// TODO(voss): review this once
	// https://github.com/pdf-association/pdf-issues/issues/462 is resolved.
	dict := pdf.Dict{
		"Type":          pdf.Name("CMap"),
		"CMapName":      c.MakeName(),
		"CIDSystemInfo": rosRef,
	}
	if c.Parent != nil {
		parent, _, err := pdf.ResourceManagerEmbed(rm, c.Parent)
		if err != nil {
			return nil, zero, err
		}
		dict["UseCMap"] = parent
	}

	ref := rm.Out.Alloc()
	stm, err := rm.Out.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, zero, err
	}
	err = toUnicodeTmplNew.Execute(stm, c)
	if err != nil {
		return nil, zero, fmt.Errorf("embedding cmap: %w", err)
	}
	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

func hexRunes(rr []rune) string {
	val := utf16.Encode(rr)
	if len(val) == 1 {
		return fmt.Sprintf("<%04x>", val[0])
	}

	valStrings := make([]string, len(val))
	for i, v := range val {
		valStrings[i] = fmt.Sprintf("%04x", v)
	}
	return "<" + strings.Join(valStrings, "") + ">"
}

// TODO(voss): once https://github.com/pdf-association/pdf-issues/issues/344
// is resoved, reconsider CIDSystemInfo.
var toUnicodeTmplNew = template.Must(template.New("cmap").Funcs(template.FuncMap{
	"PN": func(s pdf.Name) string {
		x := postscript.Name(string(s))
		return x.PS()
	},
	"B": func(x []byte) string {
		return fmt.Sprintf("<%02x>", x)
	},
	"SingleChunks": chunks[ToUnicodeSingle],
	"Single": func(s ToUnicodeSingle) string {
		val := hexRunes(s.Value)
		return fmt.Sprintf("<%x> %s", s.Code, val)
	},
	"RangeChunks": chunks[ToUnicodeRange],
	"Range": func(r ToUnicodeRange) string {
		if len(r.Values) == 1 {
			return fmt.Sprintf("<%x> <%x> %s", r.First, r.Last, hexRunes(r.Values[0]))
		}
		var repl []string
		for _, v := range r.Values {
			repl = append(repl, hexRunes(v))
		}
		return fmt.Sprintf("<%x> <%x> [%s]", r.First, r.Last, strings.Join(repl, " "))
	},
}).Parse(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
{{if .Parent -}}
{{PN .Parent.MakeName}} usecmap
{{end -}}
/CMapName {{PN .MakeName}} def
/CMapType 2 def
/CIDSystemInfo <</Registry (Adobe) /Ordering (UCS) /Supplement 0>> def

{{with .CodeSpaceRange -}}
{{len .}} begincodespacerange
{{range . -}}
{{B .Low}} {{B .High}}
{{end -}}
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
`))

// TODO(voss): reconsider once
// https://github.com/pdf-association/pdf-issues/issues/344 is resoved.
var toUnicodeROS = &CIDSystemInfo{
	Registry: "Adobe",
	Ordering: "UCS",
}

var brokenReplacement = []rune{0xFFFD}
