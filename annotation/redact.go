// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package annotation

import (
	"errors"
	"fmt"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/form"
)

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.23

// Redact represents a redaction annotation.
//
// A redaction annotation marks content for removal, and can be reviewed
// and edited until the redaction is applied.  Applying the redaction
// removes the content in the marked region, draws the replacement mark
// described by the fields below, and deletes the annotation.  This
// package represents the annotation only; it does not apply redactions.
//
// Redaction annotations require PDF 1.7 or later.
type Redact struct {
	Common
	Markup

	// QuadPoints (optional) specifies the coordinates of quadrilaterals that
	// comprise the region to be removed.  Each quadrilateral is represented
	// by 4 Vec2 points, giving the corners in the order upper-left,
	// upper-right, lower-left, lower-right, as for [TextMarkup.QuadPoints].
	// The length must be a multiple of 4.  If QuadPoints is absent,
	// Common.Rect gives the region instead.
	QuadPoints []vec.Vec2

	// FillColor (optional) is the colour used to fill the region after the
	// content has been removed.  This must be a [color.DeviceRGB] colour.
	// If FillColor is nil, the region is left transparent.  If Overlay is
	// set, FillColor is ignored.
	//
	// This corresponds to the /IC entry in the PDF annotation dictionary.
	FillColor color.Color

	// Overlay (optional) is the appearance drawn over the region after the
	// content has been removed.  The form's origin is placed at the
	// lower-left corner of Common.Rect.  If Overlay is set, FillColor,
	// OverlayText, Repeat, DefaultAppearance and Align are ignored.
	//
	// This corresponds to the /RO entry in the PDF annotation dictionary.
	Overlay *form.Form

	// OverlayText (optional) is text drawn over the region after the
	// content has been removed.
	OverlayText pdf.TextString

	// Repeat specifies whether OverlayText is repeated to fill the region.
	Repeat bool

	// DefaultAppearance is the default appearance string used to format
	// OverlayText.  It is required if OverlayText is set.
	//
	// This corresponds to the /DA entry in the PDF annotation dictionary.
	DefaultAppearance string

	// Align specifies the alignment of OverlayText.
	// The zero value is [pdf.TextAlignLeft].
	// The other allowed values are [pdf.TextAlignCenter] and [pdf.TextAlignRight].
	//
	// This corresponds to the /Q entry in the PDF annotation dictionary.
	Align pdf.TextAlign
}

var _ Annotation = (*Redact)(nil)

// AnnotationType returns "Redact".
func (r *Redact) AnnotationType() pdf.Name {
	return "Redact"
}

// Encode converts the annotation to a PDF dictionary.
//
// This implements the [Annotation] interface.
func (r *Redact) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "redaction annotation", pdf.V1_7); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Redact"),
	}

	if err := r.Common.fillDict(rm, dict, isMarkup(r), false); err != nil {
		return nil, err
	}

	if err := r.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// QuadPoints (optional)
	if len(r.QuadPoints) > 0 {
		if len(r.QuadPoints)%4 != 0 {
			return nil, errors.New("length of QuadPoints is not a multiple of 4")
		}
		quadArray := make(pdf.Array, len(r.QuadPoints)*2)
		for i, v := range r.QuadPoints {
			quadArray[i*2] = pdf.Number(v.X)
			quadArray[i*2+1] = pdf.Number(v.Y)
		}
		dict["QuadPoints"] = quadArray
	}

	// IC (optional)
	if r.FillColor != nil {
		if r.FillColor == colorenc.Transparent {
			return nil, errors.New("FillColor must be DeviceRGB")
		}
		ic, err := colorenc.EncodeRGB(r.FillColor)
		if err != nil {
			return nil, err
		}
		dict["IC"] = ic
	}

	// RO (optional)
	if r.Overlay != nil {
		ro, err := rm.Embed(r.Overlay)
		if err != nil {
			return nil, err
		}
		dict["RO"] = ro
	}

	// OverlayText (optional)
	if !r.OverlayText.IsZero() {
		if r.DefaultAppearance == "" {
			return nil, errors.New("DefaultAppearance is required when OverlayText is set")
		}
		dict["OverlayText"] = r.OverlayText
	}

	if r.DefaultAppearance != "" {
		dict["DA"] = pdf.String(r.DefaultAppearance)
	}

	if r.Repeat {
		dict["Repeat"] = pdf.Boolean(true)
	}

	// Q (optional)
	if r.Align != pdf.TextAlignLeft {
		if r.Align < pdf.TextAlignLeft || r.Align > pdf.TextAlignRight {
			return nil, fmt.Errorf("invalid text alignment %d", r.Align)
		}
		dict["Q"] = pdf.Integer(r.Align)
	}

	return dict, nil
}
