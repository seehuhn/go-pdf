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
	"strings"
	"unicode/utf16"

	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/mac"
	"seehuhn.de/go/pdf/locale"
)

// Info contains information from the "name" table.
type Info struct {
	Tables []*Table
}

// Table contains the data for a single language
// https://docs.microsoft.com/en-us/typography/opentype/spec/name#name-ids
type Table struct {
	Language locale.Language
	Country  locale.Country

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
	fmt.Fprintf(b, "Language: %s\n", t.Language.Name())
	fmt.Fprintf(b, "Country: %s\n", t.Country.Name())
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

	tables := make(map[loc]*Table)

	var nameWords []uint16
recLoop:
	for i := 0; i < numRec; i++ {
		pos := recBase + i*12
		platformID := uint16(data[pos])<<8 | uint16(data[pos+1])
		encodingID := uint16(data[pos+2])<<8 | uint16(data[pos+3])
		languageID := uint16(data[pos+4])<<8 | uint16(data[pos+5])
		nameID := uint16(data[pos+6])<<8 | uint16(data[pos+7])
		nameLen := int(data[pos+8])<<8 | int(data[pos+9])
		nameOffset := int(data[pos+10])<<8 | int(data[pos+11])

		var key loc
		switch platformID {
		case 1: // Macintosh
			key = appleCodes[languageID]
		case 3: // Windows
			key = microsoftCodes[languageID]
		}
		if key.lang == 0 {
			continue
		}

		if storageOffset+nameOffset+nameLen > len(data) {
			return nil, errMalformedNames
		}
		nameBytes := data[storageOffset+nameOffset : storageOffset+nameOffset+nameLen]

		var val string
		switch platformID {
		case 0, 3: // Unicode and Windows
			nameWords := nameWords[:0]
			for i := 0; i+1 < nameLen; i += 2 {
				nameWords = append(nameWords, uint16(nameBytes[i])<<8|uint16(nameBytes[i+1]))
			}
			val = string(utf16.Decode(nameWords))

		case 1: // Macintosh
			if encodingID != 0 {
				// TODO(voss): implement some more encodings
				continue recLoop
			}
			val = mac.Decode(nameBytes)

		default:
			continue recLoop
		}

		if _, ok := tables[key]; !ok {
			tables[key] = &Table{}
		}
		switch nameID {
		case 0:
			tables[key].Copyright = val
		case 1:
			tables[key].Family = val
		case 2:
			tables[key].Subfamily = val
		case 3:
			tables[key].Identifier = val
		case 4:
			tables[key].FullName = val
		case 5:
			tables[key].Version = val
		case 6:
			tables[key].PostScriptName = val
		case 7:
			tables[key].Trademark = val
		case 8:
			tables[key].Manufacturer = val
		case 9:
			tables[key].Designer = val
		case 10:
			tables[key].Description = val
		}
	}

	complete := make(map[locale.Language]bool)
	for key := range tables {
		if key.lang != 0 && key.country != 0 {
			complete[key.lang] = true
		}
	}

	res := &Info{}
	for key, table := range tables {
		if key.country == 0 && complete[key.lang] {
			continue
		}
		table.Language = key.lang
		table.Country = key.country
		res.Tables = append(res.Tables, table)
	}

	return res, nil
}

func (info *Info) Encode(ss cmap.Subtables) []byte {
	type record struct {
		PlatformID uint16
		EncodingID uint16
		LanguageID uint16
		enc        func(string) []byte
	}
	var records []*record

	// platform ID 1 (Macintosh)
	includeMac := false
	for i := range ss {
		if ss[i].PlatformID == 1 {
			includeMac = true
			break
		}
	}
	if includeMac {
		haveLang := make(map[locale.Language]bool)
		for _, t := range info.Tables {
			haveLang[t.Language] = true
		}
		for languageID, loc := range appleCodes {
			if haveLang[loc.lang] {
				rec := &record{
					PlatformID: 1,
					EncodingID: 0,
					LanguageID: languageID,
					enc:        mac.Encode,
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

	panic("not implemented")
}

var errMalformedNames = fmt.Errorf("malformed name table")
