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
	"math"
	"text/template"

	"seehuhn.de/go/postscript"
	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
)

// References:
// - section 9.7.5 (CMaps) in ISO 32000-2:2020
// - https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// - https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf

// CID represents a character identifier.  This identifies a character within
// a character collection.
//
// TODO(voss): remove in favour of [cid.CID]
type CID = cid.CID

// File represents the information for a CMap used with a PDF composite
// font.  This describes a mapping from character codes (one or more bytes) to
// character identifiers (CIDs).
//
// This structure reflects the structure of a CMap file.
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
	Value CID
}

// Range describes a range of character codes with consecutive CIDs.
// First and Last are the first and last code points in the range.
// Value is the CID of the first code point in the range.
type Range struct {
	First []byte
	Last  []byte
	Value CID
}

func (r Range) String() string {
	return fmt.Sprintf("% 02x-% 02x: %d", r.First, r.Last, r.Value)
}

// Extract extracts a CMap from a PDF object.
// The argument must be the name of a predefined CMap or a stream containing a CMap.
func Extract(r pdf.Getter, obj pdf.Object) (*File, error) {
	cycle := pdf.NewCycleChecker()
	return safeExtractCMap(r, cycle, obj)
}

// safeExtractCMap extracts a CMap from a PDF object with cycle detection.
// The obj parameter can be a pdf.Name (for predefined CMaps) or a stream reference.
func safeExtractCMap(r pdf.Getter, cycle *pdf.CycleChecker, obj pdf.Object) (*File, error) {
	if err := cycle.Check(obj); err != nil {
		return nil, err
	}

	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}

	var body io.Reader
	var dict pdf.Dict
	switch obj := obj.(type) {
	case pdf.Name:
		return Predefined(string(obj))

	case *pdf.Stream:
		dict = obj.Dict

		err := pdf.CheckDictType(r, dict, "CMap")
		if err != nil {
			return nil, err
		}

		stm, err := pdf.DecodeStream(r, obj, 0)
		if err != nil {
			return nil, err
		}

		body = io.NopCloser(stm)

	default:
		return nil, pdf.Errorf("invalid CMap object type: %T", obj)
	}

	res, parentName, err := readCMap(body)
	if err != nil {
		return nil, err
	}

	if name, _ := pdf.GetName(r, dict["CMapName"]); name != "" {
		res.Name = string(name)
	}
	if ros, _ := font.ExtractCIDSystemInfo(r, dict["CIDSystemInfo"]); ros != nil {
		res.ROS = ros
	}
	if x, _ := pdf.GetInteger(r, dict["WMode"]); x == 1 {
		res.WMode = font.Vertical
	}

	// Handle parent CMap
	if useCMap := dict["UseCMap"]; useCMap != nil {
		// Stream dictionary provides the parent CMap location
		res.Parent, err = safeExtractCMap(r, cycle, useCMap)
		if pdf.IsReadError(err) {
			return nil, err
		}
	} else if parentName != "" {
		// No stream dictionary, try predefined CMap
		res.Parent, err = safeExtractCMap(r, cycle, parentName)
		if pdf.IsReadError(err) {
			return nil, err
		}
	}

	return res, nil
}

