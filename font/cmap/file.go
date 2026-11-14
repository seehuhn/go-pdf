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
	"sync"
	"text/template"

	"seehuhn.de/go/postscript"
	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/internal/limits"
)

// References:
// - section 9.7.5 (CMaps) in ISO 32000-2:2020
// - https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// - https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf

// File represents the information for a CMap used with a PDF composite
// font.  This describes a mapping from character codes (one or more bytes) to
// character identifiers (CIDs).
//
// This structure reflects the structure of a CMap file.
//
// This library supports character codes of at most 4 bytes.  When a CMap is
// read, code space ranges and mappings with longer codes are ignored.
// A usecmap chain may contain at most [limits.MaxCMapChainDepth] CMaps,
// counting the CMap itself; deeper chains can be neither read nor written.
//
// Lookups use an index of the mappings of each CMap in the chain, which is
// built on first use.  The CIDSingles, CIDRanges, NotdefSingles,
// NotdefRanges and Parent fields of a File, and of every File it inherits
// from, must not be modified once it has been used by [File.LookupCID],
// [File.LookupNotdefCID] or [File.All].
type File struct {
	Name  string
	ROS   *cid.SystemInfo
	WMode font.WritingMode

	charcode.CodeSpaceRange
	CIDSingles []Single
	CIDRanges  []Range

	NotdefSingles []Single
	NotdefRanges  []Range

	Parent *File
}

// Single specifies that character code `Code` represents the CID `Value`.
type Single struct {
	Code  []byte
	Value cid.CID
}

// Range describes a range of character codes with consecutive CIDs.
// First and Last are the first and last code points in the range.
// Value is the CID of the first code point in the range.
type Range struct {
	First []byte
	Last  []byte
	Value cid.CID
}

func (r Range) String() string {
	return fmt.Sprintf("% 02x-% 02x: %d", r.First, r.Last, r.Value)
}

// Extract extracts a CMap from a PDF object.
// The argument must be the name of a predefined CMap or a stream containing a CMap.
// Cycle detection for recursive parent CMap chains is handled by routing
// parent extraction through [pdf.Cursor.Decode].
func Extract(c pdf.Cursor, obj pdf.Object, _ bool) (*File, error) {
	resolved, err := c.Resolve(obj)
	if err != nil {
		return nil, err
	}

	var body io.Reader
	var dict pdf.Dict
	switch obj := resolved.(type) {
	case pdf.Name:
		return Predefined(string(obj))

	case *pdf.Stream:
		dict = obj.Dict

		err := c.CheckDictType(dict, "CMap")
		if err != nil {
			return nil, err
		}

		data, err := c.ReadAll(obj, limits.MaxCMapBytes)
		if err != nil {
			return nil, err
		}

		body = bytes.NewReader(data)

	default:
		return nil, pdf.Errorf("invalid CMap object type: %T", obj)
	}

	res, parentName, err := readCMap(body)
	if err != nil {
		return nil, err
	}

	if name, _ := c.Name(dict["CMapName"]); name != "" {
		if clean := sanitizeName(string(name)); clean != "" {
			res.Name = clean
		}
	}
	if ros, _ := c.Decode(dict["CIDSystemInfo"], font.ExtractCIDSystemInfo); ros != nil {
		res.ROS = ros
	}
	if wMode, _ := c.Integer(dict["WMode"]); wMode == 1 {
		res.WMode = font.Vertical
	}

	// parent CMap
	if useCMap := dict["UseCMap"]; useCMap != nil {
		res.Parent, err = c.Decode(useCMap, Extract)
		if pdf.IsReadError(err) || errors.Is(err, errChainTooDeep) {
			return nil, err
		}
	} else if parentName != "" {
		res.Parent, _ = Predefined(string(parentName))
	}

	if res.Parent != nil && parentTooDeep(res.Parent.index()) {
		return nil, &pdf.MalformedFileError{Err: errChainTooDeep}
	}
	if !fileLevelFits(res) {
		return nil, &pdf.MalformedFileError{Err: errTooComplex}
	}

	// The check needs the index of res.  The indices of the parents are
	// already cached, so the work for a usecmap chain stays linear.
	if res.index().tooComplex() {
		return nil, &pdf.MalformedFileError{Err: errTooComplex}
	}

	return res, nil
}

