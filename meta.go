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
	"fmt"
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

	// Trailer is the trailer dictionary for the file. All entries relating to
	// the cross-reference table, and the entries corresponding to the `ID`,
	// `Catalog` and `Info` fields above, are omitted.
	Trailer Dict

	// Permissions records the access permissions granted when the file
	// was opened.  For unencrypted files this is PermAll.
	Permissions Perm

	// Encryption describes the encryption configuration of the file, or
	// nil if the file is unencrypted.
	Encryption *Encryption
}

// Encryption describes the encryption configuration of a PDF file.
type Encryption struct {
	// Cipher names the bulk-encryption algorithm: "RC4" or "AES".
	Cipher string

	// KeyLength is the encryption key length in bits.  For RC4 it is a
	// multiple of 8 between 40 and 128; for AES it is 128 or 256.
	KeyLength int
}

// String returns a short, human-readable label for the encryption
// configuration, e.g. "RC4 (40-bit)", "AES-128", or "AES-256".
// A nil receiver returns "None" so callers can format the unencrypted case
// without an explicit nil check.
func (e *Encryption) String() string {
	if e == nil {
		return "None"
	}

	switch e.Cipher {
	case "":
		return "Unknown"
	case "RC4":
		return fmt.Sprintf("RC4 (%d-bit)", e.KeyLength)
	}
	return fmt.Sprintf("%s-%d", e.Cipher, e.KeyLength)
}

// Version represents a version of the PDF standard.
//
// Versions are encoded as 100*major+minor, so that the natural integer
// order coincides with the order of PDF versions.  The zero value means
// "no version set".  Values beyond MaxVersion can occur when reading a
// file written against a future version of the standard; the library
// itself writes at most MaxVersion.
type Version int

// PDF versions supported by this library.
const (
	V1_0 Version = 100 + iota
	V1_1
	V1_2
	V1_3
	V1_4
	V1_5
	V1_6
	V1_7

	V2_0 Version = 200

	MaxVersion = V2_0
)

// ParseVersion parses a PDF version string such as "1.7" or "2.0".
// Well-formed versions newer than MaxVersion (e.g. "2.3") parse
// successfully, so that files written against a future version of the
// standard can still be read.
func ParseVersion(verString string) (Version, error) {
	if len(verString) != 3 || verString[1] != '.' {
		return 0, errVersion
	}
	major := verString[0]
	minor := verString[2]
	if major < '1' || major > '9' || minor < '0' || minor > '9' {
		return 0, errVersion
	}
	return Version(100*int(major-'0') + int(minor-'0')), nil
}

// ToString returns the string representation of ver, e.g. "1.7".
// If ver does not correspond to a PDF version supported by this library,
// an error is returned; use [Version.String] to format versions read
// from files written against a future version of the standard.
func (ver Version) ToString() (string, error) {
	if ver >= V1_0 && ver <= V1_7 || ver == V2_0 {
		return ver.String(), nil
	}
	return "", errVersion
}

func (ver Version) String() string {
	major := int(ver) / 100
	minor := int(ver) % 100
	if major >= 1 && major <= 9 && minor <= 9 {
		return strconv.Itoa(major) + "." + strconv.Itoa(minor)
	}
	return "pdf.Version(" + strconv.Itoa(int(ver)) + ")"
}
