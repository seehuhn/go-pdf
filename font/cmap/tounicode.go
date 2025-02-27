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
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"iter"
	"strings"
	"text/template"
	"unicode/utf16"

	"seehuhn.de/go/postscript"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
)

// ToUnicodeFile represents the contents of a ToUnicode CMap.
// Such a CMap maps character codes to unicode strings.
//
// This structure closely resembles the structure of a ToUnicode CMap file.
type ToUnicodeFile struct {
	charcode.CodeSpaceRange

	Singles []ToUnicodeSingle
	Ranges  []ToUnicodeRange

	Parent *ToUnicodeFile
}

// ToUnicodeSingle specifies that character code Code represents the given unicode string.
type ToUnicodeSingle struct {
	Code  []byte
	Value string
}

func (s ToUnicodeSingle) String() string {
	return fmt.Sprintf("% 02x: %q", s.Code, s.Value)
}

// ToUnicodeRange describes a range of character codes.
type ToUnicodeRange struct {
	First  []byte
	Last   []byte
	Values []string
}

func (r ToUnicodeRange) String() string {
	return fmt.Sprintf("% 02x-% 02x: %q", r.First, r.Last, r.Values)
}

// All returns all assigned texts of valid codes within a range.
// The argument codec is used to determine which codes are valid.
func (r ToUnicodeRange) All(codec *charcode.Codec) iter.Seq2[charcode.Code, string] {
	return func(yield func(charcode.Code, string) bool) {
		L := len(r.First)
		if L != len(r.Last) || L == 0 {
			return
		}

		seq := bytes.Clone(r.First)
		offs := 0
		for {
			code, k, ok := codec.Decode(seq)
			if ok && k == len(seq) {
				var s string
				if offs < len(r.Values) {
					s = r.Values[offs]
				} else {
					rr := []rune(r.Values[0])
					rr[len(rr)-1] += rune(offs)
					s = string(rr)
				}
				if !yield(code, s) {
					return
				}
			}
			offs++

			pos := L - 1
			for pos >= 0 {
				if seq[pos] < r.Last[pos] {
					seq[pos]++
					break
				}
				seq[pos] = r.First[pos]
				pos--
			}
			if pos < 0 {
				break
			}
		}
	}
}

// IsEmpty returns true if the ToUnicodeInfo object does not contain any mappings.
func (info *ToUnicodeFile) IsEmpty() bool {
	return info == nil ||
		len(info.Singles) == 0 && len(info.Ranges) == 0 && info.Parent == nil
}

// MakeName returns a unique name for the ToUnicodeInfo object.
//
// TODO(voss): reconsider once
// https://github.com/pdf-association/pdf-issues/issues/344 is resoved.
func (info *ToUnicodeFile) MakeName() pdf.Name {
	h := md5.New()
	info.writeBinary(h, 3)
	return pdf.Name(fmt.Sprintf("seehuhn-%x-UTF16", h.Sum(nil)))
}

// writeBinary writes a binary representation of the ToUnicodeInfo object to
// the [hash.Hash] h.  The maxGen parameter limits the number of parent
// references, to avoid infinite recursion.
func (info *ToUnicodeFile) writeBinary(h hash.Hash, maxGen int) {
	if maxGen <= 0 {
		return
	}

	const magic uint32 = 0x70e54f7a
	binary.Write(h, binary.BigEndian, magic)

	var buf [binary.MaxVarintLen64]byte
	writeInt := func(x int) {
		k := binary.PutUvarint(buf[:], uint64(x))
		h.Write(buf[:k])
	}
	writeBytes := func(b []byte) {
		writeInt(len(b))
		h.Write(b)
	}
	writeRunes := func(s string) {
		rr := []rune(s)
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
		writeInt(1)
		info.Parent.writeBinary(h, maxGen-1)
	} else {
		writeInt(0)
	}
}

// GetSimpleMapping returns a map from one-byte character codes to strings.
func (info *ToUnicodeFile) GetSimpleMapping() map[byte]string {
	res := make(map[byte]string)

	for _, r := range info.Ranges {
		if len(r.First) != 1 || len(r.Last) != 1 || r.First[0] > r.Last[0] {
			continue
		}
		n := int(r.Last[0]) - int(r.First[0]) + 1
		values := r.Values
		switch len(values) {
		case n:
			for i := range n {
				res[r.First[0]+byte(i)] = values[i]
			}
		case 1:
			rr := []rune(values[0])
			if len(rr) == 0 {
				continue
			}
			for i := range n {
				res[r.First[0]+byte(i)] = string(rr)
				rr[len(rr)-1] += 1
			}
		}
	}

	for _, s := range info.Singles {
		if len(s.Code) == 1 {
			res[s.Code[0]] = s.Value
		}
	}

	return res
}

