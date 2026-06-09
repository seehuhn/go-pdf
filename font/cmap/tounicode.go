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
	"slices"
	"strings"
	"text/template"
	"unicode/utf16"

	"seehuhn.de/go/postscript"
	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/internal/limits"
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

// ExtractToUnicode extracts a ToUnicode CMap from a PDF stream.
// Cycle detection for recursive parent CMap chains is handled by routing
// parent extraction through [pdf.ExtractorGet].
func ExtractToUnicode(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (*ToUnicodeFile, error) {
	resolved, err := x.Resolve(path, obj)
	if err != nil || resolved == nil {
		return nil, err
	}
	stmObj, ok := resolved.(*pdf.Stream)
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing ToUnicode stream, got %T", resolved),
		}
	}

	err = pdf.CheckDictType(x.R, stmObj.Dict, "CMap")
	if err != nil {
		return nil, err
	}

	body, err := pdf.ReadAll(x.R, path, stmObj, limits.MaxCMapBytes)
	if err != nil {
		return nil, err
	}

	res, err := readToUnicode(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	if parent := stmObj.Dict["UseCMap"]; parent != nil {
		parentInfo, err := pdf.ExtractorGet(x, path, parent, ExtractToUnicode)
		if pdf.IsReadError(err) {
			return nil, err
		}
		res.Parent = parentInfo
	}

	return res, nil
}

