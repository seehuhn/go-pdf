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
	"seehuhn.de/go/postscript/cid"
)

// FileOld holds the information for a PDF CMap.
type FileOld struct {
	Name string
	ROS  *CIDSystemInfo
	charcode.CodeSpaceRange
	CSFile  charcode.CodeSpaceRange // TODO(voss): remove this
	WMode   int
	UseCMap string
	Singles []SingleOld
	Ranges  []RangeOld
}

// SingleOld specifies that character code Code represents the given CID.
type SingleOld struct {
	Code  charcode.CharCodeOld
	Value cid.CID
}

// RangeOld describes a range of character codes with consecutive CIDs.
// First and Last are the first and last code points in the range.
// Value is the CID of the first code point in the range.
type RangeOld struct {
	First charcode.CharCodeOld
	Last  charcode.CharCodeOld
	Value cid.CID
}

// FromMapOld allocates a new CMap object.
func FromMapOld(ROS *CIDSystemInfo, cs charcode.CodeSpaceRange, m map[charcode.CharCodeOld]cid.CID) *FileOld {
	info := &FileOld{
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
func (info *FileOld) IsIdentity() bool {
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
func (info *FileOld) MaxCID() cid.CID {
	var maxCID cid.CID
	for _, s := range info.Singles {
		if s.Value > maxCID {
			maxCID = s.Value
		}
	}
	for _, r := range info.Ranges {
		rangeMax := r.Value + cid.CID(r.Last-r.First)
		if rangeMax > maxCID {
			maxCID = rangeMax
		}
	}
	return maxCID
}

func makeName(m map[charcode.CharCodeOld]cid.CID) string {
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
