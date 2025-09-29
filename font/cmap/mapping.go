// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"slices"
	"sort"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/cid"
)

// SetMapping updates a cmap File with a new mapping.
// This replaces the CodeSpaceRange and CIDSingles/CIDRanges fields.
//
// Codes which are already correctly set in a parent cmap are not included in
// the new mapping.
func (f *File) SetMapping(codec *charcode.Codec, data map[charcode.Code]cid.CID) {
	f.CodeSpaceRange = codec.CodeSpaceRange()
	f.CIDSingles = nil
	f.CIDRanges = nil

	// group together codes which only differ in the last byte
	type entry struct {
		code charcode.Code
		x    byte
	}
	ranges := make(map[string][]entry)
	var buf []byte
	for code, cid := range data {
		buf = codec.AppendCode(buf[:0], code)
		if f.Parent != nil {
			parentCID := f.Parent.LookupCID(buf)
			if parentCID == cid {
				continue
			}
		}
		l := len(buf)
		key := string(buf[:l-1])
		ranges[key] = append(ranges[key], entry{code, buf[l-1]})
	}

	// find all ranges, in sorted order
	keys := maps.Keys(ranges)
	sort.Slice(keys, func(i, j int) bool {
		return slices.Compare([]byte(keys[i]), []byte(keys[j])) < 0
	})

	// for each range, add the required CIDRanges and CIDSingles
	for _, key := range keys {
		info := ranges[key]
		sort.Slice(info, func(i, j int) bool {
			return info[i].x < info[j].x
		})

		start := 0
		for i := 1; i <= len(info); i++ {
			if i == len(info) ||
				info[i].x != info[i-1].x+1 ||
				data[info[i].code] != data[info[i-1].code]+1 {
				first := make([]byte, len(key)+1)
				copy(first, key)
				first[len(key)] = info[start].x
				if i-start > 1 {
					last := make([]byte, len(key)+1)
					copy(last, key)
					last[len(key)] = info[i-1].x
					f.CIDRanges = append(f.CIDRanges, Range{
						First: first,
						Last:  last,
						Value: data[info[start].code],
					})
				} else {
					f.CIDSingles = append(f.CIDSingles, Single{
						Code:  first,
						Value: data[info[start].code],
					})
				}
				start = i
			}
		}
	}
}
