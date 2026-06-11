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

package annotation

import (
	"errors"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/destination"
)

// PDF 2.0 sections: 12.5.2 12.5.6.5

// Link represents a hypertext link annotation. This is a clickable region on a
// PDF page that can perform an action or navigate to a destination when
// activated.
//
// The only graphical indication of a link is an optional border. The border
// color is specified in the Common.Color field, and the line width and style
// are specified in the BorderStyle or Common.Border fields.
// Many PDF viewers will display the contents of the Common.Contents field
// as a tooltip when the pointer hovers over the link.
type Link struct {
	Common

	// Action (optional) is an action that is performed when the link
	// annotation is activated. Mutually exclusive with Destination.
	//
	// This corresponds to the /A entry in the PDF annotation dictionary.
	Action pdf.Action

	// Destination (optional) is a destination that is displayed when the
	// annotation is activated. Mutually exclusive with Action.
	//
	// This corresponds to the /Dest entry in the PDF annotation dictionary.
	Destination destination.Destination

	// Highlight is the annotation's highlighting mode.
	//
	// When writing annotations, an empty name can be used as a shorthand
	// for [HighlightInvert].
	//
	// This corresponds to the /H entry in the PDF annotation dictionary.
	Highlight Highlight

	// QuadPoints (optional) specifies the coordinates of quadrilaterals that
	// comprise the region where the link should be activated. Each
	// quadrilateral is represented by 4 Vec2 points, giving the corners in
	// counter-clockwise order, starting at the bottom-left.  If QuadPoints is
	// absent, Common.Rect is used instead.
	//
	// All points must be contained within Common.Rect.
	//
	// If QuadPoints is present for link annotations with a border, PDF viewers
	// disagree on where and how any borders should be drawn.
	QuadPoints []vec.Vec2

	// BorderStyle (optional) is a border style dictionary specifying the line
	// width and dash pattern for drawing the annotation's border.
	//
	// If this field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// Backup (optional) is an URI action formerly associated with this
	// annotation, used for reverting go-to actions back to URI actions.
	//
	// This corresponds to the /PA entry in the PDF annotation dictionary.
	Backup pdf.Reference
}

var _ Annotation = (*Link)(nil)

// AnnotationType returns "Link".
// This implements the [Annotation] interface.
func (l *Link) AnnotationType() pdf.Name {
	return "Link"
}

func (l *Link) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if l.Action != nil && l.Destination != nil {
		return nil, errors.New("conflicting Action and Destination fields in Link annotation")
	}
	if len(l.QuadPoints)%4 != 0 {
		return nil, errors.New("length of QuadPoints is not a multiple of 4")
	}
	if l.BorderStyle != nil && l.Common.Border != nil {
		return nil, errors.New("Border and BorderStyle are mutually exclusive")
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Link"),
	}

	// Add common annotation fields
	if err := l.Common.fillDict(rm, dict, isMarkup(l), l.BorderStyle != nil); err != nil {
		return nil, err
	}

	// Add link-specific fields
	if l.Action != nil {
		if err := pdf.CheckVersion(rm.Out, "link annotation A entry", pdf.V1_1); err != nil {
			return nil, err
		}
		encoded, err := l.Action.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["A"] = encoded
	} else if l.Destination != nil {
		encoded, err := l.Destination.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["Dest"] = encoded
	}

	if err := l.Highlight.encodeEntry(rm, dict, "link annotation H entry"); err != nil {
		return nil, err
	}

	if l.Backup != 0 {
		if err := pdf.CheckVersion(rm.Out, "link annotation PA entry", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["PA"] = l.Backup
	}

	if l.QuadPoints != nil {
		if err := pdf.CheckVersion(rm.Out, "link annotation QuadPoints entry", pdf.V1_6); err != nil {
			return nil, err
		}
		// convert Vec2 slice to float array for PDF
		quadArray := make(pdf.Array, len(l.QuadPoints)*2)
		for i, v := range l.QuadPoints {
			quadArray[i*2] = pdf.Number(v.X)
			quadArray[i*2+1] = pdf.Number(v.Y)
		}
		dict["QuadPoints"] = quadArray
	}

	if l.BorderStyle != nil {
		if err := pdf.CheckVersion(rm.Out, "link annotation BS entry", pdf.V1_6); err != nil {
			return nil, err
		}
		ref, err := rm.Embed(l.BorderStyle)
		if err != nil {
			return nil, err
		}
		dict["BS"] = ref
	}

	return dict, nil
}
