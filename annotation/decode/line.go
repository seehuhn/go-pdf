// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/measure"
)

func decodeLine(c pdf.Cursor, dict pdf.Dict) (*annotation.Line, error) {
	line := &annotation.Line{}

	if err := decodeCommon(c, &line.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &line.Markup); err != nil {
		return nil, err
	}

	// Extract line-specific fields
	// L (required)
	if l, err := pdf.Optional(c.FloatArray(dict["L"])); err != nil {
		return nil, err
	} else if len(l) == 4 {
		copy(line.Coords[:], l)
	}

	// BS (optional)
	if bs, err := c.DecodeOptional(dict["BS"], annotation.ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		line.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			line.Common.Border = nil
		}
	}

	// LE (optional; PDF 1.4) - default is [None, None]
	line.LineEndingStyle = [2]annotation.LineEndingStyle{annotation.LineEndingStyleNone, annotation.LineEndingStyleNone}
	if le, err := pdf.Optional(c.Array(dict["LE"])); err != nil {
		return nil, err
	} else if len(le) >= 1 {
		if name, err := c.Name(le[0]); err == nil {
			line.LineEndingStyle[0] = annotation.LineEndingStyle(name)
		} else if pdf.IsReadError(err) {
			return nil, err
		}
		if len(le) >= 2 {
			if name, err := c.Name(le[1]); err == nil {
				line.LineEndingStyle[1] = annotation.LineEndingStyle(name)
			} else if pdf.IsReadError(err) {
				return nil, err
			}
		} else {
			// if only one element, copy first element to second
			line.LineEndingStyle[1] = line.LineEndingStyle[0]
		}
	}

	// IC (optional; PDF 1.4)
	if ic, err := pdf.Optional(colorenc.Extract(c, dict["IC"])); err != nil {
		return nil, err
	} else {
		line.FillColor = ic
	}

	// Cap (optional)
	caption, _, err := c.OptionalBoolean(dict["Cap"])
	if err != nil {
		return nil, err
	}
	line.Caption = caption

	if line.Caption {
		// CP (optional)
		if cp, err := c.Name(dict["CP"]); err == nil && cp == "Top" {
			line.CaptionAbove = true
		} else if pdf.IsReadError(err) {
			return nil, err
		}

		// CO (optional)
		if co, err := pdf.Optional(c.FloatArray(dict["CO"])); err != nil {
			return nil, err
		} else if len(co) == 2 {
			line.CaptionOffset = co
		}
	}

	// LL (optional)
	if ll, err := pdf.Optional(c.Number(dict["LL"])); err != nil {
		return nil, err
	} else {
		line.LL = ll
	}

	// LLE (optional)
	if lle, err := pdf.Optional(c.Number(dict["LLE"])); err != nil {
		return nil, err
	} else {
		line.LLE = max(lle, 0)
	}

	// LLO (optional)
	if llo, err := pdf.Optional(c.Number(dict["LLO"])); err != nil {
		return nil, err
	} else {
		line.LLO = max(llo, 0)
	}

	// Measure (optional)
	if m, err := c.DecodeOptional(dict["Measure"], measure.Extract); err != nil {
		return nil, err
	} else {
		line.Measure = m
	}

	return line, nil
}
