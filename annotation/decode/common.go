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
	"math"

	"golang.org/x/text/language"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/oc"
)

// decodeCommon extracts fields common to all annotations from a PDF dictionary.
func decodeCommon(c pdf.Cursor, common *annotation.Common, dict pdf.Dict) error {
	// Rect (required)
	if rect, err := c.Rectangle(dict["Rect"]); err == nil && rect != nil {
		common.Rect = *rect
	}

	// Contents (optional)
	if contents, err := c.TextString(dict["Contents"]); err == nil && contents != "" {
		common.Contents = string(contents)
	}

	// P (optional)
	if p, ok := dict["P"].(pdf.Reference); ok {
		common.Page = p
	}

	// NM (optional)
	if nm, err := c.TextString(dict["NM"]); err == nil && nm != "" {
		common.Name = string(nm)
	}

	// M (optional)
	if m, err := c.TextString(dict["M"]); err == nil && m != "" {
		common.LastModified = string(m)
	}

	// F (optional)
	if f, err := c.Integer(dict["F"]); err == nil && f != 0 {
		common.Flags = annotation.Flags(f)
	}

	// AP (optional)
	if ap, err := pdf.DecodeOptional(c, dict["AP"], appearance.ExtractDict); err != nil {
		return err
	} else {
		common.Appearance = ap
	}

	// AS (optional)
	if as, err := c.Name(dict["AS"]); err == nil && as != "" {
		common.AppearanceState = as
	}

	// Border (optional)
	if border, err := pdf.DecodeOptional(c, dict["Border"], annotation.ExtractBorder); err != nil {
		return err
	} else {
		common.Border = border
	}

	// C (optional)
	if cArr, err := c.Array(dict["C"]); err == nil && cArr != nil {
		colors := make([]float64, len(cArr))
		for i, col := range cArr {
			if num, err := c.Number(col); err == nil {
				colors[i] = num
			}
		}
		switch len(colors) {
		case 0:
			// empty array, treat as absent
		case 1:
			common.Color = color.DeviceGray(colors[0])
		case 3:
			common.Color = color.DeviceRGB{colors[0], colors[1], colors[2]}
		case 4:
			common.Color = color.DeviceCMYK{colors[0], colors[1], colors[2], colors[3]}
		}
	}

	// StructParent (optional)
	if dict["StructParent"] != nil {
		if key, err := pdf.Optional(c.Integer(dict["StructParent"])); err != nil {
			return err
		} else if key >= 0 && uint64(key) <= math.MaxUint {
			common.StructParent.Set(uint(key))
		}
	}

	// OC (optional)
	if oc, err := pdf.DecodeOptional(c, dict["OC"], oc.ExtractConditional); err != nil {
		return err
	} else {
		common.OptionalContent = oc
	}

	// AF (optional)
	if afArray, err := pdf.Optional(c.Array(dict["AF"])); err != nil {
		return err
	} else if afArray != nil {
		common.AssociatedFiles = make([]*file.Specification, 0, len(afArray))
		for _, afObj := range afArray {
			if spec, err := pdf.DecodeOptional(c, afObj, file.ExtractSpecification); err != nil {
				return err
			} else if spec != nil {
				common.AssociatedFiles = append(common.AssociatedFiles, spec)
			}
		}
	}

	// CA (optional) - default value is 1.0
	if dict["CA"] != nil {
		if ca, err := c.Number(dict["CA"]); err == nil {
			common.StrokingTransparency = 1 - ca
		}
	}

	// ca (optional) - if not present, defaults to the same value as CA
	if dict["ca"] != nil {
		if ca, err := c.Number(dict["ca"]); err == nil {
			common.NonStrokingTransparency = 1 - ca
		}
	} else {
		common.NonStrokingTransparency = common.StrokingTransparency
	}

	// BM (optional)
	if bm, err := c.Name(dict["BM"]); err == nil && bm != "" {
		common.BlendMode = bm
	}

	// Lang (optional)
	if lang, err := c.TextString(dict["Lang"]); err == nil && lang != "" {
		if tag, err := language.Parse(string(lang)); err == nil {
			common.Lang = tag
		}
	}

	return nil
}

// decodeHighlight reads an annotation's /H entry, supplying the default for
// a missing entry and normalising the deprecated "T" (toggle) mode to
// [annotation.HighlightPush].
func decodeHighlight(c pdf.Cursor, obj pdf.Object) annotation.Highlight {
	h, _ := c.Name(obj)
	switch h {
	case "":
		return annotation.HighlightInvert
	case "T":
		return annotation.HighlightPush
	default:
		return annotation.Highlight(h)
	}
}
