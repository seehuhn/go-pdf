// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"maps"
	"slices"
	"sort"

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
	keys := slices.SortedFunc(maps.Keys(ranges), func(a, b string) int {
		return slices.Compare([]byte(a), []byte(b))
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
