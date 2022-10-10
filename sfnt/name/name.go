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

// Package name has code for reading and wrinting OpenType "name" tables.
// These tables contain localized strings associated with a font.
// https://docs.microsoft.com/en-us/typography/opentype/spec/name
package name

import (
	"sort"
	"unicode/utf16"

	"seehuhn.de/go/pdf/sfnt/fonterror"
	"seehuhn.de/go/pdf/sfnt/mac"
)

// Info contains information from the "name" table.
type Info struct {
	Mac     Tables
	Windows Tables
}

// Decode extracts information from the "name" table.
func Decode(data []byte) (*Info, error) {
	if len(data) < 6 {
		return nil, errMalformedNames
	}
	version := uint16(data[0])<<8 | uint16(data[1])
	numRec := int(data[2])<<8 | int(data[3])
	storageOffset := int(data[4])<<8 | int(data[5])

	if version > 1 {
		// all fonts on my laptop use version 0 of the table
		return nil, errMalformedNames
	}

	recBase := 6
	endOfHeader := recBase + 12*numRec
	if endOfHeader > len(data) {
		return nil, errMalformedNames
	}

	numLang := 0
	if version > 0 {
		if endOfHeader+2 > len(data) {
			return nil, errMalformedNames
		}
		numLang = int(data[endOfHeader])<<8 | int(data[endOfHeader+1])
		endOfHeader += 2 + numLang*4
	}
	if storageOffset < endOfHeader || storageOffset > len(data) {
		return nil, errMalformedNames
	}

	macTables := make(map[string]*Table)
	msTables := make(map[string]*Table)

recLoop:
	for i := 0; i < numRec; i++ {
		pos := recBase + i*12
		platformID := uint16(data[pos])<<8 | uint16(data[pos+1])
		encodingID := uint16(data[pos+2])<<8 | uint16(data[pos+3])
		languageID := uint16(data[pos+4])<<8 | uint16(data[pos+5])
		nameID := ID(data[pos+6])<<8 | ID(data[pos+7])
		nameLen := int(data[pos+8])<<8 | int(data[pos+9])
		nameOffset := int(data[pos+10])<<8 | int(data[pos+11])

		// We only use records where we understand platformID and languageID.
		var key string
		switch platformID {
		case 1: // Macintosh
			key = appleBCP[languageID]
		case 3: // Windows
			key = msBCP[languageID]
		}
		if key == "" {
			continue
		}

		if storageOffset+nameOffset+nameLen > len(data) {
			return nil, errMalformedNames
		}
		nameBytes := data[storageOffset+nameOffset : storageOffset+nameOffset+nameLen]

		// We ignore encodings we don't understand.
		var val string
		if platformID == 3 && encodingID == 1 { // Windows, Unicode BMP
			val = utf16Decode(nameBytes)
		} else if platformID == 1 && encodingID == 0 { // Macintosh, Roman
			val = mac.Decode(nameBytes)
		}
		// TODO(voss): implement some more encodings
		// https://unicode.org/Public/MAPPINGS/VENDORS/APPLE/ReadMe.txt
		if val == "" {
			continue recLoop
		}

		switch platformID {
		case 1: // Macintosh
			t := macTables[key]
			if t == nil {
				t = &Table{}
			}
			t.set(nameID, val)
			macTables[key] = t
		case 3: // Windows
			t := msTables[key]
			if t == nil {
				t = &Table{}
			}
			t.set(nameID, val)
			msTables[key] = t
		}
	}

	res := &Info{
		Mac:     macTables,
		Windows: msTables,
	}

	return res, nil
}

