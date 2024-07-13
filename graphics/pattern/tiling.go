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
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/matrix"
)

// TilingProperties describes the properties of a tiling pattern.
type TilingProperties struct {
	// TilingType is a a code that controls adjustments to the spacing of tiles
	// relative to the device pixel grid.
	TilingType int

	BBox   *pdf.Rectangle
	XStep  float64
	YStep  float64
	Matrix matrix.Matrix
}

// A TilingColoredBuilder is used to construct a tiling pattern.
type TilingColoredBuilder struct {
	Out pdf.Putter
	*graphics.Writer
	*TilingProperties
}

// NewTilingColored returns a new TilingColoredBuilder.
func NewTilingColored(w pdf.Putter, rm *graphics.ResourceManager, prop *TilingProperties) *TilingColoredBuilder {
	contents := graphics.NewWriter(&bytes.Buffer{}, rm)
	return &TilingColoredBuilder{
		Out:              w,
		Writer:           contents,
		TilingProperties: prop,
	}
}

// Finish creates the new tiling pattern.
func (p *TilingColoredBuilder) Finish() (color.Color, error) {
	if p.Writer.Err != nil {
		return nil, p.Writer.Err
	}

	info := &dictInfo{
		w:         p.Out,
		p:         p.TilingProperties,
		paintType: 1,
		resources: p.Resources,
		body:      p.Writer.Content.(*bytes.Buffer).Bytes(),
	}
	ref, err := info.Embed()
	if err != nil {
		return nil, err
	}

	res := &color.PatternColored{
		Res: pdf.Res{
			Data: ref,
		},
	}
	return res, nil
}

// A TilingUncoloredBuilder is used to construct a tiling pattern.
type TilingUncoloredBuilder struct {
	Out pdf.Putter
	*graphics.Writer
	*TilingProperties
}

// NewTilingUncolored returns a new TilingUncoloredBuilder.
func NewTilingUncolored(w pdf.Putter, rm *graphics.ResourceManager, prop *TilingProperties) *TilingUncoloredBuilder {
	contents := graphics.NewWriter(&bytes.Buffer{}, rm)
	return &TilingUncoloredBuilder{
		Out:              w,
		Writer:           contents,
		TilingProperties: prop,
	}
}

// Finish creates the new tiling pattern.
func (p *TilingUncoloredBuilder) Finish() (*color.TilingPatternUncolored, error) {
	if p.Writer.Err != nil {
		return nil, p.Writer.Err
	}

	info := &dictInfo{
		w:         p.Out,
		p:         p.TilingProperties,
		paintType: 2,
		resources: p.Resources,
		body:      p.Writer.Content.(*bytes.Buffer).Bytes(),
	}
	ref, err := info.Embed()
	if err != nil {
		return nil, err
	}

	res := &color.TilingPatternUncolored{
		Res: pdf.Res{
			Data: ref,
		},
	}
	return res, nil
}

type dictInfo struct {
	w         pdf.Putter
	p         *TilingProperties
	paintType int
	resources *pdf.Resources
	body      []byte
}

func (info *dictInfo) Embed() (pdf.Object, error) {
	p := info.p
	if p.TilingType < 1 || p.TilingType > 3 {
		return nil, fmt.Errorf("invalid tiling type: %d", p.TilingType)
	}
	if p.XStep == 0 || p.YStep == 0 {
		return nil, fmt.Errorf("invalid step size: (%f, %f)", p.XStep, p.YStep)
	}

	dict := pdf.Dict{
		"PatternType": pdf.Integer(1),
		"PaintType":   pdf.Integer(info.paintType),
		"TilingType":  pdf.Integer(p.TilingType),
		"BBox":        p.BBox,
		"XStep":       pdf.Number(p.XStep),
		"YStep":       pdf.Number(p.YStep),
		"Resources":   pdf.AsDict(info.resources),
	}
	if p.Matrix != matrix.Identity && p.Matrix != matrix.Zero {
		dict["Matrix"] = toPDF(p.Matrix[:])
	}

	ref := info.w.Alloc()
	stm, err := info.w.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, err
	}
	_, err = stm.Write(info.body)
	if err != nil {
		return nil, err
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
}