// sanitizeName removes characters which are not valid in PostScript names.
func sanitizeName(s string) string {
	clean := make([]byte, 0, len(s))
	for i := range len(s) {
		c := s[i]
		if c > 32 && c != '(' && c != ')' && c != '<' && c != '>' &&
			c != '[' && c != ']' && c != '{' && c != '}' && c != '/' && c != '%' {
			clean = append(clean, c)
		}
	}
	return string(clean)
}

// readCMap reads and parses a CMap from a PostScript stream.
// It returns the parsed CMap, the parent CMap name (if any), and any error.
func readCMap(r io.Reader) (*File, pdf.Name, error) {
	raw, codeMap, err := postscript.ReadCMap(r)
	if err != nil {
		return nil, "", err
	}

	if tp, _ := raw["CMapType"].(postscript.Integer); !(tp == 0 || tp == 1) {
		return nil, "", pdf.Errorf("invalid CMapType: %d", tp)
	}

	res := &File{}
	var parent pdf.Name

	if name, _ := raw["CMapName"].(postscript.Name); name != "" {
		res.Name = string(name)
	}
	if wMode, _ := raw["WMode"].(postscript.Integer); wMode == 1 {
		res.WMode = font.Vertical
	}
	if rosDict, _ := raw["CIDSystemInfo"].(postscript.Dict); rosDict != nil {
		ros := &cid.SystemInfo{}
		if registry, _ := rosDict["Registry"].(postscript.String); registry != nil {
			ros.Registry = string(registry)
		}
		if ordering, _ := rosDict["Ordering"].(postscript.String); ordering != nil {
			ros.Ordering = string(ordering)
		}
		if supplement, _ := rosDict["Supplement"].(postscript.Integer); supplement != 0 {
			var sup int32
			if supplement >= 0 && supplement <= math.MaxInt32 {
				sup = int32(supplement)
			}
			ros.Supplement = sup
		}
		if ros.Registry != "" || ros.Ordering != "" || ros.Supplement != 0 {
			res.ROS = ros
		}
	}

	if codeMap.UseCMap != "" {
		parent = pdf.Name(codeMap.UseCMap)
	}

	for _, entry := range codeMap.CodeSpaceRanges {
		r := charcode.Range{Low: entry.Low, High: entry.High}
		if !r.IsValid() {
			continue
		}
		res.CodeSpaceRange = append(res.CodeSpaceRange, r)
	}

	for _, entry := range codeMap.CidChars {
		if !validCodeLength(len(entry.Src)) {
			continue
		}
		cidVal, ok := entry.Dst.(postscript.Integer)
		if !ok || cidVal < 0 || int64(cidVal) > math.MaxUint32 {
			continue
		}
		res.CIDSingles = append(res.CIDSingles, Single{
			Code:  entry.Src,
			Value: cid.CID(cidVal),
		})
	}
	for _, entry := range codeMap.CidRanges {
		if len(entry.Low) != len(entry.High) || !validCodeLength(len(entry.Low)) {
			continue
		}
		cidVal, ok := entry.Dst.(postscript.Integer)
		if !ok || cidVal < 0 || int64(cidVal) > math.MaxUint32 {
			continue
		}
		res.CIDRanges = append(res.CIDRanges, Range{
			First: entry.Low,
			Last:  entry.High,
			Value: cid.CID(cidVal),
		})
	}
	for _, entry := range codeMap.NotdefChars {
		if !validCodeLength(len(entry.Src)) {
			continue
		}
		cidVal, ok := entry.Dst.(postscript.Integer)
		if !ok || cidVal < 0 || int64(cidVal) > math.MaxUint32 {
			continue
		}
		res.NotdefSingles = append(res.NotdefSingles, Single{
			Code:  entry.Src,
			Value: cid.CID(cidVal),
		})
	}
	for _, entry := range codeMap.NotdefRanges {
		if len(entry.Low) != len(entry.High) || !validCodeLength(len(entry.Low)) {
			continue
		}
		cidVal, ok := entry.Dst.(postscript.Integer)
		if !ok || cidVal < 0 || int64(cidVal) > math.MaxUint32 {
			continue
		}
		res.NotdefRanges = append(res.NotdefRanges, Range{
			First: entry.Low,
			Last:  entry.High,
			Value: cid.CID(cidVal),
		})
	}

	return res, parent, nil
}

