// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/colorenc"
)

func decodeCircle(c pdf.Cursor, dict pdf.Dict) (*annotation.Circle, error) {
	circle := &annotation.Circle{}

	if err := decodeCommon(c, &circle.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &circle.Markup); err != nil {
		return nil, err
	}

	// Extract circle-specific fields
	// BS (optional)
	if bs, err := c.DecodeOptional(dict["BS"], annotation.ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		circle.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			circle.Common.Border = nil
		}
	}

	// BE (optional): a border effect is meaningful only together with a
	// border style, so drop it when BS is absent (the writer requires BS)
	if be, err := c.DecodeOptional(dict["BE"], annotation.ExtractBorderEffect); err != nil {
		return nil, err
	} else if circle.BorderStyle != nil {
		circle.BorderEffect = be
	}

	// IC (optional)
	if ic, err := pdf.Optional(colorenc.Extract(c, dict["IC"])); err != nil {
		return nil, err
	} else {
		circle.FillColor = ic
	}

	// RD (optional)
	if rd, err := c.FloatArray(dict["RD"]); err == nil && len(rd) == 4 {
		for i := range rd {
			rd[i] = max(rd[i], 0)
		}
		if rd[0]+rd[2] < circle.Rect.Dx() && rd[1]+rd[3] < circle.Rect.Dy() {
			circle.Margin = rd
		}
	} else if pdf.IsReadError(err) {
		return nil, err
	}

	return circle, nil
}
