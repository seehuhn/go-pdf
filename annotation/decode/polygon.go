// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/measure"
)

func decodePolygon(c pdf.Cursor, dict pdf.Dict) (*annotation.Polygon, error) {
	polygon := &annotation.Polygon{}

	if err := decodeCommon(c, &polygon.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &polygon.Markup); err != nil {
		return nil, err
	}

	// Extract polygon-specific fields
	// Vertices (required)
	vertices, err := c.FloatArray(dict["Vertices"])
	if err != nil {
		return nil, pdf.Wrap(err, "Vertices")
	}
	if len(vertices) < 4 {
		return nil, pdf.Error("polygon annotation requires Vertices")
	}
	polygon.Vertices = vertices[:len(vertices)&^1]

	// BS (optional)
	if bs, err := c.DecodeOptional(dict["BS"], annotation.ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		polygon.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			polygon.Common.Border = nil
		}
	}

	// IC (optional)
	if ic, err := pdf.Optional(colorenc.Extract(c, dict["IC"])); err != nil {
		return nil, err
	} else {
		polygon.FillColor = ic
	}

	// BE (optional)
	if be, err := c.DecodeOptional(dict["BE"], annotation.ExtractBorderEffect); err != nil {
		return nil, err
	} else {
		polygon.BorderEffect = be
	}

	// Measure (optional)
	if m, err := c.DecodeOptional(dict["Measure"], measure.Extract); err != nil {
		return nil, err
	} else {
		polygon.Measure = m
	}

	// Path (optional; PDF 2.0)
	if p, err := decodePath(c, dict["Path"]); err != nil {
		return nil, err
	} else {
		polygon.Path = p
	}

	return polygon, nil
}
