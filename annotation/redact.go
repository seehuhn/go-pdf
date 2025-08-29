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

package annotation

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// Redact represents a redaction annotation (PDF 1.7+).
// Redaction annotations identify content intended to be removed from the document
// through a two-phase process: content identification and content removal.
type Redact struct {
	Common
	Markup

	// QuadPoints (optional) - array of coordinates specifying quadrilaterals
	// for content regions to remove. If absent, Rect defines the redaction region.
	QuadPoints []float64

	// FillColor (optional) is the color used to fill the redacted area after
	// the content has been removed.   This must be a DeviceRGB color.
	//
	// This corresponds to the /IC entry in the PDF annotation dictionary.
	FillColor color.Color

	// RO (optional) - form XObject for overlay appearance
	RO pdf.Reference

	// OverlayText (optional) - text to display over redacted region
	OverlayText string

	// Repeat (optional) - whether overlay text should repeat to fill region
	Repeat bool

	// DA (required if OverlayText present) - appearance string for formatting overlay text
	DA string

	// Align specifies the text alignment used for the annotation's text.
	// The zero value if [TextAlignLeft].
	// The other allowed values are [TextAlignCenter] and [TextAlignRight].
	//
	// This corresponds to the /Q entry in the PDF annotation dictionary.
	Align TextAlign
}

var _ Annotation = (*Redact)(nil)

// AnnotationType returns "Redact".
func (r *Redact) AnnotationType() pdf.Name {
	return "Redact"
}

func decodeRedact(r pdf.Getter, dict pdf.Dict) (*Redact, error) {
	redact := &Redact{}

	// Extract common annotation fields
	if err := decodeCommon(r, &redact.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(r, dict, &redact.Markup); err != nil {
		return nil, err
	}

	// QuadPoints (optional)
	if quadPoints, err := pdf.GetFloatArray(r, dict["QuadPoints"]); err == nil && len(quadPoints) > 0 {
		// Validate that we have a multiple of 8 points (each quadrilateral needs 8 coordinates)
		if len(quadPoints)%8 == 0 {
			redact.QuadPoints = quadPoints
		}
	}

	// IC (optional) - RGB color components
	if ic, err := pdf.Optional(extractColorRGB(r, dict["IC"])); err != nil {
		return nil, err
	} else {
		redact.FillColor = ic
	}

	// RO (optional) - form XObject reference
	if ro, ok := dict["RO"].(pdf.Reference); ok {
		redact.RO = ro
	}

	// OverlayText (optional)
	if overlayText, err := pdf.GetTextString(r, dict["OverlayText"]); err == nil {
		redact.OverlayText = string(overlayText)
	}

	// Repeat (optional) - default false
	if repeat, err := pdf.GetBoolean(r, dict["Repeat"]); err == nil {
		redact.Repeat = bool(repeat)
	}

	// DA (required if OverlayText present)
	if da, err := pdf.GetString(r, dict["DA"]); err == nil {
		redact.DA = string(da)
	}

	// If OverlayText is present but DA is missing/empty, provide a reasonable default
	if redact.OverlayText != "" && redact.DA == "" {
		// Default to 12pt Helvetica black text
		redact.DA = "/Helvetica 12 Tf 0 0 0 rg"
	}

	// Q (optional) - default 0 (left-justified)
	if q, err := pdf.Optional(pdf.GetInteger(r, dict["Q"])); err != nil {
		return nil, err
	} else if q >= 0 && q <= 2 {
		redact.Align = TextAlign(q)
	}

	return redact, nil
}

func (r *Redact) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "redaction annotation", pdf.V1_7); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Redact"),
	}

	// Add common annotation fields
	if err := r.Common.fillDict(rm, dict, isMarkup(r)); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := r.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// QuadPoints (optional)
	if len(r.QuadPoints) > 0 && len(r.QuadPoints)%8 == 0 {
		quadPoints := make(pdf.Array, len(r.QuadPoints))
		for i, point := range r.QuadPoints {
			quadPoints[i] = pdf.Number(point)
		}
		dict["QuadPoints"] = quadPoints
	}

	// IC (optional) - RGB color components
	if r.FillColor != nil {
		if icArray, err := encodeColorRGB(r.FillColor); err != nil {
			return nil, err
		} else if icArray != nil {
			dict["IC"] = icArray
		}
	}

	// RO (optional) - form XObject reference
	if r.RO != 0 {
		dict["RO"] = r.RO
	}

	// OverlayText (optional)
	if r.OverlayText != "" {
		dict["OverlayText"] = pdf.TextString(r.OverlayText)

		// DA (required if OverlayText present)
		if r.DA == "" {
			return nil, fmt.Errorf("DA field is required when OverlayText is present")
		}
	}

	// DA (write if present)
	if r.DA != "" {
		dict["DA"] = pdf.String(r.DA)
	}

	// Repeat (optional) - default false
	if r.Repeat {
		dict["Repeat"] = pdf.Boolean(true)
	}

	// Q (optional) - default 0 (left-justified)
	if r.Align != TextAlignLeft {
		dict["Q"] = pdf.Integer(r.Align)
	}

	return dict, nil
}
