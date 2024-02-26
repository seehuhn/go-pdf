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
)

// TilingProperties describes the properties of a tiling pattern.
type TilingProperties struct {
	DefaultName pdf.Name
	TilingType  int
	BBox        *pdf.Rectangle
	XStep       float64
	YStep       float64
	Matrix      Matrix
}

type TilingPatternUncolored struct {
	Out pdf.Putter
	*Writer
	TilingProperties
}

func NewTilingPatternUncolored(w pdf.Putter, prop TilingProperties) *TilingPatternUncolored {
	contents := NewWriter(&bytes.Buffer{}, pdf.GetVersion(w))
	return &TilingPatternUncolored{
		Out:              w,
		Writer:           contents,
		TilingProperties: prop,
	}
}

func (p *TilingPatternUncolored) Embed() (*WhatsIt, error) {
	if p.Writer.Err != nil {
		return nil, p.Writer.Err
	}

	if p.TilingType < 1 || p.TilingType > 3 {
		return nil, fmt.Errorf("invalid tiling type: %d", p.TilingType)
	}
	if p.XStep == 0 || p.YStep == 0 {
		return nil, fmt.Errorf("invalid step size: (%f, %f)", p.XStep, p.YStep)
	}

	dict := pdf.Dict{
		"PatternType": pdf.Integer(1),
		"PaintType":   pdf.Integer(2),
		"TilingType":  pdf.Integer(p.TilingType),
		"BBox":        p.BBox,
		"XStep":       pdf.Number(p.XStep),
		"YStep":       pdf.Number(p.YStep),
		"Resources":   pdf.AsDict(p.Resources),
	}
	if p.Matrix != IdentityMatrix {
		dict["Matrix"] = toPDF(p.Matrix[:])
	}

	ref := p.Out.Alloc()
	stm, err := p.Out.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, err
	}
	_, err = stm.Write(p.Writer.Content.(*bytes.Buffer).Bytes())
	if err != nil {
		return nil, err
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	// disable the pattern builder to prevent further writes
	p.Writer = nil

	c := &WhatsIt{
		pattern: pdf.Res{
			DefName: p.DefaultName,
			Ref:     ref,
		},
	}
	return c, nil
}

type WhatsIt struct {
	pattern pdf.Res
}

func (w *WhatsIt) New(c Color) Color {
	return colorTilingPattern{col: c, pattern: w.pattern}
}

// colorSpacePattern is used for uncolored tiling patterns and shading patterns.
type colorSpacePattern struct{}

// DefaultName implements the [ColorSpace] interface.
func (s colorSpacePattern) DefaultName() pdf.Name {
	return ""
}

// PDFObject implements the [ColorSpace] interface.
func (s colorSpacePattern) PDFObject() pdf.Object {
	return pdf.Name("Pattern")
}

// ColorSpaceFamily implements the [ColorSpace] interface.
func (s colorSpacePattern) ColorSpaceFamily() string {
	return "Pattern"
}

// defaultColor implements the [ColorSpace] interface.
func (s colorSpacePattern) defaultColor() Color {
	return blankPattern // TODO(voss): check this
}

type colorSpacePatternUncolored struct {
	s ColorSpace
}

func (s colorSpacePatternUncolored) DefaultName() pdf.Name {
	return pdf.Name("")
}

func (s colorSpacePatternUncolored) PDFObject() pdf.Object {
	return pdf.Array{
		pdf.Name("Pattern"),
		s.s.PDFObject(),
	}
}

// ColorSpaceFamily implements the [ColorSpace] interface.
func (s colorSpacePatternUncolored) ColorSpaceFamily() string {
	return "Pattern"
}

// defaultColor implements the [ColorSpace] interface.
func (s colorSpacePatternUncolored) defaultColor() Color {
	return blankPattern // TODO(voss): check this
}

var blankPattern = (*colorTilingPattern)(nil)

type colorTilingPattern struct {
	col     Color
	pattern pdf.Res
}

