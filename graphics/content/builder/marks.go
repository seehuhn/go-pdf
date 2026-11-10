// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
		dict := mc.Properties.AsDirectDict()
		if dict == nil {
			b.Err = ErrNotDirect
			return nil
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
