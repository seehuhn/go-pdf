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
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/langtag"
	"seehuhn.de/go/pdf/oc"
)

// decodeCommon extracts fields common to all annotations from a PDF dictionary.
func decodeCommon(c pdf.Cursor, common *annotation.Common, dict pdf.Dict) error {
	// Rect (required)
	if rect, err := c.Rectangle(dict["Rect"]); err == nil && rect != nil {
		common.Rect = *rect
	} else if pdf.IsReadError(err) {
		return err
	}

	// Contents (optional)
	if contents, err := c.TextString(dict["Contents"]); err == nil && !contents.IsZero() {
		common.Contents = contents
	} else if pdf.IsReadError(err) {
		return err
	}

	// P (optional)
	if p, ok := dict["P"].(pdf.Reference); ok {
		common.Page = p
	}

	// NM (optional)
	if nm, err := c.TextString(dict["NM"]); err == nil && !nm.IsZero() {
		common.Name = nm.String()
	} else if pdf.IsReadError(err) {
		return err
	}

	// M (optional)
	if m, err := c.TextString(dict["M"]); err == nil && !m.IsZero() {
		common.LastModified = m.String()
	} else if pdf.IsReadError(err) {
		return err
	}

	// F (optional)
	if f, err := c.Integer(dict["F"]); err == nil && f != 0 {
		common.Flags = annotation.Flags(f)
	} else if pdf.IsReadError(err) {
		return err
	}

	// AP (optional)
	if ap, err := c.DecodeOptional(dict["AP"], appearance.ExtractDict); err != nil {
		return err
	} else {
		common.Appearance = ap
	}

	// AS (optional)
	if as, err := c.Name(dict["AS"]); err == nil && as != "" {
		common.AppearanceState = as
	} else if pdf.IsReadError(err) {
		return err
	}

	// Border (optional)
	if border, err := c.DecodeOptional(dict["Border"], annotation.ExtractBorder); err != nil {
		return err
	} else {
		common.Border = border
	}

	// C (optional)
	if col, err := pdf.Optional(colorenc.Extract(c, dict["C"])); err != nil {
		return err
	} else {
		common.Color = col
	}

	// StructParent (optional)
	if key, err := c.Integer(dict["StructParent"]); err == nil {
		if key >= 0 && uint64(key) <= math.MaxUint {
			common.StructParent.Set(uint(key))
		}
	} else if pdf.IsReadError(err) {
		return err
	}

	// OC (optional)
	if oc, err := c.DecodeOptional(dict["OC"], oc.ExtractConditional); err != nil {
		return err
	} else {
		common.OptionalContent = oc
	}

	// AF (optional)
	af, err := file.ExtractAssociatedFiles(c, dict["AF"])
	if err != nil {
		return err
	}
	common.AssociatedFiles = af

	// CA (optional) - default value is 1.0
	if ca, err := c.Number(dict["CA"]); err == nil {
		common.StrokingTransparency = 1 - min(max(ca, 0), 1)
	} else if pdf.IsReadError(err) {
		return err
	}

	// ca (optional) - if not present, defaults to the same value as CA
	if ca, err := c.Number(dict["ca"]); err == nil {
		common.NonStrokingTransparency = 1 - min(max(ca, 0), 1)
	} else if pdf.IsMalformed(err) {
		common.NonStrokingTransparency = common.StrokingTransparency
	} else {
		return err
	}

	// BM (optional)
	if bm, err := c.Name(dict["BM"]); err == nil && bm != "" {
		common.BlendMode = bm
	} else if pdf.IsReadError(err) {
		return err
	}

	// Lang (optional).  An empty string or "und" states that the language
	// is unknown; a malformed value is dropped.
	if obj, err := pdf.Optional(c.Resolve(dict["Lang"])); err != nil {
		return err
	} else if obj != nil {
		if lang, err := c.TextString(obj); err == nil {
			s := lang.String()
			unknown := s == "" || strings.EqualFold(s, "und")
			if tag := langtag.Parse(s); unknown || !tag.IsRoot() {
				common.Lang.Set(tag)
			}
		} else if pdf.IsReadError(err) {
			return err
		}
	}

	return nil
}

// decodeHighlight reads an annotation's /H entry, supplying the default for
// a missing entry and normalising the deprecated "T" (toggle) mode to
// [annotation.HighlightPush].
func decodeHighlight(c pdf.Cursor, obj pdf.Object) (annotation.Highlight, error) {
	h, err := pdf.Optional(c.Name(obj))
	if err != nil {
		return "", err
	}
	switch h {
	case "":
		return annotation.HighlightInvert, nil
	case "T":
		return annotation.HighlightPush, nil
	default:
		return annotation.Highlight(h), nil
	}
}
