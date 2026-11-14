// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"maps"
	"slices"
	"sort"

	"seehuhn.de/go/pdf/font/charcode"
)

// NewToUnicodeFile creates a ToUnicodeFile object.
//
// An error is returned if [charcode.NewCodec] fails for the code space range:
// if the range is invalid, or if its lookup tree would be too large.
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

// GetMapping returns the mapping from character codes to text, for the
// codes valid in the code space range of tu, as reported by
// [ToUnicodeFile.All].  The error is that of [charcode.NewCodec].
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
