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

package graphics

import (
	"bytes"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/matrix"
)

// TilingProperties describes the properties of a tiling pattern.
type TilingProperties struct {
	Colored     bool
	TilingType  int
	BBox        *pdf.Rectangle
	XStep       float64
	YStep       float64
	Matrix      matrix.Matrix
	DefaultName pdf.Name
}

// A TilingPatternBuilder is used to construct a tiling pattern.
type TilingPatternBuilder struct {
	Out pdf.Putter
	*Writer
	TilingProperties
}

// NewTilingPattern returns a new TilingPatternBuilder.
func NewTilingPattern(w pdf.Putter, prop TilingProperties) *TilingPatternBuilder {
	contents := NewWriter(&bytes.Buffer{}, pdf.GetVersion(w))
	return &TilingPatternBuilder{
		Out:              w,
		Writer:           contents,
		TilingProperties: prop,
	}
}

// Make creates the new tiling pattern.
func (p *TilingPatternBuilder) Make() (pdf.Res, error) {
	if p.Writer.Err != nil {
		return pdf.Res{}, p.Writer.Err
	}

	if p.TilingType < 1 || p.TilingType > 3 {
		return pdf.Res{}, fmt.Errorf("invalid tiling type: %d", p.TilingType)
	}
	if p.XStep == 0 || p.YStep == 0 {
		return pdf.Res{}, fmt.Errorf("invalid step size: (%f, %f)", p.XStep, p.YStep)
	}

	dict := pdf.Dict{
		"PatternType": pdf.Integer(1),
		"TilingType":  pdf.Integer(p.TilingType),
		"BBox":        p.BBox,
		"XStep":       pdf.Number(p.XStep),
		"YStep":       pdf.Number(p.YStep),
		"Resources":   pdf.AsDict(p.Resources),
	}
	if p.Colored {
		dict["PaintType"] = pdf.Integer(1)
	} else {
		dict["PaintType"] = pdf.Integer(2)
	}
	if p.Matrix != matrix.Identity {
		dict["Matrix"] = toPDF(p.Matrix[:])
	}

	ref := p.Out.Alloc()
	stm, err := p.Out.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return pdf.Res{}, err
	}
	_, err = stm.Write(p.Writer.Content.(*bytes.Buffer).Bytes())
	if err != nil {
		return pdf.Res{}, err
	}
	err = stm.Close()
	if err != nil {
		return pdf.Res{}, err
	}

	// disable the pattern builder to prevent further writes
	p.Writer = nil

	return pdf.Res{
		DefName: p.DefaultName,
		Data:    ref,
	}, nil
}

// NewShadingPattern creates a new shading pattern.
func NewShadingPattern(w pdf.Putter, shading *color.EmbeddedShading, M matrix.Matrix, extGState *ExtGState) (color.Color, error) {
	dict := pdf.Dict{
		"PatternType": pdf.Integer(2),
		"Shading":     shading.Data,
	}
	if M != matrix.Identity {
		dict["Matrix"] = toPDF(M[:])
	}
	if extGState != nil {
		dict["ExtGState"] = extGState.PDFObject()
	}

	ref := w.Alloc()
	err := w.Put(ref, dict)
	if err != nil {
		return nil, err
	}

	return color.NewShadingPattern(ref, ""), nil
}

func toPDF(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}
