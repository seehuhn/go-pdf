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
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodeFreeText(c pdf.Cursor, dict pdf.Dict) (*annotation.FreeText, error) {
	f := &annotation.FreeText{}

	if err := decodeCommon(c, &f.Common, dict); err != nil {
		return nil, err
	}
	if err := decodeMarkup(c, dict, &f.Markup); err != nil {
		return nil, err
	}

	// only the free text intents are valid here; snap anything else to the
	// default so the annotation can be written back
	switch f.Intent {
	case "", annotation.FreeTextIntentPlain, annotation.FreeTextIntentCallout, annotation.FreeTextIntentTypeWriter:
	default:
		f.Intent = ""
	}

	if da, err := pdf.Optional(c.String(dict["DA"])); err != nil {
		return nil, err
	} else {
		f.DefaultAppearance = string(da)
	}
	if f.DefaultAppearance == "" {
		// DA is required; default to 12pt Helvetica black text
		f.DefaultAppearance = "/Helvetica 12 Tf 0 0 0 rg"
	}

	if q, err := pdf.Optional(c.Integer(dict["Q"])); err != nil {
		return nil, err
	} else if q >= 0 && q <= 2 {
		f.Align = pdf.TextAlign(q)
	}

	if ds, err := pdf.Optional(c.TextString(dict["DS"])); err != nil {
		return nil, err
	} else {
		f.DefaultStyle = string(ds)
	}

	if cl, err := pdf.Optional(c.Array(dict["CL"])); err != nil {
		return nil, err
	} else if f.Intent == annotation.FreeTextIntentCallout && (len(cl) == 4 || len(cl) == 6) {
		points := make([]vec.Vec2, len(cl)/2)
		for i := range len(points) {
			if px, err := c.Number(cl[i*2]); err == nil {
				if py, err := c.Number(cl[i*2+1]); err == nil {
					points[i] = vec.Vec2{X: px, Y: py}
				}
			}
		}
		f.CalloutLine = points
	}

	if be, err := pdf.DecodeOptional(c, dict["BE"], annotation.ExtractBorderEffect); err != nil {
		return nil, err
	} else {
		f.BorderEffect = be
	}

	if rd, err := pdf.Optional(c.Array(dict["RD"])); err != nil {
		return nil, err
	} else if len(rd) == 4 {
		a := make([]float64, 4)
		for i, diff := range rd {
			num, _ := c.Number(diff)
			a[i] = max(num, 0)
		}
		if a[0]+a[2] < f.Rect.Dx() && a[1]+a[3] < f.Rect.Dy() {
			f.Margin = a
		}
	}

	if bs, err := pdf.DecodeOptional(c, dict["BS"], annotation.ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		f.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			f.Common.Border = nil
		}
	}

	if f.Intent == annotation.FreeTextIntentCallout {
		if le, err := pdf.Optional(c.Name(dict["LE"])); err != nil {
			return nil, err
		} else if le != "" {
			f.LineEndingStyle = annotation.LineEndingStyle(le)
		} else {
			f.LineEndingStyle = annotation.LineEndingStyleNone
		}
	}

	return f, nil
}