// Lookup returns the unicode string for the given character code,
// together with a flag indicating whether the code was present in the
// ToUnicode CMap.
func (info *ToUnicodeFile) Lookup(code []byte) (string, bool) {
	if info == nil {
		return "", false
	}

	for _, s := range info.Singles {
		if bytes.Equal(s.Code, code) {
			return s.Value, true
		}
	}

rangesLoop:
	for _, r := range info.Ranges {
		if len(r.First) != len(code) || len(r.Last) != len(code) {
			continue
		}

		var index int
		for i, b := range code {
			if b < r.First[i] || b > r.Last[i] {
				continue rangesLoop
			}
			index = index*int(r.Last[i]-r.First[i]+1) + int(b-r.First[i])
		}

		if index < len(r.Values) {
			return r.Values[index], true
		} else {
			rr := []rune(r.Values[0])
			rr[len(rr)-1] += rune(index)
			return string(rr), true
		}
	}

	if info.Parent != nil {
		return info.Parent.Lookup(code)
	}
	return "", false
}

func ExtractToUnicode(r pdf.Getter, obj pdf.Object) (*ToUnicodeFile, error) {
	cycle := pdf.NewCycleChecker()
	return safeExtractToUnicode(r, cycle, obj)
}

func safeExtractToUnicode(r pdf.Getter, cycle *pdf.CycleChecker, obj pdf.Object) (*ToUnicodeFile, error) {
	if err := cycle.Check(obj); err != nil {
		return nil, err
	}

	stmObj, err := pdf.GetStream(r, obj)
	if err != nil || stmObj == nil {
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
		if pdf.IsReadError(err) {
			return nil, err
		}
		res.Parent = parentInfo
	}

	return res, nil
}

func readToUnicode(r io.Reader) (*ToUnicodeFile, error) {
	raw, err := postscript.ReadCMap(r)
	if err != nil {
		return nil, err
	}

	if tp, _ := raw["CMapType"].(postscript.Integer); !(tp == 0 || tp == 2) {
		return nil, pdf.Errorf("invalid CMapType: %d", tp)
	}

	res := &ToUnicodeFile{}

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
		s, err := toString(entry.Dst)
		if err != nil {
			continue
		}
		res.Singles = append(res.Singles, ToUnicodeSingle{
			Code:  entry.Src,
			Value: s,
		})
	}
	for _, entry := range codeMap.BfRanges {
		if len(entry.Low) != len(entry.High) || len(entry.Low) == 0 {
			continue
		}

		switch r := entry.Dst.(type) {
		case postscript.String:
			s, err := toString(r)
			if err != nil {
				continue
			}
			res.Ranges = append(res.Ranges, ToUnicodeRange{
				First:  entry.Low,
				Last:   entry.High,
				Values: []string{s},
			})
		case postscript.Array:
			values := make([]string, 0, len(r))
			for _, v := range r {
				s, err := toString(v)
				if err == nil {
					values = append(values, s)
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

func toString(obj postscript.Object) (string, error) {
	dst, ok := obj.(postscript.String)
	if !ok || len(dst)%2 != 0 {
		return "", fmt.Errorf("invalid ToUnicode CMap")
	}
	buf := make([]uint16, 0, len(dst)/2)
	for i := 0; i < len(dst); i += 2 {
		buf = append(buf, uint16(dst[i])<<8|uint16(dst[i+1]))
	}
	return string(utf16.Decode(buf)), nil
}

func (c *ToUnicodeFile) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	opt := rm.Out.GetOptions()

	// TODO(voss): review this once
	// https://github.com/pdf-association/pdf-issues/issues/462 is resolved.
	dict := pdf.Dict{}
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("CMap")
	}
	if c.Parent != nil {
		parent, _, err := pdf.ResourceManagerEmbed(rm, c.Parent)
		if err != nil {
			return nil, zero, err
		}
		dict["UseCMap"] = parent
	}

	var filters []pdf.Filter
	if !opt.HasAny(pdf.OptPretty) {
		filters = append(filters, pdf.FilterCompress{})
	}

	ref := rm.Out.Alloc()
	stm, err := rm.Out.OpenStream(ref, dict, filters...)
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

func hexString(s string) string {
	val := utf16.Encode([]rune(s))
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
		val := hexString(s.Value)
		return fmt.Sprintf("<%x> %s", s.Code, val)
	},
	"RangeChunks": chunks[ToUnicodeRange],
	"Range": func(r ToUnicodeRange) string {
		if len(r.Values) == 1 {
			return fmt.Sprintf("<%x> <%x> %s", r.First, r.Last, hexString(r.Values[0]))
		}
		var repl []string
		for _, v := range r.Values {
			repl = append(repl, hexString(v))
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
var toUnicodeROS = &font.CIDSystemInfo{
	Registry: "Adobe",
	Ordering: "UCS",
}

var brokenReplacement = "\uFFFD"
