// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/encoding"
)

// DefaultWidthDefault is the value of DefaultWidth which is used if no
// value is specified in the PDF file.  Using this value for the "DefaultWidth"
// field in a composite font dictionary will slightly reduce PDF file size.
const DefaultWidthDefault = 1000

// setSimpleWidths updates fontDict with a FirstChar, LastChar, and Widths
// array based on non-default width values in ww. It ignores unused codes.
//
// Returns any additional objects and their references if an indirect object is
// created.
func setSimpleWidths(w *pdf.Writer, fontDict pdf.Dict, ww []float64, enc encoding.Simple, defaultWidth float64) ([]pdf.Object, []pdf.Reference) {
	// Find the range [firstChar, lastChar] of characters that need encoding.
	// The spec requires at least one character to be encoded.
	// The firstChar < lastChar condition ensures firstChar <= lastChar,
	// creating a one-element array when all chars have default width.
	firstChar, lastChar := 0, 255
	for lastChar > 0 && (enc(byte(lastChar)) == "" || ww[lastChar] == defaultWidth) {
		lastChar--
	}
	for firstChar < lastChar && (enc(byte(firstChar)) == "" || ww[firstChar] == defaultWidth) {
		firstChar++
	}

	widths := make(pdf.Array, lastChar-firstChar+1)
	for i := range widths {
		widths[i] = pdf.Number(ww[firstChar+i])
	}

	fontDict["FirstChar"] = pdf.Integer(firstChar)
	fontDict["LastChar"] = pdf.Integer(lastChar)

	if len(widths) <= 10 {
		fontDict["Widths"] = widths
		return nil, nil
	}

	widthRef := w.Alloc()
	fontDict["Widths"] = widthRef
	return []pdf.Object{widths}, []pdf.Reference{widthRef}
}

// encodeCompositeWidths creates a W array for a composite font.  Each run is
// written in its own form.  The result is nil if m is empty.
func encodeCompositeWidths(m CIDMetrics[float64]) (pdf.Array, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	var res pdf.Array
	for i := range m {
		r := &m[i]
		if r.Values == nil {
			res = append(res,
				pdf.Integer(r.First),
				pdf.Integer(r.Last),
				pdf.Number(r.Value))
			continue
		}
		list := make(pdf.Array, len(r.Values))
		for j, v := range r.Values {
			list[j] = pdf.Number(v)
		}
		res = append(res, pdf.Integer(r.First), list)
	}
	return res, nil
}

// DefaultVMetricsDefault is the value of DefaultVMetrics used, if no value is
// specified in the PDF file.  Using this value for a "DefaultVMetrics" field
// in a composite font dictionary will slightly reduce PDF file size.
var DefaultVMetricsDefault = DefaultVMetrics{
	OffsY:  880,
	DeltaY: -1000,
}

// DefaultVMetrics represents the default vertical positioning and advancement
// for a font in vertical writing mode.
type DefaultVMetrics struct {
	// OffsY is the vertical component of the glyph position vector, in PDF
	// glyph space units.
	//
	// The effect of this is that the glyph is moved down by OffsY, compared to
	// horizontal writing.
	//
	// There is no separate OffsX default value for vertical writing. Instead,
	// half of the horizontal glyph width is used as the default.
	OffsY float64

	// DeltaY is the vertical displacement of the following glyph, in PDF glyph
	// space units. (The horizontal displacement is always zero.)
	//
	// This is normally negative, so that writing goes from top to bottom.
	DeltaY float64
}

// encodeVDefault creates a PDF array representation of default vertical metrics.
// Returns nil if the metrics match the default values to save space.
func encodeVDefault(metrics DefaultVMetrics) pdf.Array {
	if metrics.OffsY == 880 && metrics.DeltaY == -1000 {
		return nil
	}
	return pdf.Array{
		pdf.Number(metrics.OffsY),
		pdf.Number(metrics.DeltaY),
	}
}

// VMetrics represents the vertical metrics for an individual CID.
type VMetrics struct {
	// OffsX is the horizontal component of the glyph position vector, in PDF
	// glyph space units.
	//
	// The effect of this is that the glyph is moved left by OffsX units,
	// compared to horizontal writing.
	OffsX float64

	// OffsY is the vertical component of the glyph position vector, in PDF
	// glyph space units.
	//
	// The effect of this is that the glyph is moved down by OffsY, compared to
	// horizontal writing.
	OffsY float64

	// DeltaY is the vertical displacement for the following glyph.
	// (The horizontal displacement is always zero.)
	//
	// This is normally negative, so that writing goes from top to bottom.
	DeltaY float64
}

// setVerticalMetrics sets the vertical writing mode fields of c, whose CID
// and Width are already set.  The metrics come from the W2 entry for the CID
// if there is one, and from DW2 and half the glyph width otherwise.
func setVerticalMetrics(c *font.Code, w2 CIDMetrics[VMetrics], dw2 DefaultVMetrics) {
	if vm, ok := w2.Get(c.CID); ok {
		c.VerticalAdvance = vm.DeltaY / 1000
		c.VerticalOriginX = vm.OffsX / 1000
		c.VerticalOriginY = vm.OffsY / 1000
		return
	}
	c.VerticalAdvance = dw2.DeltaY / 1000
	c.VerticalOriginX = c.Width / 2
	c.VerticalOriginY = dw2.OffsY / 1000
}

// encodeVMetrics creates a W2 array for a composite font.  Each run is
// written in its own form.  The result is nil if m is empty.
func encodeVMetrics(m CIDMetrics[VMetrics]) (pdf.Array, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	var res pdf.Array
	for i := range m {
		r := &m[i]
		if r.Values == nil {
			res = append(res,
				pdf.Integer(r.First),
				pdf.Integer(r.Last),
				pdf.Number(r.Value.DeltaY),
				pdf.Number(r.Value.OffsX),
				pdf.Number(r.Value.OffsY))
			continue
		}
		list := make(pdf.Array, 0, 3*len(r.Values))
		for _, v := range r.Values {
			list = append(list,
				pdf.Number(v.DeltaY),
				pdf.Number(v.OffsX),
				pdf.Number(v.OffsY))
		}
		res = append(res, pdf.Integer(r.First), list)
	}
	return res, nil
}
