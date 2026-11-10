// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/colorenc"
)

func decodeRedact(c pdf.Cursor, dict pdf.Dict) (*annotation.Redact, error) {
	redact := &annotation.Redact{}

	if err := decodeCommon(c, &redact.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &redact.Markup); err != nil {
		return nil, err
	}

	// QuadPoints (optional)
	if quadPoints, err := c.FloatArray(dict["QuadPoints"]); err == nil && len(quadPoints) > 0 {
		// Validate that we have a multiple of 8 points (each quadrilateral needs 8 coordinates)
		if len(quadPoints)%8 == 0 {
			redact.QuadPoints = quadPoints
		}
	} else if pdf.IsReadError(err) {
		return nil, err
	}

	// IC (optional) - RGB color components
	if ic, err := pdf.Optional(colorenc.ExtractRGB(c, dict["IC"])); err != nil {
		return nil, err
	} else {
		redact.FillColor = ic
	}

	// RO (optional) - form XObject reference
	if ro, ok := dict["RO"].(pdf.Reference); ok {
		redact.RO = ro
	}

	// OverlayText (optional)
	if overlayText, err := c.TextString(dict["OverlayText"]); err == nil {
		redact.OverlayText = overlayText
	} else if pdf.IsReadError(err) {
		return nil, err
	}

	// Repeat (optional) - default false
	repeat, _, err := c.OptionalBoolean(dict["Repeat"])
	if err != nil {
		return nil, err
	}
	redact.Repeat = repeat

	// DA (required if OverlayText present)
	if da, err := c.String(dict["DA"]); err == nil {
		redact.DefaultAppearance = string(da)
	} else if pdf.IsReadError(err) {
		return nil, err
	}

	// If OverlayText is present but DA is missing/empty, provide a reasonable default
	if !redact.OverlayText.IsZero() && redact.DefaultAppearance == "" {
		// Default to 12pt Helvetica black text
		redact.DefaultAppearance = "/Helvetica 12 Tf 0 0 0 rg"
	}

	// Q (optional) - default 0 (left-justified)
	if q, err := pdf.Optional(c.Integer(dict["Q"])); err != nil {
		return nil, err
	} else if q >= 0 && q <= 2 {
		redact.Align = pdf.TextAlign(q)
	}

	return redact, nil
}
