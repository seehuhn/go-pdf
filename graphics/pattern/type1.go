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
	"fmt"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
)

// Type1 represents a tiling pattern which repeats periodically in the plane.
type Type1 struct {
	// TilingType is a a code that controls adjustments to the spacing of tiles
	// relative to the device pixel grid.
	TilingType int

	// The pattern cell's bounding box.
	// The pattern cell is clipped to this rectangle before it is painted.
	BBox *pdf.Rectangle

	// XStep is the horizontal spacing between pattern cells.
	XStep float64

	// YStep is the vertical spacing between pattern cells.
	YStep float64

	// Matrix is an array of six numbers specifying the pattern cell's matrix.
	// Leave this empty to use the identity matrix.
	Matrix matrix.Matrix

	// Color indicates whether the pattern specifies color, or whether it only
	// describes a shape.
	Color bool

	// Content is the content stream that draws a single pattern cell.
	Content content.Stream

	// Res contains the resources used by the content stream (required).
	Res *content.Resources
}

var _ color.Pattern = (*Type1)(nil)

// PatternType returns 1 for tiling patterns.
// This implements the [color.Pattern] interface.
func (p *Type1) PatternType() int {
	return 1
}

// PaintType returns 1 for colored patterns and 2 for uncolored patterns.
// This implements the [color.Pattern] interface.
func (p *Type1) PaintType() int {
	if p.Color {
		return 1
	}
	return 2
}

func (p *Type1) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if p.TilingType < 1 || p.TilingType > 3 {
		return nil, fmt.Errorf("invalid tiling type: %d", p.TilingType)
	}
	if p.XStep == 0 || p.YStep == 0 {
		return nil, fmt.Errorf("invalid step size: (%f, %f)", p.XStep, p.YStep)
	}
	if p.Res == nil {
		return nil, fmt.Errorf("missing resources")
	}

	// embed resources
	res := *p.Res
	res.SingleUse = true
	resObj, err := res.Embed(rm)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"PatternType": pdf.Integer(1),
		"PaintType":   pdf.Integer(p.PaintType()),
		"TilingType":  pdf.Integer(p.TilingType),
		"BBox":        p.BBox,
		"XStep":       pdf.Number(p.XStep),
		"YStep":       pdf.Number(p.YStep),
		"Resources":   resObj,
	}
	opt := rm.Out().GetOptions()
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Pattern")
	}
	if p.Matrix != matrix.Identity && p.Matrix != matrix.Zero {
		dict["Matrix"] = toPDF(p.Matrix[:])
	}

	ref := rm.Alloc()
	stm, err := rm.Out().OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, err
	}

	ct := content.PatternColored
	if !p.Color {
		ct = content.PatternUncolored
	}
	err = content.Write(stm, p.Content, pdf.GetVersion(rm.Out()), ct, p.Res)
	if err != nil {
		return nil, err
	}

	err = stm.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
}