// Clone returns a shallow copy of the CMap file.  The copy shares the
// slices of mappings with f, so these must be replaced rather than modified
// in place.
func (f *File) Clone() *File {
	clone := *f
	return &clone
}

// UpdateName updates the Name field of the CMap to a unique value based on the
// content of the CMap.
func (f *File) UpdateName() {
	if f.IsPredefined() {
		// predefined CMaps have a fixed, unique name
		return
	}

	h := md5.New()
	f.writeBinary(h, 3)
	f.Name = fmt.Sprintf("seehuhn-%x", h.Sum(nil))
}

// writeBinary writes a binary representation of the ToUnicodeInfo object to
// the [hash.Hash] h.  The maxGen parameter limits the number of parent
// references, to avoid infinite recursion.
func (f *File) writeBinary(h hash.Hash, maxGen int) {
	if maxGen <= 0 {
		return
	}

	const magic uint32 = 0x5dafbce2
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

	writeInt(len(f.CodeSpaceRange))
	for _, r := range f.CodeSpaceRange {
		writeBytes(r.Low)
		writeBytes(r.High)
	}

	writeInt(len(f.CIDSingles))
	for _, s := range f.CIDSingles {
		writeBytes(s.Code)
		writeInt(int(s.Value))
	}

	writeInt(len(f.CIDRanges))
	for _, r := range f.CIDRanges {
		writeBytes(r.First)
		writeBytes(r.Last)
		writeInt(int(r.Value))
	}

	writeInt(len(f.NotdefSingles))
	for _, s := range f.NotdefSingles {
		writeBytes(s.Code)
		writeInt(int(s.Value))
	}

	writeInt(len(f.NotdefRanges))
	for _, r := range f.NotdefRanges {
		writeBytes(r.First)
		writeBytes(r.Last)
		writeInt(int(r.Value))
	}

	if f.Parent != nil {
		writeInt(1)
		f.Parent.writeBinary(h, maxGen-1)
	} else {
		writeInt(0)
	}
}

// Codec returns a codec for the code space ranges of the CMap and of the
// CMaps it inherits from.  The error is that of [charcode.NewCodec]: the
// code space range is invalid, or its lookup tree would be too large.
func (f *File) Codec() (*charcode.Codec, error) {
	var cs charcode.CodeSpaceRange
	for _, g := range f.Chain() {
		cs = append(cs, g.CodeSpaceRange...)
	}
	return charcode.NewCodec(cs)
}

// All iterates over the codes which the CMap, including the CMaps it
// inherits from through usecmap, maps to CIDs.  For every code, the
// iterator reports the CID which [File.LookupCID] gives.  Each code is
// reported once, and codes which are not valid for codec are skipped.
//
// The enumeration stops after [limits.MaxCMapMappings] codes have been
// considered.  Mappings with higher precedence are considered first, so
// that a CMap's own mappings are reported before those it inherits.
func (f *File) All(codec *charcode.Codec) iter.Seq2[charcode.Code, cid.CID] {
	return func(yield func(charcode.Code, cid.CID) bool) {
		if f == nil {
			return
		}
		f.index().mapped(func(x *fileLevel, id, pos int, codeBytes []byte) bool {
			code, k, valid := codec.Decode(codeBytes)
			if !valid || k != len(codeBytes) {
				return true
			}
			return yield(code, x.mapped[id].value+cid.CID(pos))
		})
	}
}

