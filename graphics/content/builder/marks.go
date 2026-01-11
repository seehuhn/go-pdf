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

package builder

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/property"
)

// MarkedContentPoint adds a marked-content point to the content stream.
//
// This implements the PDF graphics operators "MP" (without properties)
// and "DP" (with properties).
func (b *Builder) MarkedContentPoint(mc *graphics.MarkedContent) {
	if b.Err != nil {
		return
	}

	if mc.Properties == nil {
		b.emit(content.OpMarkedContentPoint, mc.Tag)
		return
	}

	prop := b.getProperties(mc)
	if b.Err != nil {
		return
	}
	b.emit(content.OpMarkedContentPointWithProperties, mc.Tag, prop)
}

// MarkedContentStart begins a marked-content sequence. The sequence is
// terminated by a call to [Builder.MarkedContentEnd].
//
// This implements the PDF graphics operators "BMC" and "BDC".
func (b *Builder) MarkedContentStart(mc *graphics.MarkedContent) {
	if b.Err != nil {
		return
	}

	if mc.Properties == nil {
		b.emit(content.OpBeginMarkedContent, mc.Tag)
		return
	}

	prop := b.getProperties(mc)
	if b.Err != nil {
		return
	}
	b.emit(content.OpBeginMarkedContentWithProperties, mc.Tag, prop)
}

// MarkedContentEnd ends a marked-content sequence.
// This must be matched with a preceding call to [Builder.MarkedContentStart].
func (b *Builder) MarkedContentEnd() {
	b.emit(content.OpEndMarkedContent)
}

func (b *Builder) getProperties(mc *graphics.MarkedContent) pdf.Object {
	if mc.Inline {
		if !mc.Properties.IsDirect() {
			b.Err = ErrNotDirect
			return nil
		}
		// build a dict from the property list
		dict := pdf.Dict{}
		for _, key := range mc.Properties.Keys() {
			val, err := mc.Properties.Get(key)
			if err == nil {
				dict[key] = val
			}
		}
		return dict
	}

	// reference via Properties resource
	key := resKey{resProperties, mc.Properties}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.Properties == nil {
		b.Resources.Properties = make(map[pdf.Name]property.List)
	}
	name := allocateName(resProperties, b.Resources.Properties)
	b.Resources.Properties[name] = mc.Properties
	b.resName[key] = name
	return name
}

// ErrNotDirect is returned when attempting to inline a property list
// that cannot be embedded inline in the content stream.
var ErrNotDirect = errors.New("property list cannot be inlined in content stream")
