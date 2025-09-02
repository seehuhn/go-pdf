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
	"fmt"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.10

// TextMarkupType represents the type of text markup annotation.
type TextMarkupType pdf.Name

const (
	TextMarkupTypeHighlight TextMarkupType = "Highlight"
	TextMarkupTypeSquiggly  TextMarkupType = "Squiggly"
	TextMarkupTypeStrikeOut TextMarkupType = "StrikeOut"
	TextMarkupTypeUnderline TextMarkupType = "Underline"
)

// TextMarkup represents a text markup annotation that appears as highlights,
// underlines, strikeouts, or jagged ("squiggly") underlines in the text.
// When opened, it displays a pop-up window containing the text of the associated note.
type TextMarkup struct {
	Common
	Markup

	// Type specifies which type of text markup annotation this is.
	Type TextMarkupType

	// QuadPoints (required) specifies the coordinates of quadrilaterals that
	// comprise the region where the text markup should be applied. Each
	// quadrilateral encompasses a word or group of contiguous words in the text
	// underlying the annotation. Each quadrilateral is represented by 4 Vec2
	// points, giving the corners in counter-clockwise order, starting at the
	// bottom-left.
	QuadPoints []vec.Vec2
}

var _ Annotation = (*TextMarkup)(nil)

// AnnotationType returns the specific text markup type.
// This implements the [Annotation] interface.
func (t *TextMarkup) AnnotationType() pdf.Name {
	return pdf.Name(t.Type)
}

func decodeTextMarkup(r pdf.Getter, dict pdf.Dict, subtype pdf.Name) (*TextMarkup, error) {
	textMarkup := &TextMarkup{}

	textMarkup.Type = TextMarkupType(subtype)

	// Extract common annotation fields
	if err := decodeCommon(r, &textMarkup.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(r, dict, &textMarkup.Markup); err != nil {
		return nil, err
	}

	// Extract text markup-specific fields
	// QuadPoints (required)
	quadPoints, err := pdf.GetFloatArray(r, dict["QuadPoints"])
	if err != nil {
		return nil, fmt.Errorf("failed to read QuadPoints: %w", err)
	}
	if len(quadPoints) < 8 {
		return nil, errors.New("QuadPoints is required for text markup annotations and must contain at least one quadrilateral (8 values)")
	}

	// process floats in groups of 8, each group becomes 4 Vec2 points
	numCompleteQuads := len(quadPoints) / 8
	points := make([]vec.Vec2, numCompleteQuads*4)
	for quad := 0; quad < numCompleteQuads; quad++ {
		for corner := 0; corner < 4; corner++ {
			idx := quad*8 + corner*2
			points[quad*4+corner] = vec.Vec2{X: quadPoints[idx], Y: quadPoints[idx+1]}
		}
	}
	textMarkup.QuadPoints = points

	return textMarkup, nil
}

func (t *TextMarkup) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	// Check version requirements based on type
	switch t.Type {
	case TextMarkupTypeHighlight, TextMarkupTypeUnderline, TextMarkupTypeStrikeOut:
		if err := pdf.CheckVersion(rm.Out, string(t.Type)+" annotation", pdf.V1_3); err != nil {
			return nil, err
		}
	case TextMarkupTypeSquiggly:
		if err := pdf.CheckVersion(rm.Out, "squiggly annotation", pdf.V1_4); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown text markup type %q", t.Type)
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name(t.Type),
	}

	// Add common annotation fields
	if err := t.Common.fillDict(rm, dict, isMarkup(t)); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := t.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add text markup-specific fields
	// QuadPoints (required)
	if len(t.QuadPoints) == 0 {
		return nil, errors.New("QuadPoints is required for text markup annotations")
	}
	if len(t.QuadPoints)%4 != 0 {
		return nil, errors.New("length of QuadPoints is not a multiple of 4")
	}
	// convert Vec2 slice to float array for PDF
	quadArray := make(pdf.Array, len(t.QuadPoints)*2)
	for i, v := range t.QuadPoints {
		quadArray[i*2] = pdf.Number(v.X)
		quadArray[i*2+1] = pdf.Number(v.Y)
	}
	dict["QuadPoints"] = quadArray

	return dict, nil
}
