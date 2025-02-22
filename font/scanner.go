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
	"iter"
	"sync"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
)

type FromFile interface {
	GetScanner() (Scanner, error)
}

type Code struct {
	// CID allows to look up the glyph in the underlying font.
	CID cid.CID

	// Notdef specifies which glyph to show if the requested glyph is not
	// present in the font.
	Notdef cid.CID

	// Width is the glyph width in PDF glyph space units.
	Width float64

	// Text is the text representation of the character.
	Text string
}

var _ Embedded = Scanner(nil)

// Scanner is an embedded font with information about the encoding.
//
// TODO(voss): merge with Scanner
type Scanner interface {
	// WritingMode indicates whether the font is for horizontal or vertical
	// writing.
	WritingMode() WritingMode

	// Codes iterates over the character codes in a PDF string.
	Codes(s pdf.String) iter.Seq[*Code]

	// TODO(voss): remove
	DecodeWidth(pdf.String) (float64, int)
}

type ReaderFunc func(r pdf.Getter, obj pdf.Object) (FromFile, error)

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

func Read(r pdf.Getter, obj pdf.Object) (FromFile, error) {
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
		return nil, fmt.Errorf("unsupported font type: %s", fontType)
	}

	return read(r, fontDict)
}
