// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/graphics/extract"
)

func decodeRedact(c pdf.Cursor, dict pdf.Dict) (*annotation.Redact, error) {
	redact := &annotation.Redact{}

	if err := decodeCommon(c, &redact.Common, dict); err != nil {
		return nil, err
	}

	if err := decodeMarkup(c, dict, &redact.Markup); err != nil {
		return nil, err
	}

	// QuadPoints (optional)
	if quadPoints, err := c.FloatArray(dict["QuadPoints"]); err == nil {
		// keep complete quadrilaterals only
		n := len(quadPoints) / 8
		if n > 0 {
			points := make([]vec.Vec2, n*4)
			for i := range points {
				points[i] = vec.Vec2{X: quadPoints[2*i], Y: quadPoints[2*i+1]}
			}
			redact.QuadPoints = points
		}
	} else if pdf.IsReadError(err) {
		return nil, err
	}

	// IC (optional)
	if ic, err := pdf.Optional(colorenc.ExtractRGB(c, dict["IC"])); err != nil {
		return nil, err
	} else if ic != colorenc.Transparent {
		redact.FillColor = ic
	}

	// RO (optional)
	if ro, err := c.DecodeOptional(dict["RO"], extract.Form); err != nil {
		return nil, err
	} else {
		redact.Overlay = ro
	}

	// OverlayText (optional)
	if overlayText, err := c.TextString(dict["OverlayText"]); err == nil {
		redact.OverlayText = overlayText
	} else if pdf.IsReadError(err) {
		return nil, err
	}

	// Repeat (optional)
	repeat, _, err := c.OptionalBoolean(dict["Repeat"])
	if err != nil {
		return nil, err
	}
	redact.Repeat = repeat

	// DA (required if OverlayText is present)
	if da, err := c.String(dict["DA"]); err == nil {
		redact.DefaultAppearance = string(da)
	} else if pdf.IsReadError(err) {
		return nil, err
	}

	// OverlayText cannot be formatted without DA
	if redact.DefaultAppearance == "" {
		redact.OverlayText = pdf.TextString{}
	}

	// Q (optional)
	if q, err := pdf.Optional(c.Integer(dict["Q"])); err != nil {
		return nil, err
	} else if q >= 0 && q <= 2 {
		redact.Align = pdf.TextAlign(q)
	}

	return redact, nil
}
