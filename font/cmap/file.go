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
	"fmt"
	"io"
	"sync"
	"text/template"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript"
)

// References:
// - section 9.7.5 (CMaps) in ISO 32000-2:2020
// - https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// - https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf

// File represents the information for a CMap used with a PDF composite
// font.  This describes a mapping from character codes (one or more bytes) to
// character identifiers (CIDs).
//
// This structure closely resembles the structure of a CMap file.
type File struct {
	Name  pdf.Name
	ROS   *CIDSystemInfo
	WMode WritingMode

	charcode.CodeSpaceRange
	CIDSingles    []SingleNew
	CIDRanges     []RangeNew
	NotdefSingles []SingleNew
	NotdefRanges  []RangeNew

	Parent *File // This corresponds to the UseCMap entry in the PDF spec.
}

// WritingMode is the "writing mode" of a PDF font (horizontal or vertical).
type WritingMode int

func (m WritingMode) String() string {
	switch m {
	case Horizontal:
		return "horizontal"
	case Vertical:
		return "vertical"
	default:
		return fmt.Sprintf("WritingMode(%d)", m)
	}
}

const (
	// Horizontal indicates horizontal writing mode.
	Horizontal WritingMode = 0

	// Vertical indicates vertical writing mode.
	Vertical WritingMode = 1
)

// SingleEntry specifies that character code Code represents the given CID.
type SingleNew struct {
	Code  []byte
	Value CID
}

// RangeEntry describes a range of character codes with consecutive CIDs.
// First and Last are the first and last code points in the range.
// Value is the CID of the first code point in the range.
type RangeNew struct {
	First []byte
	Last  []byte
	Value CID
}

func (r RangeNew) String() string {
	return fmt.Sprintf("% 02x-% 02x: %d", r.First, r.Last, r.Value)
}

// CID is a character identifier.
//
// CID values are used to look up glyphs in a CIDFont.
// The interpretation of a CID value depends on the corresponding
// CIDSystemInfo.
//
// The special value 0 is used to indicate a missing glyph.
//
// TODO(voss): merge with cid.CID
type CID uint32

// ExtractNew extracts a CMap from a PDF object.
// The argument must be the name of a predefined CMap or a stream containing a CMap.
func ExtractNew(r pdf.Getter, obj pdf.Object) (*File, error) {
	predefinedMu.Lock()
	defer predefinedMu.Unlock()

	cycle := pdf.NewCycleChecker()
	return safeExtractCMap(r, cycle, obj)
}

// This must be called with predefinedMu locked.
func safeExtractCMap(r pdf.Getter, cycle *pdf.CycleChecker, obj pdf.Object) (*File, error) {
	if err := cycle.Check(obj); err != nil {
		return nil, err
	}

	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}

	if name, ok := obj.(pdf.Name); ok {
		if res, ok := predefinedCMap[name]; ok {
			return res, nil
		}
	}

	var body io.Reader
	var dict pdf.Dict
	switch obj := obj.(type) {
	case pdf.Name:
		stm, err := openPredefined(string(obj))
		if err != nil {
			return nil, err
		}
		defer stm.Close()
		body = stm

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

	res, parent, err := readCMap(body)
	if err != nil {
		return nil, err
	}

	if name, _ := pdf.GetName(r, dict["CMapName"]); name != "" {
		res.Name = name
	}
	if ros, _ := ExtractCIDSystemInfo(r, dict["CIDSystemInfo"]); ros != nil {
		res.ROS = ros
	}
	if x, _ := pdf.GetInteger(r, dict["WMode"]); x == 1 {
		res.WMode = Vertical
	}
	if p := dict["UseCMap"]; p != nil {
		parent = p
	}

	if parent != nil {
		res.Parent, err = safeExtractCMap(r, cycle, parent)
		if pdf.IsReadError(err) {
			return nil, err
		}
	}

	if name, ok := obj.(pdf.Name); ok {
		predefinedCMap[name] = res
		predefinedName[res] = name
	}

	return res, nil
}

