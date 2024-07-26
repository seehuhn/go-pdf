// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package pattern

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/matrix"
)

// NewColoredBuilder returns a Type1Builder for a colored tiling pattern.
//
// The resulting pattern can only be used with same resource manager as was
// used for the builder.
func NewColoredBuilder(rm *pdf.ResourceManager, prop *TilingProperties) *Type1Builder {
	contents := graphics.NewWriter(&bytes.Buffer{}, rm)
	return &Type1Builder{
		Writer:           contents,
		TilingProperties: prop,
		paintType:        1,
	}
}

// NewUncoloredBuilder returns a Type1Builder for an uncolored tiling pattern.
//
// The resulting pattern can only be used with same resource manager as was
// used for the builder.
func NewUncoloredBuilder(rm *pdf.ResourceManager, prop *TilingProperties) *Type1Builder {
	contents := graphics.NewWriter(&bytes.Buffer{}, rm)
	return &Type1Builder{
		Writer:           contents,
		TilingProperties: prop,
		paintType:        2,
	}
}

// A Type1Builder is used to construct a tiling pattern.
//
// Use methods of the embedded [graphics.Writer] to draw the pattern cell.
type Type1Builder struct {
	*graphics.Writer
	*TilingProperties

	paintType int
}

// Finish creates the new tiling pattern.
func (p *Type1Builder) Finish() (color.Pattern, error) {
	if p.Writer.Err != nil {
		return nil, p.Writer.Err
	}

	info := p.TilingProperties
	if info.TilingType < 1 || info.TilingType > 3 {
		return nil, fmt.Errorf("invalid tiling type: %d", info.TilingType)
	}
	if info.XStep == 0 || info.YStep == 0 {
		return nil, fmt.Errorf("invalid step size: (%f, %f)", info.XStep, info.YStep)
	}

	dict := pdf.Dict{
		// "Type":        pdf.Name("Pattern"),
		"PatternType": pdf.Integer(1),
		"PaintType":   pdf.Integer(p.paintType),
		"TilingType":  pdf.Integer(info.TilingType),
		"BBox":        info.BBox,
		"XStep":       pdf.Number(info.XStep),
		"YStep":       pdf.Number(info.YStep),
		"Resources":   pdf.AsDict(p.Writer.Resources),
	}
	if info.Matrix != matrix.Identity && info.Matrix != matrix.Zero {
		dict["Matrix"] = toPDF(info.Matrix[:])
	}

	w := p.Writer.RM.Out
	ref := w.Alloc()
	stm, err := w.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(stm, p.Writer.Content.(*bytes.Buffer))
	if err != nil {
		return nil, err
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	return &type1{
		RM:        p.Writer.RM,
		PaintType: p.paintType,
		Ref:       ref,
	}, nil
}

type type1 struct {
	// RM is the resource manager for the pattern.
	RM *pdf.ResourceManager

	// PaintType is the paint type specified in the pattern dictionary.
	PaintType int

	// Ref is the reference to the pattern's content stream.
	Ref pdf.Reference
}

// IsColored returns true if the tiling pattern is colored.
// This implements the [seehuhn.de/go/pdf/graphics/color.Pattern] interface.
func (p *type1) IsColored() bool {
	return p.PaintType == 1
}

// Embed returns a reference to the pattern's content stream.
// The resource manager must be the same as the one used to create the pattern.
// This implements the [seehuhn.de/go/pdf/graphics/color.Pattern] interface.
func (p *type1) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused

	if p.RM != rm {
		return nil, zero, errWrongResourceManager
	}

	return p.Ref, zero, nil
}

var (
	errWrongResourceManager = errors.New("wrong resource manager")
)
