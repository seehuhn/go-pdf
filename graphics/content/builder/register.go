// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/extgstate"
)

// The Register* methods below bind a resource to a caller-chosen name in
// the builder's resource dictionary.  They are intended for scenarios like
// shared content streams (footers, watermarks) whose byte layout fixes the
// resource name in advance, so every page that includes the stream must
// expose the same binding under the same key.
//
// All Register* methods follow the same contract:
//   - If name is unbound, the object is registered under name and nil is
//     returned.
//   - If name is already bound to the same object, the call is a no-op
//     and returns nil.
//   - If name is bound to a different object of the same kind, an error
//     is returned and no state changes.
//
// Subsequent emit-time helpers that resolve the same object look up the
// prescribed name first, so once an object is registered the operator
// bytes will reference name rather than an auto-allocated key.

// RegisterFont binds f to the given resource name in [Builder.Resources].
// If the font's [font.Instance.ResourceName] is non-empty, name must match
// it; otherwise the call fails because the resource-dict key and the font
// dict's /Name would disagree.
func (b *Builder) RegisterFont(name pdf.Name, f font.Instance) error {
	if rn := f.ResourceName(); rn != "" && rn != name {
		return fmt.Errorf("font name %q conflicts with dict Name %q", name, rn)
	}
	if b.Resources.Font == nil {
		b.Resources.Font = make(map[pdf.Name]font.Instance)
	}
	if existing, ok := b.Resources.Font[name]; ok {
		if existing == f {
			return nil
		}
		return fmt.Errorf("font name %q already in use", name)
	}
	b.Resources.Font[name] = f
	b.resName[resKey{resFont, f}] = name
	return nil
}

// RegisterXObject binds x to the given resource name in [Builder.Resources].
// If the XObject's [graphics.XObject.ResourceName] is non-empty, name must
// match it; otherwise the call fails because the resource-dict key and the
// XObject dict's /Name would disagree.
func (b *Builder) RegisterXObject(name pdf.Name, x graphics.XObject) error {
	if rn := x.ResourceName(); rn != "" && rn != name {
		return fmt.Errorf("XObject name %q conflicts with dict Name %q", name, rn)
	}
	if b.Resources.XObject == nil {
		b.Resources.XObject = make(map[pdf.Name]graphics.XObject)
	}
	if existing, ok := b.Resources.XObject[name]; ok {
		if existing == x {
			return nil
		}
		return fmt.Errorf("XObject name %q already in use", name)
	}
	b.Resources.XObject[name] = x
	b.resName[resKey{resXObject, x}] = name
	return nil
}

// RegisterExtGState binds gs to the given resource name in [Builder.Resources].
func (b *Builder) RegisterExtGState(name pdf.Name, gs *extgstate.ExtGState) error {
	if b.Resources.ExtGState == nil {
		b.Resources.ExtGState = make(map[pdf.Name]*extgstate.ExtGState)
	}
	if existing, ok := b.Resources.ExtGState[name]; ok {
		if existing == gs {
			return nil
		}
		return fmt.Errorf("ExtGState name %q already in use", name)
	}
	b.Resources.ExtGState[name] = gs
	b.resName[resKey{resExtGState, gs}] = name
	return nil
}

// RegisterShading binds s to the given resource name in [Builder.Resources].
func (b *Builder) RegisterShading(name pdf.Name, s graphics.Shading) error {
	if b.Resources.Shading == nil {
		b.Resources.Shading = make(map[pdf.Name]graphics.Shading)
	}
	if existing, ok := b.Resources.Shading[name]; ok {
		if existing == s {
			return nil
		}
		return fmt.Errorf("shading name %q already in use", name)
	}
	b.Resources.Shading[name] = s
	b.resName[resKey{resShading, s}] = name
	return nil
}

// RegisterPattern binds p to the given resource name in [Builder.Resources].
func (b *Builder) RegisterPattern(name pdf.Name, p color.Pattern) error {
	if b.Resources.Pattern == nil {
		b.Resources.Pattern = make(map[pdf.Name]color.Pattern)
	}
	if existing, ok := b.Resources.Pattern[name]; ok {
		if existing == p {
			return nil
		}
		return fmt.Errorf("pattern name %q already in use", name)
	}
	b.Resources.Pattern[name] = p
	b.resName[resKey{resPattern, p}] = name
	return nil
}

// RegisterColorSpace binds cs to the given resource name in [Builder.Resources].
func (b *Builder) RegisterColorSpace(name pdf.Name, cs color.Space) error {
	if b.Resources.ColorSpace == nil {
		b.Resources.ColorSpace = make(map[pdf.Name]color.Space)
	}
	if existing, ok := b.Resources.ColorSpace[name]; ok {
		if existing == cs {
			return nil
		}
		return fmt.Errorf("color space name %q already in use", name)
	}
	b.Resources.ColorSpace[name] = cs
	b.resName[resKey{resColorSpace, cs}] = name
	return nil
}