func readToUnicode(r io.Reader) (*ToUnicodeFile, error) {
	raw, codeMap, err := postscript.ReadCMap(r)
	if err != nil {
		return nil, err
	}

	if tp, _ := raw["CMapType"].(postscript.Integer); !(tp == 0 || tp == 2) {
		return nil, pdf.Errorf("invalid CMapType: %d", tp)
	}

	res := &ToUnicodeFile{}

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

// IsEmpty returns true if the ToUnicodeInfo object does not contain any mappings.
func (tu *ToUnicodeFile) IsEmpty() bool {
	return tu == nil ||
		len(tu.Singles) == 0 && len(tu.Ranges) == 0 && tu.Parent == nil
}

// MakeName returns a unique name for the ToUnicodeInfo object.
//
// TODO(voss): reconsider once
// https://github.com/pdf-association/pdf-issues/issues/344 is resoved.
func (tu *ToUnicodeFile) MakeName() pdf.Name {
	h := md5.New()
	tu.writeBinary(h, 3)
	return pdf.Name(fmt.Sprintf("seehuhn-%x-UTF16", h.Sum(nil)))
}

// writeBinary writes a binary representation of the ToUnicodeInfo object to
// the [hash.Hash] h.  The maxGen parameter limits the number of parent
// references, to avoid infinite recursion.
func (tu *ToUnicodeFile) writeBinary(h hash.Hash, maxGen int) {
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

	writeInt(len(tu.CodeSpaceRange))
	for _, r := range tu.CodeSpaceRange {
		writeBytes(r.Low)
		writeBytes(r.High)
	}

	writeInt(len(tu.Singles))
	for _, s := range tu.Singles {
		writeBytes(s.Code)
		writeRunes(s.Value)
	}

	writeInt(len(tu.Ranges))
	for _, r := range tu.Ranges {
		writeBytes(r.First)
		writeBytes(r.Last)
		writeInt(len(r.Values))
		for _, values := range r.Values {
			writeRunes(values)
		}
	}

	if tu.Parent != nil {
		writeInt(1)
		tu.Parent.writeBinary(h, maxGen-1)
	} else {
		writeInt(0)
	}
}

func (tu *ToUnicodeFile) All(codec *charcode.Codec) iter.Seq2[charcode.Code, string] {
	// collect the chain of ToUnicode files, root first
	var chain []*ToUnicodeFile
	for g := tu; g != nil; g = g.Parent {
		chain = append(chain, g)
	}
	slices.Reverse(chain)

	return func(yield func(charcode.Code, string) bool) {
		// bound total enumeration so a wide range cannot spin or grow a
		// consumer's map disproportionately to the input size
		budget := limits.MaxCMapMappings
		for _, g := range chain {
			for _, r := range g.Ranges {
				if len(r.Values) == 0 {
					continue
				}
				for i, codeBytes := range codesInRange(r.First, r.Last) {
					if budget <= 0 {
						return
					}
					budget--
					code, k, valid := codec.Decode(codeBytes)
					if !valid || k != len(codeBytes) {
						continue
					}

					if i < len(r.Values) {
						if !yield(code, r.Values[i]) {
							return
						}
					} else {
						if !yield(code, nextString(r.Values[0], i)) {
							return
						}
					}
				}
			}
			for _, single := range g.Singles {
				if budget <= 0 {
					return
				}
				budget--
				code, k, valid := codec.Decode(single.Code)
				if !valid || k != len(single.Code) {
					continue
				}
				if !yield(code, single.Value) {
					return
				}
			}
		}
	}
}

// Lookup returns the unicode string for the given character code,
// together with a flag indicating whether the code was present in the
// ToUnicode CMap.
func (tu *ToUnicodeFile) Lookup(code []byte) (string, bool) {
	if tu == nil {
		return "", false
	}

	for _, s := range tu.Singles {
		if bytes.Equal(s.Code, code) {
			return s.Value, true
		}
	}

	for _, r := range tu.Ranges {
		if len(r.Values) == 0 {
			continue
		}
		index, ok := rangeIndex(r.First, r.Last, code)
		if !ok {
			continue
		}
		if index < len(r.Values) {
			return r.Values[index], true
		}
		return nextString(r.Values[0], index), true
	}

	if tu.Parent != nil {
		return tu.Parent.Lookup(code)
	}
	return "", false
}

// CodeForText returns a character code whose text is exactly text.  If several
// codes map to text, the smallest one is returned.  The flag is false if no
// code maps to text.
//
// The search is bounded, so on malformed input some matches may be missed.
func (tu *ToUnicodeFile) CodeForText(text string) ([]byte, bool) {
	var best []byte
	// codes already resolved by a more specific file, honoring the same
	// child-shadows-parent precedence as Lookup
	seen := make(map[string]struct{})

	// bound the total work so a wide range cannot spin
	budget := limits.MaxCMapMappings
	emit := func(code []byte, value string) bool {
		if budget <= 0 {
			return false
		}
		budget--
		key := string(code)
		if _, dup := seen[key]; dup {
			return true
		}
		seen[key] = struct{}{}
		if value == text && (best == nil || codeLess(code, best)) {
			best = bytes.Clone(code)
		}
		return true
	}

	// child first, and within a file singles before ranges, matching Lookup
	for g := tu; g != nil; g = g.Parent {
		for _, single := range g.Singles {
			if !emit(single.Code, single.Value) {
				return best, best != nil
			}
		}
		for _, r := range g.Ranges {
			if len(r.Values) == 0 {
				continue
			}
			for i, codeBytes := range codesInRange(r.First, r.Last) {
				var value string
				if i < len(r.Values) {
					value = r.Values[i]
				} else {
					value = nextString(r.Values[0], i)
				}
				if !emit(codeBytes, value) {
					return best, best != nil
				}
			}
		}
	}
	return best, best != nil
}

// codeLess reports whether character code a sorts before b, shorter codes
// first and then lexicographically.
func codeLess(a, b []byte) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return bytes.Compare(a, b) < 0
}

func (tu *ToUnicodeFile) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	opt := rm.Out().GetOptions()

	// TODO(voss): review this once
	// https://github.com/pdf-association/pdf-issues/issues/462 is resolved.
	dict := pdf.Dict{}
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("CMap")
	}
	if tu.Parent != nil {
		parent, err := rm.Embed(tu.Parent)
		if err != nil {
			return nil, err
		}
		dict["UseCMap"] = parent
	}

	var filters []pdf.Filter
	if !opt.HasAny(pdf.OptPretty) {
		filters = append(filters, pdf.FilterCompress{})
	}

	ref := rm.Alloc()
	stm, err := rm.Out().OpenStream(ref, dict, filters...)
	if err != nil {
		return nil, err
	}
	err = toUnicodeTmplNew.Execute(stm, tu)
	if err != nil {
		return nil, fmt.Errorf("embedding cmap: %w", err)
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
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
var toUnicodeROS = &cid.SystemInfo{
	Registry: "Adobe",
	Ordering: "UCS",
}

const brokenReplacement = "\uFFFD"
