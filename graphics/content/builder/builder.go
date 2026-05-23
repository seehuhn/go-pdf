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
	"fmt"
	"strconv"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extgstate"
)

// Builder constructs PDF content streams using a type-safe API.
//
// Builder is the only API in this package that produces validated content
// streams.  Each emit-time check uses the builder's target PDF version and
// content-type context: operators unknown or unavailable in the chosen
// version are rejected, deprecated operators are rejected (callers must use
// the modern typed helper instead, e.g. Fill rather than the "F" operator),
// and structural constraints such as the pre-PDF-2.0 q/Q stack depth limit
// and q/Q-in-text-object prohibition are enforced when q or Q is emitted.
// The first failure becomes a sticky [Builder.Err]; subsequent emits become
// no-ops.  The failing operator is appended to the stream once so that
// diagnostic round-tripping still surfaces the root-cause operator rather
// than a cascading nesting failure.
//
// To retrieve the built-up content as a [*content.Operators] segment,
// call [Builder.Harvest] (or [Builder.Build] for a one-shot
// reset-then-build pattern); [Must] makes the common error-free path a
// one-liner: builder.Must(b.Harvest()).
//
// [Builder.RawContent] is the one bypass: bytes written through it are
// inserted verbatim and not validated.  Callers using RawContent are
// responsible for their content's correctness.
type Builder struct {
	contentType content.Type
	version     pdf.Version

	Resources *content.Resources
	Stream    []content.Operator
	State     *content.State
	Err       error

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

// New creates a Builder targeting the given PDF version and content-type
// context.  The version is used to reject operators that are unknown or
// unavailable in that version (e.g. `ri` on PDF 1.0, `gs` on pre-1.2,
// the deprecated `F` at any version) and to enforce structural limits
// (q/Q stack depth and q/Q-in-text-object) for pre-PDF-2.0.
// If res is nil, the function allocates a new Resources object.
func New(ct content.Type, res *content.Resources, version pdf.Version) *Builder {
	if res == nil {
		res = &content.Resources{}
	}
	state := content.NewState(ct, res)
	state.Version = version
	b := &Builder{
		contentType: ct,
		version:     version,
		Resources:   res,
		State:       state,
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
		// PDF spec Table 109 / 110: when Name is present on the font, it
		// must equal the resource-dict key used here.
		if rn := v.ResourceName(); rn != "" && rn != name {
			b.Err = fmt.Errorf("font resource name %q does not match dict Name %q",
				name, rn)
		}
	}
	for name, v := range res.Pattern {
		b.resName[resKey{resPattern, v}] = name
	}
	for name, v := range res.Shading {
		b.resName[resKey{resShading, v}] = name
	}
	for name, v := range res.XObject {
		b.resName[resKey{resXObject, v}] = name
		// PDF spec Table 93: when Name is present on the XObject, it
		// must equal the resource-dict key used here.
		if rn := v.ResourceName(); rn != "" && rn != name {
			b.Err = fmt.Errorf("XObject resource name %q does not match dict Name %q",
				name, rn)
		}
	}

	return b
}

// Harvest returns the stream built so far as a [*content.Operators]
// segment and clears the builder's accumulator.  Graphics state continues
// for building the next segment.  Returns the sticky error if Err is set
// (the error is not cleared).
func (b *Builder) Harvest() (*content.Operators, error) {
	if b.Err != nil {
		return nil, b.Err
	}
	stream := b.Stream
	b.Stream = nil
	return &content.Operators{Ops: stream}, nil
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
	b.State = content.NewState(b.contentType, b.Resources)
	b.State.Version = b.version
	b.Err = nil
}

// emit appends an operator to the content stream and updates the graphics state.
//
// State tracking for q/Q, BT/ET, and BMC/EMC is handled via State methods.
// Individual methods update Known bits and Builder.Param values as needed.
//
// If an error occurs, it is stored in [Builder.Err] and subsequent calls
// become no-ops.  The failing operator itself is still appended to the
// stream so that a diagnostic replay of [Builder.Stream] reproduces the
// same root-cause error rather than a cascading failure such as
// "unclosed operators: q/Q" caused by suppressed matching closers.
func (b *Builder) emit(name content.OpName, args ...pdf.Object) {
	if b.Err != nil {
		return
	}

	op := content.Operator{Name: name, Args: args}
	if err := content.CheckOperatorVersion(name, b.version); err != nil {
		b.Err = fmt.Errorf("operator %s: %w", name, err)
		b.Stream = append(b.Stream, op)
		return
	}
	if err := b.State.ApplyOperator(name, args); err != nil {
		b.Err = err
		b.Stream = append(b.Stream, op)
		return
	}

	b.Stream = append(b.Stream, op)
}

func (b *Builder) isUsable(bits graphics.Bits) bool {
	return b.State.IsUsable(bits)
}

func (b *Builder) isSet(bits graphics.Bits) bool {
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

func (b *Builder) getExtGStateName(gs *extgstate.ExtGState) pdf.Name {
	key := resKey{resExtGState, gs}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.ExtGState == nil {
		b.Resources.ExtGState = make(map[pdf.Name]*extgstate.ExtGState)
	}
	name := allocateName("E", b.Resources.ExtGState)
	b.Resources.ExtGState[name] = gs
	b.resName[key] = name
	return name
}

// FontName returns the resource name for the given font instance.
// If the font hasn't been registered yet, it is added to the builder's
// font resources.  If the font's [font.Instance.ResourceName] returns a
// non-empty value, that value is used as the resource-dict key (per
// PDF spec Table 109 / 110); collisions with a different font already
// stored under the same key set [Builder.Err].  Otherwise a fresh name
// is allocated.
func (b *Builder) FontName(f font.Instance) pdf.Name {
	key := resKey{resFont, f}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.Font == nil {
		b.Resources.Font = make(map[pdf.Name]font.Instance)
	}
	name := f.ResourceName()
	if name != "" {
		if existing, ok := b.Resources.Font[name]; ok && existing != f {
			b.Err = fmt.Errorf("font name %q already in use", name)
			return ""
		}
	} else {
		name = allocateName("F", b.Resources.Font)
	}
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
	name := x.ResourceName()
	if name != "" {
		if existing, ok := b.Resources.XObject[name]; ok && existing != x {
			b.Err = fmt.Errorf("XObject name %q already in use", name)
			return ""
		}
	} else {
		name = allocateName("X", b.Resources.XObject)
	}
	b.Resources.XObject[name] = x
	b.resName[key] = name
	return name
}

func allocateName[T any](prefix pdf.Name, dict map[pdf.Name]T) pdf.Name {
	// In normal sequential use, len(dict)+1 is always free (one iteration).
	// Decrementing handles user-assigned names by filling gaps downward.
	for k := len(dict) + 1; ; k-- {
		name := pdf.Name(string(prefix) + strconv.Itoa(k))
		if _, exists := dict[name]; !exists {
			return name
		}
	}
}
