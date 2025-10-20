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

package property

import (
	"slices"

	"seehuhn.de/go/pdf"
)

type proxyList struct {
	x          *pdf.Extractor
	dict       pdf.Dict
	isIndirect bool
}

// ExtractList extracts a property list from a PDF object.
// The object must be a dictionary or a reference to a dictionary.
// Returns an error if the object cannot be converted to a dictionary.
func ExtractList(x *pdf.Extractor, obj pdf.Object) (List, error) {
	_, isIndirect := obj.(pdf.Reference)
	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	}
	p := &proxyList{
		x:          x,
		dict:       dict,
		isIndirect: isIndirect,
	}
	return p, nil
}

// Keys returns the dictionary keys present in the property list.
func (p *proxyList) Keys() []pdf.Name {
	keys := make([]pdf.Name, 0, len(p.dict))
	for k := range p.dict {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (p *proxyList) Get(key pdf.Name) (*ResolvedObject, error) {
	obj, ok := p.dict[key]
	if !ok {
		return nil, ErrNoKey
	}
	return &ResolvedObject{obj, p.x}, nil
}

// Embed converts the Go representation of the object into a PDF object,
// corresponding to the PDF version of the output file.
//
// The first return value is the PDF representation of the object.
// If the object is embedded in the PDF file, this may be a reference.
func (p *proxyList) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	copier := e.CopierFrom(p.x)
	dictOut, err := copier.CopyDict(p.dict)
	if err != nil {
		return nil, err
	}

	if !p.isIndirect {
		return dictOut, nil
	}

	ref := e.AllocSelf()
	err = e.Out().Put(ref, dictOut)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
