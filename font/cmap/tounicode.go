// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"iter"
	"math"
	"slices"
	"strings"
	"sync"
	"text/template"
	"unicode/utf16"

	"seehuhn.de/go/postscript"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/internal/limits"
)

// ToUnicodeFile represents the contents of a ToUnicode CMap.
// Such a CMap maps character codes to unicode strings.
//
// This structure closely resembles the structure of a ToUnicode CMap file.
//
// This library supports character codes of at most 4 bytes.  When a CMap is
// read, code space ranges and mappings with longer codes are ignored.
// A usecmap chain may contain at most [limits.MaxCMapChainDepth] CMaps,
// counting the CMap itself; deeper chains can be neither read nor written.
//
// Lookups use an index of the mappings of each CMap in the chain, which is
// built on first use.  The Singles, Ranges and Parent fields of a
// ToUnicodeFile, and of every ToUnicodeFile it inherits from, must not be
// modified once it has been used by [ToUnicodeFile.Lookup],
// [ToUnicodeFile.All] or [ToUnicodeFile.CodeForText].
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
// parent extraction through [pdf.Cursor.Decode].
func ExtractToUnicode(c pdf.Cursor, obj pdf.Object, _ bool) (*ToUnicodeFile, error) {
	resolved, err := c.Resolve(obj)
	if err != nil || resolved == nil {
		return nil, err
	}
	stmObj, ok := resolved.(*pdf.Stream)
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing ToUnicode stream, got %T", resolved),
		}
	}

	err = c.CheckDictType(stmObj.Dict, "CMap")
	if err != nil {
		return nil, err
	}

	body, err := c.ReadAll(stmObj, limits.MaxCMapBytes)
	if err != nil {
		return nil, err
	}

	res, err := readToUnicode(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	if parent := stmObj.Dict["UseCMap"]; parent != nil {
		parentInfo, err := c.Decode(parent, ExtractToUnicode)
		if pdf.IsReadError(err) || errors.Is(err, errChainTooDeep) {
			return nil, err
		}
		res.Parent = parentInfo
	}

	if res.Parent != nil && parentTooDeep(res.Parent.index()) {
		return nil, &pdf.MalformedFileError{Err: errChainTooDeep}
	}
	if !toUnicodeLevelFits(res) {
		return nil, &pdf.MalformedFileError{Err: errTooComplex}
	}

	// The check needs the index of res.  The indices of the parents are
	// already cached, so the work for a usecmap chain stays linear.
	if res.index().tooComplex() {
		return nil, &pdf.MalformedFileError{Err: errTooComplex}
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
		r := charcode.Range{Low: entry.Low, High: entry.High}
		if !r.IsValid() {
			continue
		}
		res.CodeSpaceRange = append(res.CodeSpaceRange, r)
	}

	for _, entry := range codeMap.BfChars {
		if !validCodeLength(len(entry.Src)) {
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
		if len(entry.Low) != len(entry.High) || !validCodeLength(len(entry.Low)) {
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
// https://github.com/pdf-association/pdf-issues/issues/344 is resolved.
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

// All iterates over the codes which the ToUnicode CMap, including the CMaps
// it inherits from, maps to text.  For every code, the iterator reports the
// text which [ToUnicodeFile.Lookup] gives.  Each code is reported once, and
// codes which are not valid for codec are skipped.
//
// The enumeration stops after [limits.MaxCMapMappings] codes have been
// considered.  Mappings with higher precedence are considered first, so
// that a CMap's own mappings are reported before those it inherits.
func (tu *ToUnicodeFile) All(codec *charcode.Codec) iter.Seq2[charcode.Code, string] {
	return func(yield func(charcode.Code, string) bool) {
		if tu == nil {
			return
		}
		tu.index().mapped(func(x *toUnicodeLevel, id, pos int, codeBytes []byte) bool {
			code, k, valid := codec.Decode(codeBytes)
			if !valid || k != len(codeBytes) {
				return true
			}
			return yield(code, x.entries[id].text(pos))
		})
	}
}

// Lookup returns the unicode string for the given character code,
// together with a flag indicating whether the code was present in the
// ToUnicode CMap.
//
// A mapping in a CMap takes precedence over one in the CMap it uses.
// Within one CMap, bfchar mappings take precedence over bfrange mappings,
// and a later mapping takes precedence over an earlier one.  A code whose
// position in a range exceeds math.MaxInt32 is not mapped by that range.
//
// The lookup uses an index which is built on first use, see
// [ToUnicodeFile].
func (tu *ToUnicodeFile) Lookup(code []byte) (string, bool) {
	if tu == nil {
		return "", false
	}
	level, id := tu.index().lookup(code)
	if id < 0 {
		return "", false
	}
	e := level.entries[id]
	if !e.isRange {
		return e.values[0], true
	}
	index, _ := rangeIndex(e.first, e.last, code)
	return e.text(index), true
}

// CodeForText returns a character code whose text is exactly text.  If several
// codes map to text, the smallest one is returned.  The flag is false if no
// code maps to text.
//
// The search is bounded, so on malformed input some matches may be missed.
func (tu *ToUnicodeFile) CodeForText(text string) ([]byte, bool) {
	if tu == nil {
		return nil, false
	}
	var best []byte
	tu.index().mapped(func(x *toUnicodeLevel, id, pos int, code []byte) bool {
		if (best == nil || codeLess(code, best)) && x.entries[id].text(pos) == text {
			best = bytes.Clone(code)
		}
		return true
	})
	return best, best != nil
}

// toUnicodeLevel allows to look up the mappings of a single ToUnicode CMap,
// without those it inherits.  Entries are numbered in order of precedence,
// so that a smaller id takes precedence over a larger one.
type toUnicodeLevel struct {
	entries []toUnicodeEntry
	index   *codeIndex
}

func (x *toUnicodeLevel) numEntries() int {
	return len(x.entries)
}

func (x *toUnicodeLevel) entryCodes(id int) iter.Seq2[int, []byte] {
	return x.entries[id].codes()
}

func (x *toUnicodeLevel) lookup(code []byte) int {
	return x.index.lookup(code)
}

func (x *toUnicodeLevel) hasLeftover() bool {
	return len(x.index.leftover) > 0
}

// toUnicodeEntry is a bfchar or bfrange entry of a ToUnicode CMap.
type toUnicodeEntry struct {
	first, last []byte
	values      []string
	isRange     bool
}

// codes iterates over the codes of the entry, together with their position
// in the entry.
func (e toUnicodeEntry) codes() iter.Seq2[int, []byte] {
	if e.isRange {
		return codesInRange(e.first, e.last)
	}
	return func(yield func(int, []byte) bool) {
		yield(0, e.first)
	}
}

// text returns the text for the code at the given position in the entry.
func (e toUnicodeEntry) text(i int) string {
	if i < len(e.values) {
		return e.values[i]
	}
	return nextString(e.values[0], i)
}

// toUnicodeIndexes holds the indices of [ToUnicodeFile] objects, see
// [cachedChainIndex].
var toUnicodeIndexes sync.Map

// index returns the index of tu and its parents.  The receiver must not be
// nil.
func (tu *ToUnicodeFile) index() *chainIndex[*toUnicodeLevel] {
	return cachedChainIndex(&toUnicodeIndexes, tu, toUnicodeParent, buildToUnicodeLevel)
}

func toUnicodeParent(tu *ToUnicodeFile) *ToUnicodeFile {
	return tu.Parent
}

// buildToUnicodeLevel builds the index for the mappings of tu, without
// those of its parents.  This takes O(n log n) time for n entries.
func buildToUnicodeLevel(tu *ToUnicodeFile) *toUnicodeLevel {
	x, boxes := toUnicodeLevelBoxes(tu)
	x.index = newCodeIndex(boxes)
	return x
}

// toUnicodeLevelFits reports whether the index for the mappings of tu,
// without those of its parents, has no leftover boxes, see [piecesFit].
// This takes time linear in the number of entries.
func toUnicodeLevelFits(tu *ToUnicodeFile) bool {
	_, boxes := toUnicodeLevelBoxes(tu)
	return piecesFit(boxes)
}

// toUnicodeLevelBoxes returns the entries of tu, without their index,
// together with the boxes for the index.
func toUnicodeLevelBoxes(tu *ToUnicodeFile) (x *toUnicodeLevel, boxes []box) {
	x = &toUnicodeLevel{}
	for _, s := range slices.Backward(tu.Singles) {
		boxes = append(boxes, box{s.Code, s.Code, len(x.entries)})
		x.entries = append(x.entries, toUnicodeEntry{
			first:  s.Code,
			last:   s.Code,
			values: []string{s.Value},
		})
	}
	for _, r := range slices.Backward(tu.Ranges) {
		if len(r.Values) == 0 {
			continue
		}
		boxes = indexedBoxes(boxes, r.First, r.Last, math.MaxInt32, len(x.entries))
		x.entries = append(x.entries, toUnicodeEntry{
			first:   r.First,
			last:    r.Last,
			values:  r.Values,
			isRange: true,
		})
	}
	return x, boxes
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
	if err := checkWritable(tu, toUnicodeParent, toUnicodeLevelFits); err != nil {
		return nil, err
	}

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
// is resolved, reconsider CIDSystemInfo.
var toUnicodeTmplNew = template.Must(template.New("cmap").Funcs(template.FuncMap{
	"PN": func(s pdf.Name) string {
		x := postscript.Name(string(s))
		return x.PS()
	},
	"B": func(x []byte) string {
		return fmt.Sprintf("<%02x>", x)
	},
	"CodeSpaceChunks": chunks[charcode.Range],
	"SingleChunks":    chunks[ToUnicodeSingle],
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
{{range CodeSpaceChunks .CodeSpaceRange -}}
{{len .}} begincodespacerange
{{range . -}}
{{B .Low}} {{B .High}}
{{end -}}
endcodespacerange
{{end -}}
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

const brokenReplacement = "\uFFFD"
