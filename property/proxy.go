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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/opaque"
)

// proxyList adapts a [opaque.Object] to the [List] interface so a
// property list extracted from a PDF file can be re-embedded
// (translating cross-file references) and re-interpreted via
// [ListGet].
type proxyList struct {
	inner *opaque.Object
	path  *pdf.CycleCheck
}

// ExtractList extracts a property list from a PDF object.
// The object must be a dictionary or a reference to a dictionary.
func ExtractList(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (List, error) {
	if !isDirect && path != nil {
		obj = path.Ref
		path = path.Parent
	}

	// validate that the object resolves to a dictionary
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	return &proxyList{
		inner: opaque.Extract(x, obj),
		path:  path,
	}, nil
}

// AsDirectDict returns the property list as a direct dictionary if it
// can be embedded inline in a content stream.  Returns nil for indirect
// property lists or if the dictionary contains indirect references.
func (p *proxyList) AsDirectDict() pdf.Dict {
	return p.inner.AsDirectDict()
}

// Equal reports whether two property lists are semantically equal,
// resolving any indirect references through their respective source
// extractors.  The comparison is structural except for streams and
// placeholders, which are compared by pointer identity (inherited from
// [pdf.Equal]).
func (p *proxyList) Equal(other List) bool {
	q, ok := other.(*proxyList)
	if !ok {
		return false
	}
	return p.inner.Equal(q.inner)
}

// Embed converts the property list into a PDF object.
func (p *proxyList) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	return e.Embed(p.inner)
}
