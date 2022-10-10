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

package name

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
)

// ID encodes the meaning of a given name string.
// https://learn.microsoft.com/en-us/typography/opentype/spec/name#name-ids
type ID uint16

const maxID ID = 25

// Table contains the name table data for a single language
type Table struct {
	Copyright                string
	Family                   string
	Subfamily                string
	Identifier               string
	FullName                 string
	Version                  string
	PostScriptName           string
	Trademark                string
	Manufacturer             string
	Designer                 string
	Description              string
	VendorURL                string
	DesignerURL              string
	License                  string
	LicenseURL               string
	TypographicFamily        string
	TypographicSubfamily     string
	MacFullName              string
	SampleText               string
	CIDFontName              string
	WWSFamily                string
	WWSSubfamily             string
	LightBackgroundPalette   string
	DarkBackgroundPalette    string
	VariationsPostScriptName string

	Extra map[ID]string
}

func (t *Table) String() string {
	b := &strings.Builder{}
	if t.Family != "" {
		fmt.Fprintf(b, "Family: %q\n", t.Family)
	}
	if t.Subfamily != "" {
		fmt.Fprintf(b, "Subfamily: %q\n", t.Subfamily)
	}
	if t.TypographicFamily != "" {
		fmt.Fprintf(b, "TypographicFamily: %q\n", t.TypographicFamily)
	}
	if t.TypographicSubfamily != "" {
		fmt.Fprintf(b, "TypographicSubfamily: %q\n", t.TypographicSubfamily)
	}
	if t.WWSFamily != "" {
		fmt.Fprintf(b, "WWSFamily: %q\n", t.WWSFamily)
	}
	if t.WWSSubfamily != "" {
		fmt.Fprintf(b, "WWSSubfamily: %q\n", t.WWSSubfamily)
	}
	if t.Identifier != "" {
		fmt.Fprintf(b, "Identifier: %q\n", t.Identifier)
	}
	if t.FullName != "" {
		fmt.Fprintf(b, "FullName: %q\n", t.FullName)
	}
	if t.Version != "" {
		fmt.Fprintf(b, "Version: %q\n", t.Version)
	}
	if t.PostScriptName != "" {
		fmt.Fprintf(b, "PostScriptName: %q\n", t.PostScriptName)
	}
	if t.Trademark != "" {
		fmt.Fprintf(b, "Trademark: %q\n", t.Trademark)
	}
	if t.Manufacturer != "" {
		fmt.Fprintf(b, "Manufacturer: %q\n", t.Manufacturer)
	}
	if t.Description != "" {
		fmt.Fprintf(b, "Designer: %q\n", t.Designer)
	}
	if t.Description != "" {
		fmt.Fprintf(b, "Description: %q\n", t.Description)
	}
	if t.VendorURL != "" {
		fmt.Fprintf(b, "VendorURL: %s\n", t.VendorURL)
	}
	if t.DesignerURL != "" {
		fmt.Fprintf(b, "DesignerURL: %s\n", t.DesignerURL)
	}
	if t.Copyright != "" {
		fmt.Fprintf(b, "Copyright: %q\n", t.Copyright)
	}
	if t.License != "" {
		fmt.Fprintf(b, "License: %q\n", t.License)
	}
	if t.LicenseURL != "" {
		fmt.Fprintf(b, "LicenseURL: %s\n", t.LicenseURL)
	}
	if t.MacFullName != "" {
		fmt.Fprintf(b, "MacFullName: %q\n", t.MacFullName)
	}
	if t.SampleText != "" {
		fmt.Fprintf(b, "SampleText: %q\n", t.SampleText)
	}
	if t.CIDFontName != "" {
		fmt.Fprintf(b, "CIDFontName: %q\n", t.CIDFontName)
	}
	if t.LightBackgroundPalette != "" {
		fmt.Fprintf(b, "LightBackgroundPalette: %q\n", t.LightBackgroundPalette)
	}
	if t.DarkBackgroundPalette != "" {
		fmt.Fprintf(b, "DarkBackgroundPalette: %q\n", t.DarkBackgroundPalette)
	}
	if t.VariationsPostScriptName != "" {
		fmt.Fprintf(b, "VariationsPostScriptName: %q\n", t.VariationsPostScriptName)
	}

	if t.Extra != nil {
		keys := maps.Keys(t.Extra)
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] < keys[j]
		})
		for _, nameID := range keys {
			fmt.Fprintf(b, "%d: %q\n", nameID, t.VariationsPostScriptName)
		}
	}

	return b.String()
}

func (t *Table) get(nameID ID) string {
	switch nameID {
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
	case 11:
		return t.VendorURL
	case 12:
		return t.DesignerURL
	case 13:
		return t.License
	case 14:
		return t.LicenseURL
	case 16:
		return t.TypographicFamily
	case 17:
		return t.TypographicSubfamily
	case 18:
		return t.MacFullName
	case 19:
		return t.SampleText
	case 20:
		return t.CIDFontName
	case 21:
		return t.WWSFamily
	case 22:
		return t.WWSSubfamily
	case 23:
		return t.LightBackgroundPalette
	case 24:
		return t.DarkBackgroundPalette
	case 25:
		return t.VariationsPostScriptName
	default:
		if t.Extra != nil {
			return t.Extra[nameID]
		}
		return ""
	}
}

func (t *Table) set(nameID ID, val string) {
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
	case 11:
		t.VendorURL = val
	case 12:
		t.DesignerURL = val
	case 13:
		t.License = val
	case 14:
		t.LicenseURL = val
	case 16:
		t.TypographicFamily = val
	case 17:
		t.TypographicSubfamily = val
	case 18:
		t.MacFullName = val
	case 19:
		t.SampleText = val
	case 20:
		t.CIDFontName = val
	case 21:
		t.WWSFamily = val
	case 22:
		t.WWSSubfamily = val
	case 23:
		t.LightBackgroundPalette = val
	case 24:
		t.DarkBackgroundPalette = val
	case 25:
		t.VariationsPostScriptName = val
	default:
		if t.Extra == nil {
			t.Extra = map[ID]string{}
		}
		t.Extra[nameID] = val
	}
}

func (t *Table) keys() []ID {
	var res []ID
	for nameID := ID(0); nameID <= maxID; nameID++ {
		val := t.get(nameID)
		if val != "" {
			res = append(res, nameID)
		}
	}
	if t.Extra != nil {
		for nameID, val := range t.Extra {
			if val != "" && nameID > maxID {
				res = append(res, nameID)
			}
		}
		sort.Slice(res, func(i, j int) bool {
			return res[i] < res[j]
		})
	}
	return res
}
