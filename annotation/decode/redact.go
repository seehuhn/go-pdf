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

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/colorenc"
)

func decodeRedact(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) (*annotation.Redact, error) {
	r := x.R
	redact := &annotation.Redact{}

	// Extract common annotation fields
	if err := decodeCommon(x, path, &redact.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(x, path, dict, &redact.Markup); err != nil {
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
	if ic, err := pdf.Optional(colorenc.ExtractRGB(r, dict["IC"])); err != nil {
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
		redact.DefaultAppearance = string(da)
	}

	// If OverlayText is present but DA is missing/empty, provide a reasonable default
	if redact.OverlayText != "" && redact.DefaultAppearance == "" {
		// Default to 12pt Helvetica black text
		redact.DefaultAppearance = "/Helvetica 12 Tf 0 0 0 rg"
	}

	// Q (optional) - default 0 (left-justified)
	if q, err := pdf.Optional(pdf.GetInteger(r, dict["Q"])); err != nil {
		return nil, err
	} else if q >= 0 && q <= 2 {
		redact.Align = pdf.TextAlign(q)
	}

	return redact, nil
}
