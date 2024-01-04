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
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
)

// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf

// Info holds the information for a PDF CMap.
type Info struct {
	Name string
	ROS  *type1.CIDSystemInfo
	charcode.CodeSpaceRange
	CSFile  charcode.CodeSpaceRange // TODO(voss): remove this
	WMode   int
	UseCMap string
	Singles []SingleEntry
	Ranges  []RangeEntry
}

// SingleEntry specifies that character code Code represents the given CID.
type SingleEntry struct {
	Code  charcode.CharCode
	Value type1.CID
}

// RangeEntry describes a range of character codes with consecutive CIDs.
// First and Last are the first and last code points in the range.
// Value is the CID of the first code point in the range.
type RangeEntry struct {
	First charcode.CharCode
	Last  charcode.CharCode
	Value type1.CID
}

// New allocates a new CMap object.
func New(ROS *type1.CIDSystemInfo, cs charcode.CodeSpaceRange, m map[charcode.CharCode]type1.CID) *Info {
	info := &Info{
		ROS:            ROS,
		CodeSpaceRange: cs,
		CSFile:         cs,
	}
	info.SetMapping(m)

	if info.IsIdentity() {
		if info.WMode == 0 {
			info.Name = "Identity-H"
		} else {
			info.Name = "Identity-V"
		}
	} else {
		info.Name = makeName(m)
	}

	// TODO(voss): check whether any of the other predefined CMaps can be used.

	return info
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

// MaxCID returns the largest CID used by this CMap.
func (info *Info) MaxCID() type1.CID {
	var maxCID type1.CID
	for _, s := range info.Singles {
		if s.Value > maxCID {
			maxCID = s.Value
		}
	}
	for _, r := range info.Ranges {
		rangeMax := r.Value + type1.CID(r.Last-r.First)
		if rangeMax > maxCID {
			maxCID = rangeMax
		}
	}
	return maxCID
}

func makeName(m map[charcode.CharCode]type1.CID) string {
	codes := maps.Keys(m)
	slices.Sort(codes)
	h := sha256.New()
	h.Write([]byte("seehuhn.de/go/pdf/font/cmap.makeName\n"))
	for _, k := range codes {
		binary.Write(h, binary.BigEndian, uint32(k))
		binary.Write(h, binary.BigEndian, uint32(m[k]))
	}
	sum := h.Sum(nil)
	return fmt.Sprintf("Seehuhn-%x", sum[:8])
}
