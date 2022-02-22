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

// Package name has code for reading and wrinting the "name" table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/name
package name

import (
	"fmt"
	"sort"
	"strings"

	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/mac"
	"seehuhn.de/go/pdf/locale"
)

const MaxNameID = 10 // TODO(voss)

// Info contains information from the "name" table.
type Info struct {
	Tables map[Loc]*Table
}

func (info *Info) SelectTable(lang locale.Language, country locale.Country) *Table {
	for key, t := range info.Tables {
		if key.Language == lang && key.Country == country {
			return t
		}
	}
	return nil
}

func (info *Info) selectLangTable(lang locale.Language) *Table {
	var candidates []Loc
	for key := range info.Tables {
		if key.Language == lang {
			candidates = append(candidates, key)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		// TODO(voss): use a better criterion here
		return candidates[i].Country < candidates[j].Country
	})
	return info.Tables[candidates[0]]
}

// Table contains the data for a single language
// https://docs.microsoft.com/en-us/typography/opentype/spec/name#name-ids
type Table struct {
	Copyright      string
	Family         string
	Subfamily      string
	Identifier     string
	FullName       string
	Version        string
	PostScriptName string
	Trademark      string
	Manufacturer   string
	Designer       string
	Description    string
	// VendorURL      *url.URL
	// DesignerURL    *url.URL
	// License        string
	// LicenseURL     *url.URL
}

func (t *Table) String() string {
	b := &strings.Builder{}
	fmt.Fprintf(b, "Copyright: %q\n", t.Copyright)
	fmt.Fprintf(b, "Family: %q\n", t.Family)
	fmt.Fprintf(b, "Subfamily: %q\n", t.Subfamily)
	fmt.Fprintf(b, "Identifier: %q\n", t.Identifier)
	fmt.Fprintf(b, "FullName: %q\n", t.FullName)
	fmt.Fprintf(b, "Version: %q\n", t.Version)
	fmt.Fprintf(b, "PostScriptName: %q\n", t.PostScriptName)
	fmt.Fprintf(b, "Trademark: %q\n", t.Trademark)
	fmt.Fprintf(b, "Manufacturer: %q\n", t.Manufacturer)
	fmt.Fprintf(b, "Designer: %q\n", t.Designer)
	fmt.Fprintf(b, "Description: %q\n", t.Description)
	return b.String()
}

func (t *Table) get(i int) string {
	switch i {
	case 0:
		return t.Copyright
	case 1:
		return t.Family
	case 2:
		return t.Subfamily
	case 3:
		return t.Identifier
	case 4:
		return t.FullName
	case 5:
		return t.Version
	case 6:
		return t.PostScriptName
	case 7:
		return t.Trademark
	case 8:
		return t.Manufacturer
	case 9:
		return t.Designer
	case 10:
		return t.Description
	default:
		return ""
	}
}

// Decode extracts information from the "name" table.
func Decode(data []byte) (*Info, error) {
	if len(data) < 6 {
		return nil, errMalformedNames
	}
	version := uint16(data[0])<<8 | uint16(data[1])
	if version > 1 {
		return nil, errMalformedNames
	}
	// all fonts on my laptop use version 0 of the table

	numRec := int(data[2])<<8 + int(data[3])
	storageOffset := int(data[4])<<8 + int(data[5])

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
		numLang = int(data[endOfHeader])<<8 + int(data[endOfHeader+1])
		endOfHeader += 2 + numLang*4
	}
	if storageOffset < endOfHeader || storageOffset > len(data) {
		return nil, errMalformedNames
	}

	tables := make(map[Loc]*Table)

recLoop:
	for i := 0; i < numRec; i++ {
		pos := recBase + i*12
		platformID := uint16(data[pos])<<8 | uint16(data[pos+1])
		encodingID := uint16(data[pos+2])<<8 | uint16(data[pos+3])
		languageID := uint16(data[pos+4])<<8 | uint16(data[pos+5])
		nameID := uint16(data[pos+6])<<8 | uint16(data[pos+7])
		nameLen := int(data[pos+8])<<8 | int(data[pos+9])
		nameOffset := int(data[pos+10])<<8 | int(data[pos+11])

		var key Loc
		switch platformID {
		case 1: // Macintosh
			key = appleCodes[languageID]
		case 3: // Windows
			key = microsoftCodes[languageID]
		}
		if key.Language == 0 {
			continue
		}

		if storageOffset+nameOffset+nameLen > len(data) {
			return nil, errMalformedNames
		}
		nameBytes := data[storageOffset+nameOffset : storageOffset+nameOffset+nameLen]

		var val string
		switch platformID {
		case 0, 3: // Unicode and Windows
			val = utf16Decode(nameBytes)
		case 1: // Macintosh
			if encodingID != 0 {
				// TODO(voss): implement some more encodings
				continue recLoop
			}
			val = mac.Decode(nameBytes)
		}
		if val == "" {
			continue recLoop
		}

		t := tables[key]
		if t == nil {
			t = &Table{}
		}
		switch nameID {
		case 0:
			t.Copyright = val
		case 1:
			t.Family = val
		case 2:
			t.Subfamily = val
		case 3:
			t.Identifier = val
		case 4:
			t.FullName = val
		case 5:
			t.Version = val
		case 6:
			t.PostScriptName = val
		case 7:
			t.Trademark = val
		case 8:
			t.Manufacturer = val
		case 9:
			t.Designer = val
		case 10:
			t.Description = val
		default:
			continue recLoop
		}
		tables[key] = t
	}

	complete := make(map[locale.Language]bool)
	for key := range tables {
		if key.Language != 0 && key.Country != 0 {
			complete[key.Language] = true
		}
	}

	for key := range tables {
		if key.Country == 0 && complete[key.Language] {
			delete(tables, key)
		}
	}
	res := &Info{
		Tables: tables,
	}

	return res, nil
}

// Encode converts a "name" table into its binary form.
func (info *Info) Encode(ss cmap.Subtables) []byte {
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
	includeMac := false
	for i := range ss {
		if ss[i].PlatformID == 1 {
			includeMac = true
			break
		}
	}
	if includeMac {
		for languageID, loc := range appleCodes {
			t := info.selectLangTable(loc.Language)
			if t == nil {
				continue
			}
			for nameID := 0; nameID <= MaxNameID; nameID++ {
				val := t.get(nameID)
				if val == "" {
					continue
				}

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
	}

	// Platform ID 3 (Windows).
	// Encoding IDs for platform 3 'name' entries should match the encoding IDs
	// used for platform 3 subtables in the 'cmap' table.
	encodingIDs := make(map[uint16]bool)
	for i := range ss {
		if ss[i].PlatformID == 3 {
			encodingIDs[ss[i].EncodingID] = true
		}
	}
	if len(encodingIDs) == 0 {
		encodingIDs[1] = true
	}
	for languageID, loc := range microsoftCodes {
		t := info.SelectTable(loc.Language, loc.Country)
		if t == nil {
			continue
		}

		for nameID := 0; nameID <= MaxNameID; nameID++ {
			val := t.get(nameID)
			if val == "" {
				continue
			}
			offset, length := b.Add(utf16Encode(val))

			for encodingID := range encodingIDs {
				rec := &recInfo{
					PlatformID: 3, // Windows
					EncodingID: encodingID,
					LanguageID: languageID,
					NameID:     uint16(nameID),
					offset:     offset,
					length:     length,
				}
				records = append(records, rec)
			}
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

var errMalformedNames = fmt.Errorf("malformed name table")
