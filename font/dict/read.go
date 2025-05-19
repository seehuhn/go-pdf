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

package dict

import (
	"errors"
	"fmt"
	"sync"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// Read reads a font dictionary from a PDF file.
func Read(r pdf.Getter, obj pdf.Object) (font.Dict, error) {
	fontDict, err := pdf.GetDictTyped(r, obj, "Font")
	if err != nil {
		return nil, err
	} else if fontDict == nil {
		return nil, pdf.Error("missing font dictionary")
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

type readerFunc func(r pdf.Getter, obj pdf.Object) (font.Dict, error)

var (
	readerMutex sync.Mutex
	readers     map[pdf.Name]readerFunc
)

func registerReader(tp pdf.Name, fn readerFunc) {
	readerMutex.Lock()
	defer readerMutex.Unlock()

	if readers == nil {
		readers = make(map[pdf.Name]readerFunc)
	}

	if _, alreadyPresent := readers[tp]; alreadyPresent {
		panic(fmt.Sprintf("conflicting readers for font type %s", tp))
	}

	readers[tp] = fn
}
