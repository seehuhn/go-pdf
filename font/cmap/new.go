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
	"errors"
	"io"

	"seehuhn.de/go/pdf"
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

	CIDSingles    []SingleEntry
	CIDRanges     []RangeEntry
	NotdefSingles []SingleEntry
	NotdefRanges  []RangeEntry
}

func ExtractCMap(r pdf.Getter, obj pdf.Object) (*InfoNew, error) {
	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}
	switch obj := obj.(type) {
	case pdf.Name:
		r, err := openPredefined(string(obj))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return readCMap(r)

	case *pdf.Stream:
		err := pdf.CheckDictType(r, obj.Dict, "CMap")
		if err != nil {
			return nil, err
		}

		body, err := pdf.DecodeStream(r, obj, 0)
		if err != nil {
			return nil, err
		}
		res, err := readCMap(body)
		if err != nil {
			return nil, err
		}

		if name, _ := pdf.GetName(r, obj.Dict["CMapName"]); name != "" {
			res.Name = name
		}
		if ros, _ := ExtractCIDSystemInfo(r, obj.Dict["CIDSystemInfo"]); ros != nil {
			res.ROS = ros
		}
		if x, _ := pdf.GetInteger(r, obj.Dict["WMode"]); x == 1 {
			res.WMode = Vertical
		}
		if parent, _ := ExtractCMap(r, obj.Dict["UseCMap"]); parent != nil {
			res.Parent = parent
		}
		return res, nil

	default:
		return nil, errInvalidCMap
	}
}

func readCMap(r io.Reader) (*InfoNew, error) {
	panic("not implemented")
}

var (
	errInvalidCMap = &pdf.MalformedFileError{
		Err: errors.New("invalid CMap object"),
	}
)
