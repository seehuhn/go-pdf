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
	var nameRunes []rune
recLoop:
	for i := 0; i < numRec; i++ {
		pos := recBase + i*12
		platformID := uint16(data[pos])<<8 | uint16(data[pos+1])
		encodingID := uint16(data[pos+2])<<8 | uint16(data[pos+3])
		languageID := uint16(data[pos+4])<<8 | uint16(data[pos+5])
		nameID := uint16(data[pos+6])<<8 | uint16(data[pos+7])
		nameLen := int(data[pos+8])<<8 | int(data[pos+9])
		nameOffset := int(data[pos+10])<<8 | int(data[pos+11])
		if storageOffset+nameOffset+nameLen > len(data) {
			return nil, errMalformedNames
		}
		nameBytes := data[storageOffset+nameOffset : storageOffset+nameOffset+nameLen]

		var name string
		switch platformID {
		case 0, 3: // Unicode and Windows
			nameWords := nameWords[:0]
			for i := 0; i+1 < nameLen; i += 2 {
				nameWords = append(nameWords, uint16(nameBytes[i])<<8|uint16(nameBytes[i+1]))
			}
			name = string(utf16.Decode(nameWords))

		case 1: // Macintosh
			if encodingID != 0 {
				continue recLoop
			}
			nameRunes = nameRunes[:0]
			for i := 0; i < nameLen; i++ {
				nameRunes = append(nameRunes, mac.Roman[nameBytes[i]])
			}
			name = string(nameRunes)

		default:
			continue recLoop
		}

		var key loc
		switch platformID {
		case 1: // Macintosh
			key = appleCodes[languageID]
		case 3: // Windows
			key = microsoftCodes[languageID]
		}

		if _, ok := tables[key]; !ok {
			tables[key] = &Table{}
		}
		switch nameID {
		case 0:
			tables[key].Copyright = name
		case 1:
			tables[key].Family = name
		case 2:
			tables[key].Subfamily = name
		case 3:
			tables[key].Identifier = name
		case 4:
			tables[key].FullName = name
		case 5:
			tables[key].Version = name
		case 6:
			tables[key].PostScriptName = name
		case 7:
			tables[key].Trademark = name
		case 8:
			tables[key].Manufacturer = name
		case 9:
			tables[key].Designer = name
		case 10:
			tables[key].Description = name
		}
	}

	for key := range tables {
		fmt.Println(key.lang, key.country)
	}
	fmt.Println()

	return nil, nil
}

var errMalformedNames = fmt.Errorf("malformed name table")
