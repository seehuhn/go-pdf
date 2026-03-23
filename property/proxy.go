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
)

type proxyList struct {
	x         *pdf.Extractor
	path      *pdf.CycleCheck
	obj       pdf.Object
	wasInline bool
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

	p := &proxyList{
		x:         x,
		path:      path,
		obj:       obj,
		wasInline: isDirect,
	}
	return p, nil
}

// AsDirectDict returns the property list as a direct dictionary if it can
// be embedded inline in a content stream.  Returns nil for indirect
// property lists or if the dictionary contains indirect references.
func (p *proxyList) AsDirectDict() pdf.Dict {
	if !p.wasInline {
		return nil
	}
	dict, _ := p.obj.(pdf.Dict)
	if !pdf.IsDirect(dict) {
		return nil
	}
	return dict
}

// Equal reports whether two property lists are semantically equal.
func (p *proxyList) Equal(other List) bool {
	q, ok := other.(*proxyList)
	if !ok {
		return false
	}
	resolvedA, errA := p.x.DeepResolve(p.obj)
	resolvedB, errB := q.x.DeepResolve(q.obj)
	if errA != nil || errB != nil {
		return false
	}
	return pdf.Equal(resolvedA, resolvedB)
}

// Embed converts the property list into a PDF object.
func (p *proxyList) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	dict, err := p.x.GetDict(p.path, p.obj)
	if err != nil {
		return nil, err
	}

	copier := e.CopierFrom(p.x)
	dictOut, err := copier.CopyDict(dict)
	if err != nil {
		return nil, err
	}

	if p.wasInline {
		return dictOut, nil
	}

	ref := e.AllocSelf()
	err = e.Out().Put(ref, dictOut)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