// Encode converts a "name" table into its binary form.
func (info *Info) Encode(windowsEncodingID uint16) []byte {
	type recInfo struct {
		PlatformID uint16
		EncodingID uint16
		LanguageID uint16
		NameID     uint16
		offset     uint16
		length     uint16
	}
	var records []*recInfo

	b := newNameBuilder()

	// platform ID 1 (Macintosh)
	for languageID, tag := range appleBCP {
		t := info.Mac[tag]
		if t == nil {
			continue
		}
		for _, nameID := range t.keys() {
			val := t.get(nameID)
			offset, length := b.Add(mac.Encode(val))
			rec := &recInfo{
				PlatformID: 1, // Macintosh
				EncodingID: 0, // Roman
				LanguageID: languageID,
				NameID:     uint16(nameID),
				offset:     offset,
				length:     length,
			}
			records = append(records, rec)
		}
	}

	// Platform ID 3 (Windows).
	// Encoding IDs for platform 3 'name' entries must match the encoding IDs
	// used for platform 3 subtables in the 'cmap' table.
	for languageID, tag := range msBCP {
		t := info.Windows[tag]
		if t == nil {
			continue
		}
		for _, nameID := range t.keys() {
			val := t.get(nameID)
			// TODO(voss): implement support for different encodings
			offset, length := b.Add(utf16Encode(val))
			rec := &recInfo{
				PlatformID: 3, // Windows
				EncodingID: windowsEncodingID,
				LanguageID: languageID,
				NameID:     uint16(nameID),
				offset:     offset,
				length:     length,
			}
			records = append(records, rec)
		}
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].PlatformID != records[j].PlatformID {
			return records[i].PlatformID < records[j].PlatformID
		}
		if records[i].EncodingID != records[j].EncodingID {
			return records[i].EncodingID < records[j].EncodingID
		}
		if records[i].LanguageID != records[j].LanguageID {
			return records[i].LanguageID < records[j].LanguageID
		}
		return records[i].NameID < records[j].NameID
	})

	numRec := len(records)
	startOfRecords := 6
	startOfStrings := startOfRecords + numRec*12
	res := make([]byte, startOfStrings+len(b.data))

	res[2] = byte(numRec >> 8)
	res[3] = byte(numRec)
	res[4] = byte(startOfStrings >> 8)
	res[5] = byte(startOfStrings)
	for i := 0; i < numRec; i++ {
		rec := records[i]
		base := startOfRecords + i*12
		res[base] = byte(rec.PlatformID >> 8)
		res[base+1] = byte(rec.PlatformID)
		res[base+2] = byte(rec.EncodingID >> 8)
		res[base+3] = byte(rec.EncodingID)
		res[base+4] = byte(rec.LanguageID >> 8)
		res[base+5] = byte(rec.LanguageID)
		res[base+6] = byte(rec.NameID >> 8)
		res[base+7] = byte(rec.NameID)
		res[base+8] = byte(rec.length >> 8)
		res[base+9] = byte(rec.length)
		res[base+10] = byte(rec.offset >> 8)
		res[base+11] = byte(rec.offset)
	}
	copy(res[startOfStrings:], b.data)

	return res
}

type nameBuilder struct {
	data []byte
	idx  map[string]uint16
}

func newNameBuilder() *nameBuilder {
	return &nameBuilder{
		idx: make(map[string]uint16),
	}
}

func (nb *nameBuilder) Add(b []byte) (offs, length uint16) {
	key := string(b)
	if idx, ok := nb.idx[key]; ok {
		return idx, uint16(len(b))
	}
	idx := uint16(len(nb.data))
	nb.idx[key] = idx
	nb.data = append(nb.data, b...)
	return idx, uint16(len(b))
}

func utf16Encode(s string) []byte {
	rr := utf16.Encode([]rune(s))
	res := make([]byte, len(rr)*2)
	for i, r := range rr {
		res[i*2] = byte(r >> 8)
		res[i*2+1] = byte(r)
	}
	return res
}

func utf16Decode(buf []byte) string {
	var nameWords []uint16
	for i := 0; i+1 < len(buf); i += 2 {
		nameWords = append(nameWords, uint16(buf[i])<<8|uint16(buf[i+1]))
	}
	return string(utf16.Decode(nameWords))
}

var errMalformedNames = &fonterror.InvalidFontError{
	SubSystem: "sfnt/name",
	Reason:    "malformed name table",
}