// rangeIndex returns the position of code within the rectangular range
// [first,last], computed as a mixed-radix number over the per-byte spans.
// ok is false if code lies outside the range, the lengths disagree, or the
// position exceeds math.MaxInt32; in all of these cases the caller should
// treat the code as unmapped.  The MaxInt32 cap keeps the result within both
// cid.CID (a uint32) and a 32-bit int, so the behaviour is identical on 32- and
// 64-bit hosts and the callers can narrow without truncating or going
// negative.  No real CMap range needs a larger index: that would require more
// than 2^31 consecutive codes mapping to consecutive values.  The running
// accumulator uses int64 so it cannot wrap before the cap is checked.
func rangeIndex(first, last, code []byte) (index int, ok bool) {
	if len(first) != len(code) || len(last) != len(code) {
		return 0, false
	}
	var acc int64
	for i, b := range code {
		if b < first[i] || b > last[i] {
			return 0, false
		}
		span := int64(last[i]) - int64(first[i]) + 1
		acc = acc*span + (int64(b) - int64(first[i]))
		if acc > math.MaxInt32 {
			return 0, false
		}
	}
	return int(acc), true
}

// LookupCID returns the CID which the CMap, including the CMaps it inherits
// from through usecmap, assigns to the given code.  If the code has no CID
// mapping, the notdef mappings are consulted, as for [File.LookupNotdefCID].
//
// A mapping in a CMap takes precedence over one in the CMap it uses.
// Within one CMap, single-code mappings take precedence over ranges, and a
// later mapping takes precedence over an earlier one.  A code whose position
// in a range exceeds math.MaxInt32, or whose CID would exceed
// math.MaxUint32, is not mapped by that range.
//
// The lookup uses an index which is built on first use, see [File].
func (f *File) LookupCID(code []byte) cid.CID {
	if f == nil {
		return 0
	}
	x := f.index()
	if level, id := x.lookup(code); id >= 0 {
		return level.mapped[id].cid(code)
	}
	return lookupNotdef(x, code)
}

// LookupNotdefCID returns the CID which the notdef mappings of the CMap,
// including those it inherits through usecmap, assign to the given code.
// The precedence is the same as for [File.LookupCID].  If no notdef mapping
// matches, the result is 0.
func (f *File) LookupNotdefCID(code []byte) cid.CID {
	if f == nil {
		return 0
	}
	return lookupNotdef(f.index(), code)
}

// fileLevel allows to look up the mappings of a single CMap, without those
// it inherits.  Entries are numbered in order of precedence, so that a
// smaller id takes precedence over a larger one.
type fileLevel struct {
	mapped      []mappedEntry
	mappedIndex *codeIndex

	notdef      []cid.CID
	notdefIndex *codeIndex
}

func (x *fileLevel) numEntries() int {
	return len(x.mapped)
}

func (x *fileLevel) entryCodes(id int) iter.Seq2[int, []byte] {
	return x.mapped[id].codes()
}

func (x *fileLevel) lookup(code []byte) int {
	return x.mappedIndex.lookup(code)
}

func (x *fileLevel) hasLeftover() bool {
	return len(x.mappedIndex.leftover) > 0 || len(x.notdefIndex.leftover) > 0
}

// mappedEntry is a cidchar or cidrange entry of a CMap.
type mappedEntry struct {
	first, last []byte
	value       cid.CID
	isRange     bool
}

// codes iterates over the codes of the entry, together with their position
// in the entry.
func (e mappedEntry) codes() iter.Seq2[int, []byte] {
	if e.isRange {
		return codesInRange(e.first, e.last)
	}
	return func(yield func(int, []byte) bool) {
		yield(0, e.first)
	}
}

