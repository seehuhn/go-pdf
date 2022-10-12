// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

// Package cmap reads and writes "cmap" tables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap
package cmap

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"

	"golang.org/x/exp/slices"
	"seehuhn.de/go/pdf/sfnt/mac"
)

// Key selects a subtable of a cmap table.
type Key struct {
	PlatformID uint16 // Platform ID.
	EncodingID uint16 // Platform-specific encoding ID.
	Language   uint16
}

// Table contains all subtables from a cmap table.
type Table map[Key][]byte

// Decode returns all subtables of the given "cmap" table.
// The returned subtables are guaranteed to be at least 10 bytes long
// and to have a valid format value (0, 2, 4, 6, 8, 10, 12, 13 or 14)
// in the first two bytes.
func Decode(data []byte) (Table, error) {
	const minLength = 10 // length of an empty format 6 subtable

	if len(data) < 4 || len(data) > math.MaxUint32 {
		return nil, errMalformedTable
	}
	version := uint16(data[0])<<8 | uint16(data[1])
	if version != 0 {
		return nil, fmt.Errorf("cmap: unknown table version %d", version)
	}
	numTables := int(data[2])<<8 | int(data[3])
	if len(data) < 4+8*numTables {
		return nil, errMalformedTable
	}

	endOfHeader := uint32(4 + 8*numTables)
	endOfData := uint32(len(data))

	type seg struct {
		start, end uint32
	}
	var segs []seg

	res := make(Table)
	for i := 0; i < numTables; i++ {
		platformID := uint16(data[4+i*8])<<8 | uint16(data[5+i*8])
		if platformID > 4 {
			return nil, errMalformedTable
		}
		encodingID := uint16(data[6+i*8])<<8 | uint16(data[7+i*8])

		o := uint32(data[8+i*8])<<24 |
			uint32(data[9+i*8])<<16 |
			uint32(data[10+i*8])<<8 |
			uint32(data[11+i*8])
		if o < endOfHeader || o > endOfData-minLength {
			return nil, errMalformedTable
		}

		var language uint16
		var length uint32
		format := uint16(data[o])<<8 | uint16(data[o+1])
		checkLength := uint32(minLength)
		switch format {
		case 0, 2, 4, 6:
			length = uint32(data[o+2])<<8 | uint32(data[o+3])
			language = uint16(data[o+4])<<8 | uint16(data[o+5])
		case 8, 10, 12, 13:
			checkLength = 12
			if o > endOfData-checkLength {
				return nil, errMalformedTable
			}
			length = uint32(data[o+4])<<24 |
				uint32(data[o+5])<<16 |
				uint32(data[o+6])<<8 |
				uint32(data[o+7])
			language = uint16(data[o+10])<<8 | uint16(data[o+11])
		case 14:
			length = uint32(data[o+2])<<24 |
				uint32(data[o+3])<<16 |
				uint32(data[o+4])<<8 |
				uint32(data[o+5])
		default:
			return nil, errMalformedTable
		}
		if length < checkLength || length > endOfData-o {
			return nil, errMalformedTable
		}

		if platformID != 1 {
			language = 0
		}

		// check that subtables are either disjoint or identical
		idx := sort.Search(len(segs), func(i int) bool {
			return o <= segs[i].start
		})
		if idx == len(segs) || o != segs[idx].start {
			if idx > 0 && o < segs[idx-1].end ||
				idx < len(segs) && o+length > segs[idx].start {
				return nil, errMalformedTable
			}
			segs = slices.Insert(segs, idx, seg{o, o + length})
		}

		key := Key{
			PlatformID: platformID,
			EncodingID: encodingID,
			Language:   language,
		}
		res[key] = data[o : o+length]
	}

	return res, nil
}

// Encode converts the cmap table into binary form.
func (ss Table) Encode() []byte {
	type extended struct {
		Data []byte
		Offs uint32
		Key
	}
	ext := make([]extended, 0, len(ss))
	for key, data := range ss {
		ext = append(ext, extended{
			Data: data,
			Key:  key,
		})
	}
	sort.Slice(ext, func(i, j int) bool {
		if ext[i].PlatformID != ext[j].PlatformID {
			return ext[i].PlatformID < ext[j].PlatformID
		}
		if ext[i].EncodingID != ext[j].EncodingID {
			return ext[i].EncodingID < ext[j].EncodingID
		}
		return ext[i].Language < ext[j].Language
	})

	numTables := len(ext)
	endOfHeader := uint32(4 + 8*numTables)

	pos := endOfHeader
offsLoop:
	for i, e := range ext {
		for j := 0; j < i; j++ {
			if bytes.Equal(e.Data, ext[j].Data) {
				ext[i].Offs = ext[j].Offs
				ext[i].Data = nil
				continue offsLoop
			}
		}
		ext[i].Offs = pos
		pos += uint32(len(e.Data))
	}

	res := make([]byte, endOfHeader, pos)
	// header[0] = 0
	// header[1] = 0
	res[2] = byte(numTables >> 8)
	res[3] = byte(numTables)
	for i, e := range ext {
		res[4+i*8] = byte(e.PlatformID >> 8)
		res[5+i*8] = byte(e.PlatformID)
		res[6+i*8] = byte(e.EncodingID >> 8)
		res[7+i*8] = byte(e.EncodingID)
		res[8+i*8] = byte(e.Offs >> 24)
		res[9+i*8] = byte(e.Offs >> 16)
		res[10+i*8] = byte(e.Offs >> 8)
		res[11+i*8] = byte(e.Offs)
	}
	for _, e := range ext {
		res = append(res, e.Data...)
	}

	return res
}

// Get decodes the given cmap subtable.
func (ss Table) Get(key Key) (Subtable, error) {
	data, ok := ss[key]
	if !ok {
		return nil, errors.New("cmap: no such subtable")
	}

	macRoman := func(code int) rune {
		return mac.DecodeOne(byte(code))
	}

	var code2rune func(int) rune
	if key.PlatformID == 1 {
		if key.EncodingID != 0 {
			return nil, errors.New("cmap: unsupported Mac encoding")
		}
		code2rune = macRoman
	}

	format := uint16(data[0])<<8 | uint16(data[1])
	decode := decoders[format]
	return decode(data, code2rune)
}

// GetBest selects the "best" subtable from a cmap table.
func (ss Table) GetBest() (Subtable, error) {
	candidates := []struct {
		PlatformID uint16
		EncodingID uint16
	}{
		{3, 10}, // full unicode
		{0, 4},
		{3, 1}, // BMP
		{0, 3},
		{1, 0}, // vintage Apple format
	}

	for _, c := range candidates {
		if sub, err := ss.Get(Key{c.PlatformID, c.EncodingID, 0}); err == nil {
			return sub, nil
		}
	}
	return nil, errors.New("cmap: no suitable subtable found")
}

var (
	errMalformedTable        = errors.New("cmap: malformed table")
	errMalformedSubtable     = errors.New("cmap: malformed subtable")
	errUnsupportedCmapFormat = errors.New("unsupported cmap format")
)
