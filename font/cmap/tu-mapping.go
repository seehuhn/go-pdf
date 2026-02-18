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
	"maps"
	"slices"
	"sort"

	"seehuhn.de/go/pdf/font/charcode"
)

// NewToUnicodeFile creates a ToUnicodeFile object.
//
// The only error returned is if the CodeSpaceRange is invalid.
func NewToUnicodeFile(csr charcode.CodeSpaceRange, data map[charcode.Code]string) (*ToUnicodeFile, error) {
	res := &ToUnicodeFile{
		CodeSpaceRange: csr,
	}

	// group together codes which only differ in the last byte
	codec, err := charcode.NewCodec(csr)
	if err != nil {
		return nil, err
	}
	type entry struct {
		code charcode.Code
		x    byte
	}
	ranges := make(map[string][]entry)
	var buf []byte
	for code := range data {
		buf = codec.AppendCode(buf[:0], code)
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
			if i == len(info) || info[i].x != info[i-1].x+1 {
				first := make([]byte, len(key)+1)
				copy(first, key)
				first[len(key)] = info[start].x
				if i-start > 1 {
					last := make([]byte, len(key)+1)
					copy(last, key)
					last[len(key)] = info[i-1].x

					needsList := false
					for j := start; j < i-1; j++ {
						if data[info[j+1].code] != nextString(data[info[j].code], 1) {
							needsList = true
							break
						}
					}

					var values []string
					if needsList {
						values = make([]string, i-start)
						for j := start; j < i; j++ {
							values[j-start] = data[info[j].code]
						}
					} else {
						values = []string{data[info[start].code]}
					}

					res.Ranges = append(res.Ranges, ToUnicodeRange{
						First:  first,
						Last:   last,
						Values: values,
					})
				} else {
					res.Singles = append(res.Singles, ToUnicodeSingle{
						Code:  first,
						Value: data[info[start].code],
					})
				}
				start = i
			}
		}
	}

	return res, nil
}

func (tu *ToUnicodeFile) GetMapping() (map[charcode.Code]string, error) {
	codec, err := charcode.NewCodec(tu.CodeSpaceRange)
	if err != nil {
		return nil, err
	}

	return maps.Collect(tu.All(codec)), nil
}

func nextString(s string, inc int) string {
	rr := []rune(s)
	if len(rr) == 0 {
		return ""
	}
	rr[len(rr)-1] += rune(inc)
	return string(rr)
}
