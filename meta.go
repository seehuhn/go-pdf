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
	"time"

	"golang.org/x/text/language"
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
	// This excludes entries related to the cross-reference table.
	Trailer Dict
}

// Version represents a version of PDF standard.
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

// Catalog represents a PDF Document Catalog.  The only required field in this
// structure is Pages, which specifies the root of the page tree.
// This struct can be used with the [DecodeDict] and [AsDict] functions.
//
// The Document Catalog is documented in section 7.7.2 of PDF 32000-1:2008.
type Catalog struct {
	_                 struct{} `pdf:"Type=Catalog"`
	Version           Version  `pdf:"optional"`
	Extensions        Object   `pdf:"optional"`
	Pages             Reference
	PageLabels        Object       `pdf:"optional"`
	Names             Object       `pdf:"optional"`
	Dests             Object       `pdf:"optional"`
	ViewerPreferences Object       `pdf:"optional"`
	PageLayout        Name         `pdf:"optional"`
	PageMode          Name         `pdf:"optional"`
	Outlines          Reference    `pdf:"optional"`
	Threads           Reference    `pdf:"optional"`
	OpenAction        Object       `pdf:"optional"`
	AA                Object       `pdf:"optional"`
	URI               Object       `pdf:"optional"`
	AcroForm          Object       `pdf:"optional"`
	MetaData          Reference    `pdf:"optional"`
	StructTreeRoot    Object       `pdf:"optional"`
	MarkInfo          Object       `pdf:"optional"`
	Lang              language.Tag `pdf:"optional"`
	SpiderInfo        Object       `pdf:"optional"`
	OutputIntents     Object       `pdf:"optional"`
	PieceInfo         Object       `pdf:"optional"`
	OCProperties      Object       `pdf:"optional"`
	Perms             Object       `pdf:"optional"`
	Legal             Object       `pdf:"optional"`
	Requirements      Object       `pdf:"optional"`
	Collection        Object       `pdf:"optional"`
	NeedsRendering    bool         `pdf:"optional"`
}

// Info represents a PDF Document Information Dictionary.
// All fields in this structure are optional.
//
// The Document Information Dictionary is documented in section
// 14.3.3 of PDF 32000-1:2008.
type Info struct {
	Title    string `pdf:"text string,optional"`
	Author   string `pdf:"text string,optional"`
	Subject  string `pdf:"text string,optional"`
	Keywords string `pdf:"text string,optional"`

	// Creator gives the name of the application that created the original
	// document, if the document was converted to PDF from another format.
	Creator string `pdf:"text string,optional"`

	// Producer gives the name of the application that converted the document,
	// if the document was converted to PDF from another format.
	Producer string `pdf:"text string,optional"`

	// CreationDate gives the date and time the document was created.
	CreationDate time.Time `pdf:"optional"`

	// ModDate gives the date and time the document was most recently modified.
	ModDate time.Time `pdf:"optional"`

	// Trapped indicates whether the document has been modified to include
	// trapping information.  (A trap is an overlap between adjacent areas of
	// of different colours, used to avoid visual problems caused by imprecise
	// alignment of different layers of ink.) Possible values are:
	//   * "True": The document has been fully trapped.  No further trapping is
	//     necessary.
	//   * "False": The document has not been trapped.
	//   * "Unknown" (default): Either it is unknown whether the document has
	//     been trapped, or the document has been partially trapped.  Further
	//     trapping may be necessary.
	Trapped Name `pdf:"optional,allowstring"`

	// Custom contains all non-standard fields in the Info dictionary.
	Custom map[string]string `pdf:"extra"`
}