// ColorSpace implements the [Color] interface.
func (c colorTilingPattern) ColorSpace() ColorSpace {
	return colorSpacePatternUncolored{s: c.col.ColorSpace()}
}

// values implements the [Color] interface.
func (c colorTilingPattern) values() []float64 {
	return c.col.values()
}

// func NewShadingPattern(w pdf.Putter, shading pdf.Dict, matrix Matrix, extGState *ExtGState) (*ShadingPattern, error) {
// 	if stp, ok := shading["ShadingType"].(pdf.Integer); !ok || stp < 1 || stp > 7 {
// 		return nil, fmt.Errorf("invalid shading type: %d", stp)
// 	}

// 	dict := pdf.Dict{
// 		"PatternType": pdf.Integer(2),
// 		"Shading":     shading,
// 	}
// 	if matrix != IdentityMatrix {
// 		dict["Matrix"] = toPDF(matrix[:])
// 	}
// 	if extGState != nil {
// 		dict["ExtGState"] = extGState.PDFObject()
// 	}

// 	res := &ShadingPattern{
// 		Res: pdf.Res{
// 			Ref: dict,
// 		},
// 	}
// 	return res, nil
// }

// type ShadingPattern struct {
// 	pdf.Res
// }

// func (c *ShadingPattern) Embed(w pdf.Putter) (*ShadingPattern, error) {
// 	if _, ok := c.Res.Ref.(pdf.Reference); ok {
// 		return c, nil
// 	}

// 	ref := w.Alloc()
// 	err := w.Put(ref, c.Ref)
// 	if err != nil {
// 		return nil, err
// 	}

// 	c2 := clone(c)
// 	c2.Ref = ref
// 	return c2, nil
// }

// func (c *ShadingPattern) setStroke(w *Writer) {
// 	minVersion := pdf.V1_3
// 	if w.Version < minVersion {
// 		w.Err = &pdf.VersionError{Operation: "shading patterns", Earliest: minVersion}
// 		return
// 	}

// 	cur := w.StrokeColor

// 	var isPattern bool
// 	switch cur.(type) {
// 	case colorTilingPattern, *ShadingPattern:
// 		isPattern = true
// 	}

// 	// First set the color space, if needed.
// 	if !isPattern {
// 		_, w.Err = w.Content.Write([]byte("/Pattern CS\n"))
// 		if w.Err != nil {
// 			return
// 		}
// 		cur = nil
// 	}

// 	// Then set the pattern.
// 	if cur == c {
// 		return
// 	}
// 	name := w.getResourceName(catPattern, c)
// 	w.Err = name.PDF(w.Content)
// 	if w.Err != nil {
// 		return
// 	}
// 	_, w.Err = fmt.Fprintln(w.Content, " SCN")
// 	if w.Err != nil {
// 		return
// 	}

// 	w.StrokeColor = c
// 	w.State.Set |= StateStrokeColor
// }

// func (c *ShadingPattern) setFill(w *Writer) {
// 	minVersion := pdf.V1_3
// 	if w.Version < minVersion {
// 		w.Err = &pdf.VersionError{Operation: "shading patterns", Earliest: minVersion}
// 		return
// 	}

// 	cur := w.FillColor

// 	var isPattern bool
// 	switch cur.(type) {
// 	case colorTilingPattern, *ShadingPattern:
// 		isPattern = true
// 	}

// 	// First set the color space, if needed.
// 	if !isPattern {
// 		_, w.Err = w.Content.Write([]byte("/Pattern cs\n"))
// 		if w.Err != nil {
// 			return
// 		}
// 		cur = nil
// 	}

// 	// Then set the pattern.
// 	if cur == c {
// 		return
// 	}
// 	name := w.getResourceName(catPattern, c)
// 	w.Err = name.PDF(w.Content)
// 	if w.Err != nil {
// 		return
// 	}
// 	_, w.Err = fmt.Fprintln(w.Content, " scn")
// 	if w.Err != nil {
// 		return
// 	}

// 	w.FillColor = c
// 	w.State.Set |= StateFillColor
// }
