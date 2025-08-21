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
)

// PDF 2.0 sections: 12.5.2 12.5.6.5

// Link represents a hypertext link annotation.
// This is a clickable region on a PDF page that can perform an action
// or navigate to a destination when activated.
type Link struct {
	Common

	// Action (optional) is an action that is performed when the link
	// annotation is activated. Mutually exclusive with Destination.
	//
	// This corresponds to the /A entry in the PDF annotation dictionary.
	Action pdf.Object

	// Destination (optional) is a destination that is displayed when the
	// annotation is activated. Mutually exclusive with Action.
	//
	// This corresponds to the /Dest entry in the PDF annotation dictionary.
	Destination pdf.Object

	// Highlight is the annotation's highlighting mode.
	//
	// When writing annotations, an empty name can be used as a shorthand
	// [LinkHighlightInvert].
	//
	// This corresponds to the /H entry in the PDF annotation dictionary.
	Highlight LinkHighlight

	// QuadPoints (optional) specifies the coordinates of
	// quadrilaterals that comprise the region where the link should be
	// activated. Each quadrilateral is represented by 4 Vec2 points.
	//
	// All points must be contained within Common.Rect.
	QuadPoints []vec.Vec2

	// BorderStyle (optional) is a border style dictionary specifying the line width
	// and dash pattern for drawing the annotation's border.
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

func decodeLink(r pdf.Getter, dict pdf.Dict) (*Link, error) {
	link := &Link{}

	// Extract common annotation fields
	if err := decodeCommon(r, &link.Common, dict); err != nil {
		return nil, err
	}

	// Extract link-specific fields
	if a := dict["A"]; a != nil {
		link.Action = a
	}
	if dest := dict["Dest"]; dest != nil {
		link.Destination = dest
	}
	if link.Action != nil {
		link.Destination = nil // mutually exclusive fields
	}

	if h, _ := pdf.GetName(r, dict["H"]); h != "" {
		link.Highlight = LinkHighlight(h)
	} else {
		link.Highlight = LinkHighlightInvert // default value
	}

	if pa, ok := dict["PA"].(pdf.Reference); ok {
		link.Backup = pa
	}

	if quadPoints, err := pdf.GetFloatArray(r, dict["QuadPoints"]); err == nil && len(quadPoints) > 0 {
		// convert float array to Vec2 slice
		if len(quadPoints)%2 == 0 {
			points := make([]vec.Vec2, len(quadPoints)/2)
			for i := 0; i < len(quadPoints); i += 2 {
				points[i/2] = vec.Vec2{X: quadPoints[i], Y: quadPoints[i+1]}
			}
			link.QuadPoints = points
		}
	}

	// BS (optional)
	link.BorderStyle, _ = ExtractBorderStyle(r, dict["BS"])

	return link, nil
}

func (l *Link) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if l.Action != nil && l.Destination != nil {
		return nil, errors.New("conflicting Action and Destination fields in Link annotation")
	}
	if len(l.QuadPoints)%4 != 0 {
		return nil, errors.New("length of QuadPoints is not a multiple of 4")
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Link"),
	}

	// Add common annotation fields
	if err := l.Common.fillDict(rm, dict, isMarkup(l)); err != nil {
		return nil, err
	}

	// Add link-specific fields
	if l.Action != nil {
		if err := pdf.CheckVersion(rm.Out, "link annotation A entry", pdf.V1_1); err != nil {
			return nil, err
		}
		dict["A"] = l.Action
	} else if l.Destination != nil {
		dict["Dest"] = l.Destination
	}

	if l.Highlight != "" {
		if err := pdf.CheckVersion(rm.Out, "link annotation H entry", pdf.V1_2); err != nil {
			return nil, err
		}
		if l.Highlight != LinkHighlightInvert {
			dict["H"] = pdf.Name(l.Highlight)
		}
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
		ref, _, err := pdf.ResourceManagerEmbed(rm, l.BorderStyle)
		if err != nil {
			return nil, err
		}
		dict["BS"] = ref
	}

	return dict, nil
}

// LinkHighlight represents a highlighting mode for a link annotation.
// The valid names are provided as constants.
type LinkHighlight pdf.Name

// Valid values for the [LinkHighlight] type.
const (
	LinkHighlightNone    LinkHighlight = "N" // no highlighting
	LinkHighlightInvert  LinkHighlight = "I" // invert the annotation rectangle
	LinkHighlightOutline LinkHighlight = "O" // invert the border
	LinkHighlightPush    LinkHighlight = "P" // push-down effect
)