// maxRangeIndex returns the largest index, as computed by [rangeIndex], of a
// code which a CID range starting at value maps: the index is at most
// math.MaxInt32, and the CID must not exceed math.MaxUint32.
func maxRangeIndex(value cid.CID) int {
	return int(min(math.MaxInt32, math.MaxUint32-uint64(value)))
}

// cid returns the CID of a code which the index has attributed to e.
// The index guarantees that the result does not overflow.
func (e mappedEntry) cid(code []byte) cid.CID {
	if !e.isRange {
		return e.value
	}
	index, _ := rangeIndex(e.first, e.last, code)
	return e.value + cid.CID(index)
}

// lookupNotdef returns the CID which the notdef mappings of the chain x
// assign to code, or 0.
func lookupNotdef(x *chainIndex[*fileLevel], code []byte) cid.CID {
	y := x
	for range x.depth {
		if id := y.level.notdefIndex.lookup(code); id >= 0 {
			return y.level.notdef[id]
		}
		y = y.parent
	}
	return 0
}

// fileIndexes holds the indices of [File] objects, see [cachedChainIndex].
var fileIndexes sync.Map

// index returns the index of f and its parents.  The receiver must not be
// nil.
func (f *File) index() *chainIndex[*fileLevel] {
	return cachedChainIndex(&fileIndexes, f, fileParent, buildFileLevel)
}

func fileParent(f *File) *File {
	return f.Parent
}

// buildFileLevel builds the index for the mappings of f, without those of
// its parents.  This takes O(n log n) time for n entries.
func buildFileLevel(f *File) *fileLevel {
	x, mapped, notdef := fileLevelBoxes(f)
	x.mappedIndex = newCodeIndex(mapped)
	x.notdefIndex = newCodeIndex(notdef)
	return x
}

// fileLevelFits reports whether the indices for the mappings of f, without
// those of its parents, have no leftover boxes, see [piecesFit].  This
// takes time linear in the number of entries.
func fileLevelFits(f *File) bool {
	_, mapped, notdef := fileLevelBoxes(f)
	return piecesFit(mapped) && piecesFit(notdef)
}

// fileLevelBoxes returns the entries of f, without their indices, together
// with the boxes for the CID and notdef indices.
func fileLevelBoxes(f *File) (x *fileLevel, mapped, notdef []box) {
	x = &fileLevel{}
	for _, s := range slices.Backward(f.CIDSingles) {
		id := len(x.mapped)
		x.mapped = append(x.mapped, mappedEntry{first: s.Code, last: s.Code, value: s.Value})
		mapped = append(mapped, box{s.Code, s.Code, id})
	}
	for _, r := range slices.Backward(f.CIDRanges) {
		id := len(x.mapped)
		x.mapped = append(x.mapped, mappedEntry{first: r.First, last: r.Last, value: r.Value, isRange: true})
		mapped = indexedBoxes(mapped, r.First, r.Last, maxRangeIndex(r.Value), id)
	}
	for _, s := range slices.Backward(f.NotdefSingles) {
		notdef = append(notdef, box{s.Code, s.Code, len(x.notdef)})
		x.notdef = append(x.notdef, s.Value)
	}
	for _, r := range slices.Backward(f.NotdefRanges) {
		notdef = append(notdef, box{r.First, r.Last, len(x.notdef)})
		x.notdef = append(x.notdef, r.Value)
	}
	return x, mapped, notdef
}

// Chain returns f and the CMaps it inherits from through usecmap, starting
// with f.  If the Parent links form a cycle, each CMap is listed once.
// For a nil f, the result is nil.
func (f *File) Chain() []*File {
	return walkChain(f, fileParent)
}

