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

package content

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// handleShading implements the sh operator (paint shading)
func handleShading(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	name := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	if _, ok := res.Shading[name]; !ok {
		return errors.New("shading not found")
	}

	return nil
}

// handleXObject implements the Do operator (paint XObject)
func handleXObject(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	name := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	if _, ok := res.XObject[name]; !ok {
		return errors.New("XObject not found")
	}

	return nil
}

// handleMarkedContentPoint implements the MP operator
func handleMarkedContentPoint(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	_ = p.GetName() // tag
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleMarkedContentPointWithProperties implements the DP operator
func handleMarkedContentPointWithProperties(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	_ = p.GetName() // tag
	// Second arg can be name or dict
	if len(p.args) > 0 {
		p.args = p.args[1:]
	}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleBeginMarkedContent implements the BMC operator
func handleBeginMarkedContent(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	_ = p.GetName() // tag
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleBeginMarkedContentWithProperties implements the BDC operator
func handleBeginMarkedContentWithProperties(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	_ = p.GetName() // tag
	// Second arg can be name or dict
	if len(p.args) > 0 {
		p.args = p.args[1:]
	}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleEndMarkedContent implements the EMC operator
func handleEndMarkedContent(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleType3d0 implements the d0 operator (Type 3 font glyph width)
func handleType3d0(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	_ = p.GetFloat() // wx
	_ = p.GetFloat() // wy
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleType3d1 implements the d1 operator (Type 3 font glyph width and bbox)
func handleType3d1(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	_ = p.GetFloat() // wx
	_ = p.GetFloat() // wy
	_ = p.GetFloat() // llx
	_ = p.GetFloat() // lly
	_ = p.GetFloat() // urx
	_ = p.GetFloat() // ury
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleBeginCompatibility implements the BX operator
func handleBeginCompatibility(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleEndCompatibility implements the EX operator
func handleEndCompatibility(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleRawContent implements the %raw% special operator
func handleRawContent(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	_ = p.GetString() // raw content
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleInlineImage implements the %image% special operator
func handleInlineImage(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	_ = p.GetDict()   // image parameters
	_ = p.GetString() // image data
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}
