// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package font

import (
	"errors"
	"fmt"
	"sync"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/glyphdata"
)

// FromFile represents an immutable font read from a PDF file.
type FromFile interface {
	Embedded
	GetDict() Dict
}

// Dict represents a font dictionary in a PDF file.
//
// This interface is implemented by the following types, corresponding to the
// different font dictionary types supported by PDF:
//   - [seehuhn.de/go/pdf/font/dict.Type1]
//   - [seehuhn.de/go/pdf/font/dict.TrueType]
//   - [seehuhn.de/go/pdf/font/dict.Type3]
//   - [seehuhn.de/go/pdf/font/dict.CIDFontType0]
//   - [seehuhn.de/go/pdf/font/dict.CIDFontType2]
type Dict interface {
	// WriteToPDF adds this font dictionary to the PDF file using the given
	// reference.
	//
	// The resource manager is used to deduplicate child objects
	// like encoding dictionaries, CMap streams, etc.
	WriteToPDF(*pdf.ResourceManager, pdf.Reference) error

	// MakeFont returns a new font object that can be used to typeset text.
	// The font is immutable, i.e. no new glyphs can be added and no new codes
	// can be defined via the returned font object.
	MakeFont() (FromFile, error)

	// GlyphData returns information about the embedded font program.
	//
	// Returns the format of the embedded font program (Type1, TrueType, CFF,
	// etc.) as a glyphdata.Type, or glyphdata.None if the font is not
	// embedded. Also returns a pdf.Reference to the stream object containing
	// the font program. This reference is zero if and only if the type is
	// glyphdata.None.
	GlyphData() (glyphdata.Type, pdf.Reference)
}

// ReadDict reads a font dictionary from a PDF file.
func ReadDict(r pdf.Getter, obj pdf.Object) (Dict, error) {
	fontDict, err := pdf.GetDictTyped(r, obj, "Font")
	if err != nil {
		return nil, err
	}

	fontType, err := pdf.GetName(r, fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	fontDict["Subtype"] = fontType

	if fontType == "Type0" {
		a, err := pdf.GetArray(r, fontDict["DescendantFonts"])
		if err != nil {
			return nil, err
		} else if len(a) < 1 {
			return nil, &pdf.MalformedFileError{
				Err: errors.New("composite font with no descendant fonts"),
			}
		}
		fontDict["DescendantFonts"] = a

		cidFontDict, err := pdf.GetDictTyped(r, a[0], "Font")
		if err != nil {
			return nil, err
		}
		a[0] = cidFontDict

		fontType, err = pdf.GetName(r, cidFontDict["Subtype"])
		if err != nil {
			return nil, err
		}
		cidFontDict["Subtype"] = fontType
	}

	readerMutex.Lock()
	defer readerMutex.Unlock()

	read, ok := readers[fontType]
	if !ok {
		return nil, pdf.Errorf("unsupported font type: %s", fontType)
	}

	return read(r, fontDict)
}

type ReaderFunc func(r pdf.Getter, obj pdf.Object) (Dict, error)

var (
	readerMutex sync.Mutex
	readers     map[pdf.Name]ReaderFunc
)

func RegisterReader(tp pdf.Name, fn ReaderFunc) {
	readerMutex.Lock()
	defer readerMutex.Unlock()

	if readers == nil {
		readers = make(map[pdf.Name]ReaderFunc)
	}

	if _, alreadyPresent := readers[tp]; alreadyPresent {
		panic(fmt.Sprintf("conflicting readers for font type %s", tp))
	}

	readers[tp] = fn
}
