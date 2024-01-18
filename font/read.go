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

package font

import (
	"errors"
	"sync"

	"seehuhn.de/go/pdf"
)

var (
	loaderMutex sync.Mutex
	loader      map[EmbeddingType]func(pdf.Getter, pdf.Object, pdf.Name) (NewFont, error)
)

// RegisterLoader registers a new font loader.
func RegisterLoader(t EmbeddingType, f func(pdf.Getter, pdf.Object, pdf.Name) (NewFont, error)) {
	loaderMutex.Lock()
	defer loaderMutex.Unlock()
	if loader == nil {
		loader = make(map[EmbeddingType]func(pdf.Getter, pdf.Object, pdf.Name) (NewFont, error))
	}
	loader[t] = f
}

func getLoader(t EmbeddingType) func(pdf.Getter, pdf.Object, pdf.Name) (NewFont, error) {
	loaderMutex.Lock()
	defer loaderMutex.Unlock()
	return loader[t]
}

// Read extracts a font from a PDF file.
func Read(r pdf.Getter, ref pdf.Object, name pdf.Name) (NewFont, error) {
	fontDicts, err := ExtractDicts(r, ref)
	if err != nil {
		return nil, err
	}

	load := getLoader(fontDicts.Type)
	if load == nil {
		return nil, errors.New("unsupported font type " + fontDicts.Type.String())
	}
	return load(r, ref, name)
}
