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
	"seehuhn.de/go/postscript/cid"
)

// FromFile represents an immutable font read from a PDF file.
type FromFile interface {
	Embedded
	GetDict() Dict
}

// Dict represents a font dictionary in a PDF file.
//
// This interface is implemented by the following types:
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

	// DefaultTextMapping returns the text content implied by each character
	// identifier.  For simple fonts, the cid is taken to be the character code
	// plus one.
	//
	// The text content is based on the CID only and does not take information
	// from the ToUnicode map or from the font file itself into account.
	DefaultTextMapping() map[cid.CID]string

	// TextMapping returns the mapping from character identifiers to text
	// content for this font.  The mapping is based on the ToUnicode map
	// and on the character encoding used in the font.
	//
	// TODO(voss): this is not right!  Text is mapped from codes, not from CIDs!
	TextMapping() map[cid.CID]string

	// GlyphData returns information about the embedded font program associated
	// with this font dictionary.
	//
	// The returned glyphdata.Type indicates the format of the embedded font
	// program (such as Type1, TrueType, CFF, etc.) or [glyphdata.None] if the
	// font is not embedded.
	//
	// The returned pdf.Reference points to the stream object containing the
	// font program in the PDF file. The reference is zero, if and only if the
	// the type is [glyphdata.None].
	//
	// This information can be used to extract the actual glyph outlines from
	// the PDF file for rendering or further processing.
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
