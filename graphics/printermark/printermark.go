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

// Package printermark implements the form XObject entries specific to
// printer's marks.
//
// A printer's mark is a graphic symbol such as a registration target, colour
// bar or cut mark, added to a page to assist production personnel.  Its
// visual presentation is a form XObject appearing as an appearance stream in
// the N entry of a printer's mark annotation's appearance dictionary.  Unlike
// group XObjects and reference XObjects, a printer's mark form is not
// identified by an entry in the form dictionary; it is a printer's mark
// because of where it is referenced from.
//
// The entries represented by [Attributes] are stored directly in the form
// dictionary, alongside the normal form XObject entries.
package printermark

import (
	"fmt"
	"maps"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 14.11.3

// Attributes holds the form dictionary entries specific to a printer's mark.
type Attributes struct {
	// MarkStyle (optional) describes the printer's mark to the user.
	MarkStyle string

	// Colorants (optional) identifies the individual colourants associated
	// with the printer's mark, such as a colour bar.  Each key must equal the
	// colorant name of the Separation colour space it maps to.
	Colorants map[pdf.Name]*color.SpaceSeparation
}

// Equal reports whether two Attributes are equal.
func (a *Attributes) Equal(other *Attributes) bool {
	if a == nil || other == nil {
		return a == other
	}
	if a.MarkStyle != other.MarkStyle {
		return false
	}
	return maps.EqualFunc(a.Colorants, other.Colorants,
		func(x, y *color.SpaceSeparation) bool {
			return color.SpacesEqual(x, y)
		})
}

// FillDict adds the printer's mark entries to a form XObject dictionary.
//
// The entries are stored directly in the form dictionary, so this cannot
// use the [pdf.Embedder] interface.
//
// Every entry in Colorants must have a colour space whose colorant name
// equals the key it is stored under.
func (a *Attributes) FillDict(e *pdf.EmbedHelper, dict pdf.Dict) error {
	if err := pdf.CheckVersion(e.Out(), "printer's mark form XObject", pdf.V1_4); err != nil {
		return err
	}

	if a.MarkStyle != "" {
		dict["MarkStyle"] = pdf.TextString(a.MarkStyle)
	}

	if len(a.Colorants) > 0 {
		colorants := pdf.Dict{}
		for name, space := range a.Colorants {
			if space == nil {
				return fmt.Errorf("colorant %q has no colour space", name)
			}
			if space.Colorant != name {
				return fmt.Errorf("colorant %q uses colour space for %q", name, space.Colorant)
			}
			obj, err := e.Embed(space)
			if err != nil {
				return err
			}
			colorants[name] = obj
		}
		dict["Colorants"] = colorants
	}

	return nil
}

// ExtractAttributes reads the printer's mark entries from a form XObject
// dictionary.  It returns nil if the dictionary uses none of them, or if none
// of the values are usable.
//
// Colorants entries which are not Separation colour spaces, and those whose
// colorant name does not match the key they are stored under, are ignored.
func ExtractAttributes(c pdf.Cursor, dict pdf.Dict) (*Attributes, error) {
	_, hasStyle := dict["MarkStyle"]
	_, hasColorants := dict["Colorants"]
	if !hasStyle && !hasColorants {
		return nil, nil
	}

	a := &Attributes{}

	if style, err := pdf.Optional(c.TextString(dict["MarkStyle"])); err != nil {
		return nil, err
	} else {
		a.MarkStyle = string(style)
	}

	colorants, err := pdf.Optional(c.Dict(dict["Colorants"]))
	if err != nil {
		return nil, err
	}
	for name, obj := range colorants {
		space, err := pdf.DecodeOptional(c, obj, color.ExtractSpace)
		if err != nil {
			return nil, err
		}
		// The colorant name is written back from the colour space, so an
		// entry filed under a different name could not be written back
		// unchanged.  Such entries are dropped instead.
		sep, ok := space.(*color.SpaceSeparation)
		if !ok || sep.Colorant != name {
			continue
		}
		if a.Colorants == nil {
			a.Colorants = make(map[pdf.Name]*color.SpaceSeparation)
		}
		a.Colorants[name] = sep
	}

	// Both entries are optional, so a dictionary whose printer's mark entries
	// are all empty or unusable is indistinguishable from one which has none.
	// Report it as such, so that reading the result back gives the same value.
	if a.MarkStyle == "" && len(a.Colorants) == 0 {
		return nil, nil
	}

	return a, nil
}
