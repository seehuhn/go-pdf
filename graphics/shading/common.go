// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package shading

import (
	"errors"
	"fmt"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// Common holds the entries every shading dictionary has, whatever its shading
// type.  Each of the shading types embeds this, so its fields can be used as
// if they were declared on the type itself.
type Common = graphics.ShadingCommon

// commonEqual compares the entries common to all shading types.
func commonEqual(a, b *Common) bool {
	return color.SpacesEqual(a.ColorSpace, b.ColorSpace) &&
		slices.Equal(a.Background, b.Background) &&
		a.BBox.Equal(b.BBox) &&
		a.AntiAlias == b.AntiAlias
}

// spaceAllowed reports whether cs may be used as the colour space of a
// shading.  Shading types 1 to 3 map a colour space value through a function,
// so an Indexed colour space cannot be used with them.  Types 4 to 7 give
// colour values directly and allow it.
func spaceAllowed(cs color.Space, allowIndexed bool) bool {
	switch cs.Family() {
	case color.FamilyPattern:
		return false
	case color.FamilyIndexed:
		return allowIndexed
	}
	return true
}

// validateCommon checks the entries common to all shading types against the
// rules for writing a shading dictionary.
func validateCommon(c *Common, allowIndexed bool) error {
	if c.ColorSpace == nil {
		return errors.New("missing ColorSpace")
	}
	if !spaceAllowed(c.ColorSpace, allowIndexed) {
		return errors.New("invalid ColorSpace")
	}
	if n := len(c.Background); n > 0 && n != c.ColorSpace.Channels() {
		return fmt.Errorf("wrong number of background values: expected %d, got %d",
			c.ColorSpace.Channels(), n)
	}
	return nil
}

// embedCommon writes the optional entries common to all shading types.
// ColorSpace is written by the caller, which needs the embedded value for the
// dictionary anyway.
func embedCommon(dict pdf.Dict, c *Common) {
	if len(c.Background) > 0 {
		dict["Background"] = toPDF(c.Background)
	}
	if c.BBox != nil {
		dict["BBox"] = c.BBox
	}
	if c.AntiAlias {
		dict["AntiAlias"] = pdf.Boolean(true)
	}
}

// extractCommon reads the entries common to all shading types.  ColorSpace is
// required and must be usable for a shading of this type, so that the result
// can be written back out; a malformed value for any of the other entries is
// ignored, leaving the zero value in place.
func extractCommon(c pdf.Cursor, d pdf.Dict, out *Common, allowIndexed bool) error {
	csObj, ok := d["ColorSpace"]
	if !ok {
		return &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /ColorSpace entry"),
		}
	}
	cs, err := pdf.Decode(c, csObj, color.ExtractSpace)
	if err != nil {
		return err
	}
	if !spaceAllowed(cs, allowIndexed) {
		return pdf.Error("invalid shading ColorSpace")
	}
	out.ColorSpace = cs

	if bgObj, ok := d["Background"]; ok {
		if bg, err := pdf.Optional(c.FloatArray(bgObj)); err != nil {
			return err
		} else if len(bg) > 0 && len(bg) == cs.Channels() {
			out.Background = bg
		}
		// a wrong number of background values is ignored
	}

	if bboxObj, ok := d["BBox"]; ok {
		if bbox, err := pdf.Optional(c.Rectangle(bboxObj)); err != nil {
			return err
		} else if bbox != nil {
			out.BBox = bbox
		}
	}

	if aaObj, ok := d["AntiAlias"]; ok {
		if aa, err := pdf.Optional(c.Boolean(aaObj)); err != nil {
			return err
		} else {
			out.AntiAlias = bool(aa)
		}
	}

	return nil
}
