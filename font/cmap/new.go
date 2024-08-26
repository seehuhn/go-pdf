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
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript"
)

// References:
// - section 9.7.5 (CMaps) in ISO 32000-2:2020
// - https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// - https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf

// WritingMode is the "writing mode" of a PDF font (horizontal or vertical).
type WritingMode int

const (
	// Horizontal indicates horizontal writing mode.
	Horizontal WritingMode = 0

	// Vertical indicates vertical writing mode.
	Vertical WritingMode = 1
)

// InfoNew represents the information for a CMap used with a PDF composite font.
type InfoNew struct {
	Name   pdf.Name
	ROS    *CIDSystemInfo
	WMode  WritingMode
	Parent *InfoNew // This corresponds to the UseCMap entry in the PDF spec.

	charcode.CodeSpaceRange
	CIDSingles    []SingleEntryNew
	CIDRanges     []RangeEntryNew
	NotdefSingles []SingleEntryNew
	NotdefRanges  []RangeEntryNew
}

// SingleEntry specifies that character code Code represents the given CID.
type SingleEntryNew struct {
	Code  []byte
	Value CID
}

// RangeEntry describes a range of character codes with consecutive CIDs.
// First and Last are the first and last code points in the range.
// Value is the CID of the first code point in the range.
type RangeEntryNew struct {
	First []byte
	Last  []byte
	Value CID
}

type CID uint32

func ExtractCMap(r pdf.Getter, obj pdf.Object) (*InfoNew, error) {
	cycle := pdf.NewCycleChecker()
	return safeExtractCMap(r, cycle, obj)
}

func safeExtractCMap(r pdf.Getter, cycle *pdf.CycleChecker, obj pdf.Object) (*InfoNew, error) {
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
		stm, err := openPredefined(string(obj))
		if err != nil {
			return nil, err
		}
		defer stm.Close()
		body = stm

	case *pdf.Stream:
		err := pdf.CheckDictType(r, obj.Dict, "CMap")
		if err != nil {
			return nil, err
		}

		stm, err := pdf.DecodeStream(r, obj, 0)
		if err != nil {
			return nil, err
		}

		dict = obj.Dict
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
		res.Parent, _ = safeExtractCMap(r, cycle, parent)
	}

	return res, nil
}

func readCMap(r io.Reader) (*InfoNew, pdf.Object, error) {
	raw, err := postscript.ReadCMap(r)
	if err != nil {
		return nil, nil, err
	}

	if tp, _ := raw["CMapType"].(postscript.Integer); !(tp == 0 || tp == 1) {
		return nil, nil, pdf.Errorf("invalid CMapType: %d", tp)
	}

	res := &InfoNew{}
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

	codeMap, ok := raw["CodeMap"].(*postscript.CMapInfo)
	if !ok {
		return nil, nil, pdf.Error("unsupported CMap format")
	}
	if codeMap.UseCMap != "" {
		parent = pdf.Name(codeMap.UseCMap)
	}

	for _, entry := range codeMap.CodeSpaceRanges {
		res.CodeSpaceRange = append(res.CodeSpaceRange,
			charcode.Range{Low: entry.Low, High: entry.High})
	}

	for _, entry := range codeMap.CidChars {
		cid, ok := entry.Dst.(postscript.Integer)
		if !ok || cid < 0 || cid > 0xFFFF_FFFF {
			continue
		}
		res.CIDSingles = append(res.CIDSingles, SingleEntryNew{
			Code:  entry.Src,
			Value: CID(cid),
		})
	}
	for _, entry := range codeMap.CidRanges {
		cid, ok := entry.Dst.(postscript.Integer)
		if !ok || cid < 0 || cid > 0xFFFF_FFFF {
			continue
		}
		res.CIDRanges = append(res.CIDRanges, RangeEntryNew{
			First: entry.Low,
			Last:  entry.High,
			Value: CID(cid),
		})
	}
	for _, entry := range codeMap.NotdefChars {
		cid, ok := entry.Dst.(postscript.Integer)
		if !ok || cid < 0 || cid > 0xFFFF_FFFF {
			continue
		}
		res.NotdefSingles = append(res.NotdefSingles, SingleEntryNew{
			Code:  entry.Src,
			Value: CID(cid),
		})
	}
	for _, entry := range codeMap.NotdefRanges {
		cid, ok := entry.Dst.(postscript.Integer)
		if !ok || cid < 0 || cid > 0xFFFF_FFFF {
			continue
		}
		res.NotdefRanges = append(res.NotdefRanges, RangeEntryNew{
			First: entry.Low,
			Last:  entry.High,
			Value: CID(cid),
		})
	}

	if len(res.CIDSingles) == 0 && len(res.CIDRanges) == 0 && len(res.NotdefSingles) == 0 && len(res.NotdefRanges) == 0 {
		return nil, nil, pdf.Error("no CMAP entries found")
	}

	return res, parent, nil
}
