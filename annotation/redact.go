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

	// IC (optional) - RGB color components for interior fill after redaction
	IC []float64

	// RO (optional) - form XObject for overlay appearance
	RO pdf.Reference

	// OverlayText (optional) - text to display over redacted region
	OverlayText string

	// Repeat (optional) - whether overlay text should repeat to fill region
	Repeat bool

	// DA (required if OverlayText present) - appearance string for formatting overlay text
	DA string

	// Q (optional) - quadding/justification code (0=left, 1=center, 2=right)
	Q int
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
	if ic, err := pdf.GetFloatArray(r, dict["IC"]); err == nil && len(ic) == 3 {
		// Clamp invalid color values to valid range [0, 1]
		colors := make([]float64, 3)
		for i, val := range ic {
			if val < 0.0 {
				val = 0.0
			} else if val > 1.0 {
				val = 1.0
			}
			colors[i] = val
		}
		redact.IC = colors
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
	if q, err := pdf.GetInteger(r, dict["Q"]); err == nil {
		qVal := int(q)
		// Only accept valid Q values (0, 1, 2)
		if qVal >= 0 && qVal <= 2 {
			redact.Q = qVal
		}
	}

	return redact, nil
}

func (r *Redact) Encode(rm *pdf.ResourceManager) (pdf.Dict, error) {
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
	if len(r.IC) == 3 {
		// Validate color components are in range [0, 1]
		validColors := true
		for _, c := range r.IC {
			if c < 0.0 || c > 1.0 {
				validColors = false
				break
			}
		}
		if validColors {
			dict["IC"] = pdf.Array{
				pdf.Number(r.IC[0]),
				pdf.Number(r.IC[1]),
				pdf.Number(r.IC[2]),
			}
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
	if r.Q != 0 {
		if r.Q < 0 || r.Q > 2 {
			return nil, fmt.Errorf("the Q field must be 0, 1 or 2")
		}
		dict["Q"] = pdf.Integer(r.Q)
	}

	return dict, nil
}