func readCMap(r io.Reader) (*File, pdf.Object, error) {
	raw, err := postscript.ReadCMap(r)
	if err != nil {
		return nil, nil, err
	}

	if tp, _ := raw["CMapType"].(postscript.Integer); !(tp == 0 || tp == 1) {
		return nil, nil, pdf.Errorf("invalid CMapType: %d", tp)
	}

	res := &File{}
	var parent pdf.Object

	if name, _ := raw["CMapName"].(postscript.Name); name != "" {
		res.Name = pdf.Name(name)
	}
	if wMode, _ := raw["WMode"].(postscript.Integer); wMode == 1 {
		res.WMode = Vertical
	}
	if rosDict, _ := raw["CIDSystemInfo"].(postscript.Dict); rosDict != nil {
		ros := &CIDSystemInfo{}
		if registry, _ := rosDict["Registry"].(postscript.String); registry != nil {
			ros.Registry = string(registry)
		}
		if ordering, _ := rosDict["Ordering"].(postscript.String); ordering != nil {
			ros.Ordering = string(ordering)
		}
		if supplement, _ := rosDict["Supplement"].(postscript.Integer); supplement != 0 {
			ros.Supplement = pdf.Integer(supplement)
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
		res.CIDSingles = append(res.CIDSingles, SingleNew{
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
		res.CIDRanges = append(res.CIDRanges, RangeNew{
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
		res.NotdefSingles = append(res.NotdefSingles, SingleNew{
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
		res.NotdefRanges = append(res.NotdefRanges, RangeNew{
			First: entry.Low,
			Last:  entry.High,
			Value: CID(cid),
		})
	}

	return res, parent, nil
}

func (c *File) LookupCID(code []byte) CID {
	for _, s := range c.CIDSingles {
		if bytes.Equal(s.Code, code) {
			return s.Value
		}
	}

rangesLoop:
	for _, r := range c.CIDRanges {
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

	if c.Parent != nil {
		return c.Parent.LookupCID(code)
	}

	return c.LookupNotdefCID(code)
}

func (c *File) LookupNotdefCID(code []byte) CID {
	for _, s := range c.NotdefSingles {
		if bytes.Equal(s.Code, code) {
			return s.Value
		}
	}

rangesLoop:
	for _, r := range c.NotdefRanges {
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

	if c.Parent != nil {
		return c.Parent.LookupNotdefCID(code)
	}
	return 0
}

func (c *File) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	predefinedMu.Lock()
	predefinedName, ok := predefinedName[c]
	predefinedMu.Unlock()
	if ok {
		return predefinedName, zero, nil
	}

	ros, _, err := pdf.ResourceManagerEmbed(rm, c.ROS)
	if err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Type":          pdf.Name("CMap"),
		"CMapName":      pdf.Name(c.Name),
		"CIDSystemInfo": ros,
	}
	if c.WMode != Horizontal {
		dict["WMode"] = pdf.Integer(c.WMode)
	}
	if c.Parent != nil {
		parent, _, err := pdf.ResourceManagerEmbed(rm, c.Parent)
		if err != nil {
			return nil, zero, err
		}
		dict["UseCMap"] = parent
	}

	var filters []pdf.Filter
	opt := rm.Out.GetOptions()
	if !opt.HasAny(pdf.OptPretty) {
		filters = append(filters, pdf.FilterCompress{})
	}

	data := templateData{
		HeaderComment: opt.HasAny(pdf.OptPretty),
		File:          c,
	}

	ref := rm.Out.Alloc()
	stm, err := rm.Out.OpenStream(ref, dict, filters...)
	if err != nil {
		return nil, zero, err
	}
	err = cmapTmplNew.Execute(stm, data)
	if err != nil {
		return nil, zero, fmt.Errorf("embedding cmap: %w", err)
	}
	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
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
	"PN": func(s pdf.Name) string {
		x := postscript.Name(string(s))
		return x.PS()
	},
	"B": func(x []byte) string {
		return fmt.Sprintf("<%02x>", x)
	},
	"SingleChunks": chunks[SingleNew],
	"Single": func(s SingleNew) string {
		return fmt.Sprintf("<%x> %d", s.Code, s.Value)
	},
	"RangeChunks": chunks[RangeNew],
	"Range": func(r RangeNew) string {
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

var (
	predefinedMu   sync.Mutex
	predefinedCMap = make(map[pdf.Name]*File)
	predefinedName = make(map[*File]pdf.Name)
)
