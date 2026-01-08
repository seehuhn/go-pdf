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
	"strconv"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/state"
)

// Builder constructs PDF content streams using a type-safe API.
type Builder struct {
	contentType content.Type

	Resources *content.Resources
	Stream    content.Stream
	State     *content.State
	Param     graphics.Parameters
	Err       error

	// paramStack holds saved Param values for q/Q operators.
	paramStack []*graphics.Parameters

	// resName tracks allocated resource names for deduplication
	resName map[resKey]pdf.Name

	// glyphBuf is reused by TextShow to avoid allocations
	glyphBuf *font.GlyphSeq
}

// resKey identifies a resource for deduplication purposes.
// The prefix is included so the same object can be used in different categories.
type resKey struct {
	prefix pdf.Name
	obj    pdf.Embedder
}

// resource type prefixes
const (
	resColorSpace pdf.Name = "C"
	resExtGState  pdf.Name = "E"
	resFont       pdf.Name = "F"
	resProperties pdf.Name = "M"
	resPattern    pdf.Name = "P"
	resShading    pdf.Name = "S"
	resXObject    pdf.Name = "X"
)

// New creates a Builder for the given content type.
// If res is nil, the function allocates a new Resources object.
func New(ct content.Type, res *content.Resources) *Builder {
	if res == nil {
		res = &content.Resources{}
	}
	b := &Builder{
		contentType: ct,
		Resources:   res,
		State:       content.NewState(ct),
		Param:       *graphics.NewState().Parameters,
		resName:     make(map[resKey]pdf.Name),
	}

	// pre-fill resName from existing resources
	for name, v := range res.ColorSpace {
		b.resName[resKey{resColorSpace, v}] = name
	}
	for name, v := range res.ExtGState {
		b.resName[resKey{resExtGState, v}] = name
	}
	for name, v := range res.Font {
		b.resName[resKey{resFont, v}] = name
	}
	for name, v := range res.Pattern {
		b.resName[resKey{resPattern, v}] = name
	}
	for name, v := range res.Shading {
		b.resName[resKey{resShading, v}] = name
	}
	for name, v := range res.XObject {
		b.resName[resKey{resXObject, v}] = name
	}

	return b
}

// Harvest returns the stream built so far and clears it.
// State continues for building the next segment.
// Returns error if Err is set (error is sticky, not cleared).
func (b *Builder) Harvest() (content.Stream, error) {
	if b.Err != nil {
		return nil, b.Err
	}
	stream := b.Stream
	b.Stream = nil
	return stream, nil
}

// Close checks that all q/Q, BT/ET, BMC/EMC groups are correctly closed.
// The Builder remains usable after Close.
func (b *Builder) Close() error {
	if b.Err != nil {
		return b.Err
	}
	return b.State.CanClose()
}

// Reset clears the stream and state while preserving the resources dictionary.
// This allows generating multiple separate content streams that share resources.
func (b *Builder) Reset() {
	b.Stream = nil
	b.State = content.NewState(b.contentType)
	b.Param = *graphics.NewState().Parameters
	b.Err = nil
	b.paramStack = nil
}

// emit appends an operator to the content stream and updates the graphics state.
//
// State tracking for q/Q, BT/ET, and BMC/EMC is handled via State methods.
// Individual methods update Known bits and Builder.Param values as needed.
//
// If an error occurs, it is stored in b.Err and subsequent calls become no-ops.
func (b *Builder) emit(name content.OpName, args ...pdf.Object) {
	if b.Err != nil {
		return
	}

	if err := b.State.ApplyOperator(name, args); err != nil {
		b.Err = err
		return
	}

	op := content.Operator{Name: name, Args: args}
	b.Stream = append(b.Stream, op)
}

func (b *Builder) isKnown(bits state.Bits) bool {
	return b.State.IsKnown(bits)
}

func (b *Builder) isSet(bits state.Bits) bool {
	return b.State.IsSet(bits)
}

func (b *Builder) getColorSpaceName(cs color.Space) pdf.Name {
	key := resKey{resColorSpace, cs}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.ColorSpace == nil {
		b.Resources.ColorSpace = make(map[pdf.Name]color.Space)
	}
	name := allocateName("C", b.Resources.ColorSpace)
	b.Resources.ColorSpace[name] = cs
	b.resName[key] = name
	return name
}

func (b *Builder) getExtGStateName(gs *graphics.ExtGState) pdf.Name {
	key := resKey{resExtGState, gs}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.ExtGState == nil {
		b.Resources.ExtGState = make(map[pdf.Name]*graphics.ExtGState)
	}
	name := allocateName("E", b.Resources.ExtGState)
	b.Resources.ExtGState[name] = gs
	b.resName[key] = name
	return name
}

func (b *Builder) getFontName(f font.Instance) pdf.Name {
	key := resKey{resFont, f}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.Font == nil {
		b.Resources.Font = make(map[pdf.Name]font.Instance)
	}
	name := allocateName("F", b.Resources.Font)
	b.Resources.Font[name] = f
	b.resName[key] = name
	return name
}

func (b *Builder) getPatternName(p color.Pattern) pdf.Name {
	key := resKey{resPattern, p}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.Pattern == nil {
		b.Resources.Pattern = make(map[pdf.Name]color.Pattern)
	}
	name := allocateName("P", b.Resources.Pattern)
	b.Resources.Pattern[name] = p
	b.resName[key] = name
	return name
}

func (b *Builder) getShadingName(s graphics.Shading) pdf.Name {
	key := resKey{resShading, s}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.Shading == nil {
		b.Resources.Shading = make(map[pdf.Name]graphics.Shading)
	}
	name := allocateName("S", b.Resources.Shading)
	b.Resources.Shading[name] = s
	b.resName[key] = name
	return name
}

func (b *Builder) getXObjectName(x graphics.XObject) pdf.Name {
	key := resKey{resXObject, x}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.XObject == nil {
		b.Resources.XObject = make(map[pdf.Name]graphics.XObject)
	}
	name := allocateName("X", b.Resources.XObject)
	b.Resources.XObject[name] = x
	b.resName[key] = name
	return name
}

func allocateName[T any](prefix pdf.Name, dict map[pdf.Name]T) pdf.Name {
	// Start from len+1 and decrement to avoid quadratic complexity when
	// allocating many resources: the first check usually succeeds.
	for k := len(dict) + 1; ; k-- {
		name := pdf.Name(string(prefix) + strconv.Itoa(k))
		if _, exists := dict[name]; !exists {
			return name
		}
	}
}
