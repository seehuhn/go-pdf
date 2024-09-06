// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package pdf

import (
	"strconv"
)

// MetaInfo represents the meta information of a PDF file.
type MetaInfo struct {
	// Version is the PDF version used in this file.
	Version Version

	// The ID of the file.  This is either a slice of two byte slices (the
	// original ID of the file, and the ID of the current version), or nil if
	// the file does not specify an ID.
	ID [][]byte

	// Catalog is the document catalog for this file.
	Catalog *Catalog

	// Info is the document information dictionary for this file.
	// This is nil if the file does not contain a document information
	// dictionary.
	Info *Info

	// Trailer is the trailer dictionary for the file.
	// All entries relating to the cross-reference table are omitted.
	Trailer Dict
}

// Version represents a version of the PDF standard.
type Version int

// PDF versions supported by this library.
const (
	_ Version = iota
	V1_0
	V1_1
	V1_2
	V1_3
	V1_4
	V1_5
	V1_6
	V1_7
	V2_0
	tooHighVersion // TODO(voss): remove
)

// ParseVersion parses a PDF version string.
func ParseVersion(verString string) (Version, error) {
	switch verString {
	case "1.0":
		return V1_0, nil
	case "1.1":
		return V1_1, nil
	case "1.2":
		return V1_2, nil
	case "1.3":
		return V1_3, nil
	case "1.4":
		return V1_4, nil
	case "1.5":
		return V1_5, nil
	case "1.6":
		return V1_6, nil
	case "1.7":
		return V1_7, nil
	case "2.0":
		return V2_0, nil
	}
	return 0, errVersion
}

// ToString returns the string representation of ver, e.g. "1.7".
// If ver does not correspond to a supported PDF version, an error is
// returned.
func (ver Version) ToString() (string, error) {
	if ver >= V1_0 && ver <= V1_7 {
		return "1." + string([]byte{byte(ver - V1_0 + '0')}), nil
	}
	if ver == V2_0 {
		return "2.0", nil
	}
	return "", errVersion
}

func (ver Version) String() string {
	versionString, err := ver.ToString()
	if err != nil {
		versionString = "pdf.Version(" + strconv.Itoa(int(ver)) + ")"
	}
	return versionString
}
