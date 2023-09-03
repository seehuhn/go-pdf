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
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"slices"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
)

// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf

type Info struct {
	Name     string
	Version  float64
	ROS      *type1.CIDSystemInfo
	CS       charcode.CodeSpaceRange
	CSFile   charcode.CodeSpaceRange
	WMode    int
	UseCMap  string
	Singles  []Single
	Ranges   []Range
	Comments bool
}

// Single specifies that character code Code represents the given CID.
type Single struct {
	Code  charcode.CharCode
	Value type1.CID
}

// Range describes a range of character codes with consecutive CIDs.
// First and Last are the first and last code points in the range.
// Value is the CID of the first code point in the range.
type Range struct {
	First charcode.CharCode
	Last  charcode.CharCode
	Value type1.CID
}

func New(ROS *type1.CIDSystemInfo, cs charcode.CodeSpaceRange, m map[charcode.CharCode]type1.CID) *Info {
	res := &Info{
		ROS:    ROS,
		CS:     cs,
		CSFile: cs,
	}
	res.SetMapping(m)

	// TODO(voss): check whether any of the predefined CMaps can be used.

	if res.IsIdentity() {
		res.Name = "Identity-H"
	} else {
		res.Name = makeName(m)
	}

	return res
}

// IsIdentity returns true if all codes are equal to the corresponding CID.
func (info *Info) IsIdentity() bool {
	for _, s := range info.Singles {
		if int(s.Code) != int(s.Value) {
			return false
		}
	}
	for _, r := range info.Ranges {
		if int(r.First) != int(r.Value) {
			return false
		}
	}
	return true
}

func (info *Info) Embed(w pdf.Putter, ref pdf.Reference, other map[string]pdf.Reference) error {
	dict := pdf.Dict{
		"Type":     pdf.Name("CMap"),
		"CMapName": pdf.Name(info.Name),
		"CIDSystemInfo": pdf.Dict{
			"Registry":   pdf.String(info.ROS.Registry),
			"Ordering":   pdf.String(info.ROS.Ordering),
			"Supplement": pdf.Integer(info.ROS.Supplement),
		},
	}
	if info.WMode != 0 {
		dict["WMode"] = pdf.Integer(info.WMode)
	}
	if info.UseCMap != "" {
		var useCMap pdf.Object
		isInOther := false
		if other != nil {
			_, isInOther = other[info.UseCMap]
		}
		if _, ok := builtinCS[info.UseCMap]; ok {
			useCMap = pdf.Name(info.UseCMap)
		} else if isInOther {
			useCMap = other[info.UseCMap]
		} else {
			return fmt.Errorf("unknown CMap %q", info.UseCMap)
		}
		dict["UseCMap"] = useCMap
	}
	stm, err := w.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	err = info.Write(stm)
	if err != nil {
		return fmt.Errorf("embedding cmap: %w", err)
	}
	err = stm.Close()
	if err != nil {
		return err
	}
	return nil
}

func makeName(m map[charcode.CharCode]type1.CID) string {
	codes := maps.Keys(m)
	slices.Sort(codes)
	h := sha256.New()
	for _, k := range codes {
		binary.Write(h, binary.BigEndian, uint32(k))
		binary.Write(h, binary.BigEndian, uint32(m[k]))
	}
	sum := h.Sum(nil)
	return fmt.Sprintf("Seehuhn-%x", sum[:8])
}