func (f *File) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	// Embedding by name is keyed on object identity with the predefined
	// cache, not on CMap content: a modified copy of a predefined CMap
	// must be written out in full, and comparing content would cost a
	// serialisation round trip for no practical benefit.
	if f.IsPredefined() {
		return pdf.Name(f.Name), nil
	}

	// checked before anything is written, so that a failed Embed leaves
	// nothing behind
	if err := checkWritable(f, fileParent, fileLevelFits); err != nil {
		return nil, err
	}

	ros, err := rm.EmbedFunc(font.WriteCIDSystemInfo, f.ROS)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Type":          pdf.Name("CMap"),
		"CMapName":      pdf.Name(f.Name),
		"CIDSystemInfo": ros,
	}
	if f.WMode != font.Horizontal {
		dict["WMode"] = pdf.Integer(f.WMode)
	}
	if f.Parent != nil {
		parent, err := rm.Embed(f.Parent)
		if err != nil {
			return nil, err
		}
		dict["UseCMap"] = parent
	}

	var filters []pdf.Filter
	opt := rm.Out().GetOptions()
	if !opt.HasAny(pdf.OptPretty) {
		filters = append(filters, pdf.FilterCompress{})
	}

	ref := rm.Alloc()
	stm, err := rm.Out().OpenStream(ref, dict, filters...)
	if err != nil {
		return nil, err
	}

	err = f.WriteTo(stm, opt.HasAny(pdf.OptPretty))
	if err != nil {
		return nil, fmt.Errorf("embedding cmap: %w", err)
	}

	err = stm.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// WriteTo writes the CMap file to w.  The parent CMap is referenced by name.
//
// An error is returned if the CMap, together with the CMaps it inherits
// from, has too many large ranges in which more than the last byte varies,
// since such a CMap could not be read back.
func (f *File) WriteTo(w io.Writer, pretty bool) error {
	if err := checkWritable(f, fileParent, fileLevelFits); err != nil {
		return err
	}

	data := templateData{
		HeaderComment: pretty,
		File:          f,
	}
	return cmapTmplNew.Execute(w, data)
}

const chunkSize = 100

func chunks[T any](x []T) [][]T {
	var res [][]T
	for len(x) >= chunkSize {
		res = append(res, x[:chunkSize])
		x = x[chunkSize:]
	}
	if len(x) > 0 {
		res = append(res, x)
	}
	return res
}

type templateData struct {
	HeaderComment bool
	*File
}

// cmapTmplNew is a template for generating a CMap stream.
// The expected context is a templateData structure.
var cmapTmplNew = template.Must(template.New("cmap").Funcs(template.FuncMap{
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
	"CodeSpaceChunks": chunks[charcode.Range],
	"SingleChunks":    chunks[Single],
	"Single": func(s Single) string {
		return fmt.Sprintf("<%x> %d", s.Code, s.Value)
	},
	"RangeChunks": chunks[Range],
	"Range": func(r Range) string {
		return fmt.Sprintf("<%x> <%x> %d", r.First, r.Last, r.Value)
	},
}).Parse(`{{if .HeaderComment -}}
%!PS-Adobe-3.0 Resource-CMap
{{end -}}
/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
{{if .Parent -}}
{{PN .Parent.Name}} usecmap
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
/WMode {{printf "%d" .WMode}} def
{{range CodeSpaceChunks .CodeSpaceRange -}}
{{len .}} begincodespacerange
{{range . -}}
{{B .Low}} {{B .High}}
{{end -}}
endcodespacerange
{{end -}}
{{/* */ -}}

{{range SingleChunks .CIDSingles -}}
{{len .}} begincidchar
{{range . -}}
{{Single .}}
{{end -}}
endcidchar
{{end -}}

{{range RangeChunks .CIDRanges -}}
{{len .}} begincidrange
{{range . -}}
{{Range .}}
{{end -}}
endcidrange
{{end -}}

{{range SingleChunks .NotdefSingles -}}
{{len .}} beginnotdefchar
{{range . -}}
{{Single .}}
{{end -}}
endnotdefchar
{{end -}}

{{range RangeChunks .NotdefRanges -}}
{{len .}} beginnotdefrange
{{range . -}}
{{Range .}}
{{end -}}
endnotdefrange
{{end -}}

endcmap
CMapName currentdict /CMap defineresource pop
end
end`))
