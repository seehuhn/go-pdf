package builder

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/property"
)

// MarkedContent represents a marked-content point or sequence.
type MarkedContent struct {
	// Tag specifies the role or significance of the point/sequence.
	Tag pdf.Name

	// Properties is an optional property list providing additional data.
	// Set to nil for marked content without properties (MP/BMC operators).
	Properties property.List

	// Inline controls whether the property list is embedded inline in the
	// content stream (true) or referenced via the Properties resource
	// dictionary (false). Only relevant if Properties is not nil.
	// Property lists can only be inlined if IsDirect() returns true.
	Inline bool
}

// MarkedContentPoint adds a marked-content point to the content stream.
//
// This implements the PDF graphics operators "MP" (without properties)
// and "DP" (with properties).
func (b *Builder) MarkedContentPoint(mc *MarkedContent) {
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
func (b *Builder) MarkedContentStart(mc *MarkedContent) {
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

func (b *Builder) getProperties(mc *MarkedContent) pdf.Object {
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
	key := resKey{"M", mc.Properties}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.Properties == nil {
		b.Resources.Properties = make(map[pdf.Name]property.List)
	}
	name := allocateName("M", b.Resources.Properties)
	b.Resources.Properties[name] = mc.Properties
	b.resName[key] = name
	return name
}

// ErrNotDirect is returned when attempting to inline a property list
// that cannot be embedded inline in the content stream.
var ErrNotDirect = errors.New("property list cannot be inlined in content stream")