// readCMap reads and parses a CMap from a PostScript stream.
// It returns the parsed CMap, the parent CMap name (if any), and any error.
func readCMap(r io.Reader) (*File, pdf.Name, error) {
	raw, err := postscript.ReadCMap(r)
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

	codeMap := raw["CodeMap"].(*postscript.CMapInfo)
	if codeMap.UseCMap != "" {
		parent = pdf.Name(codeMap.UseCMap)
	}

	for _, entry := range codeMap.CodeSpaceRanges {
		if len(entry.Low) != len(entry.High) || len(entry.Low) == 0 {
			continue
		}
		res.CodeSpaceRange = append(res.CodeSpaceRange,
			charcode.Range{Low: entry.Low, High: entry.High})
	}

	for _, entry := range codeMap.CidChars {
		if len(entry.Src) == 0 {
			continue
		}
		cid, ok := entry.Dst.(postscript.Integer)
		if !ok || cid < 0 || cid > 0xFFFF_FFFF {
			continue
		}
		res.CIDSingles = append(res.CIDSingles, Single{
			Code:  entry.Src,
			Value: CID(cid),
		})
	}
	for _, entry := range codeMap.CidRanges {
		if len(entry.Low) != len(entry.High) || len(entry.Low) == 0 {
			continue
		}
		cid, ok := entry.Dst.(postscript.Integer)
		if !ok || cid < 0 || cid > 0xFFFF_FFFF {
			continue
		}
		res.CIDRanges = append(res.CIDRanges, Range{
			First: entry.Low,
			Last:  entry.High,
			Value: CID(cid),
		})
	}
	for _, entry := range codeMap.NotdefChars {
		if len(entry.Src) == 0 {
			continue
		}
		cid, ok := entry.Dst.(postscript.Integer)
		if !ok || cid < 0 || cid > 0xFFFF_FFFF {
			continue
		}
		res.NotdefSingles = append(res.NotdefSingles, Single{
			Code:  entry.Src,
			Value: CID(cid),
		})
	}
	for _, entry := range codeMap.NotdefRanges {
		if len(entry.Low) != len(entry.High) || len(entry.Low) == 0 {
			continue
		}
		cid, ok := entry.Dst.(postscript.Integer)
		if !ok || cid < 0 || cid > 0xFFFF_FFFF {
			continue
		}
		res.NotdefRanges = append(res.NotdefRanges, Range{
			First: entry.Low,
			Last:  entry.High,
			Value: CID(cid),
		})
	}

	return res, parent, nil
}

// Clone returns a shallow copy of the CMap file.
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

func (f *File) All(codec *charcode.Codec) iter.Seq2[charcode.Code, cid.CID] {
	return func(yield func(charcode.Code, cid.CID) bool) {
		if f.Parent != nil {
			for code, cid := range f.Parent.All(codec) {
				if !yield(code, cid) {
					return
				}
			}
		}

		for _, r := range f.CIDRanges {
			for i, codeBytes := range codesInRange(r.First, r.Last) {
				code, k, valid := codec.Decode(codeBytes)
				if !valid || k != len(codeBytes) {
					continue
				}
				if !yield(code, r.Value+CID(i)) {
					return
				}
			}
		}
		for _, single := range f.CIDSingles {
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

func (f *File) LookupCID(code []byte) CID {
	for _, s := range f.CIDSingles {
		if bytes.Equal(s.Code, code) {
			return s.Value
		}
	}

rangesLoop:
	for _, r := range f.CIDRanges {
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

		return r.Value + CID(index)
	}

	if f.Parent != nil {
		return f.Parent.LookupCID(code)
	}

	return f.LookupNotdefCID(code)
}

func (f *File) LookupNotdefCID(code []byte) CID {
	for _, s := range f.NotdefSingles {
		if bytes.Equal(s.Code, code) {
			return s.Value
		}
	}

rangesLoop:
	for _, r := range f.NotdefRanges {
		if len(r.First) != len(code) || len(r.Last) != len(code) {
			continue
		}
		for i, b := range code {
			if b < r.First[i] || b > r.Last[i] {
				continue rangesLoop
			}
		}

		return r.Value
	}

	if f.Parent != nil {
		return f.Parent.LookupNotdefCID(code)
	}
	return 0
}

func (f *File) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	// TODO(voss): decide this based on the CMap content?
	if f.IsPredefined() {
		return pdf.Name(f.Name), zero, nil
	}

	ros, err := pdf.EmbedHelperEmbedFunc(rm, font.WriteCIDSystemInfo, f.ROS)
	if err != nil {
		return nil, zero, err
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
		parent, _, err := pdf.EmbedHelperEmbed(rm, f.Parent)
		if err != nil {
			return nil, zero, err
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
		return nil, zero, err
	}

	err = f.WriteTo(stm, opt.HasAny(pdf.OptPretty))
	if err != nil {
		return nil, zero, fmt.Errorf("embedding cmap: %w", err)
	}

	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

func (f *File) WriteTo(w io.Writer, pretty bool) error {
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
	"SingleChunks": chunks[Single],
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
{{with .CodeSpaceRange -}}
{{len .}} begincodespacerange
{{range . -}}
{{B .Low}} {{B .High}}
{{end -}}
{{end -}}
endcodespacerange
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
