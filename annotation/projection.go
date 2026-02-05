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

import "seehuhn.de/go/pdf"

// PDF 2.0 sections: 12.5.2 12.5.6.24

// Projection represents a projection annotation (PDF 2.0).
// Projection annotations are markup annotation subtypes that provide a way to
// save 3D and other specialised measurements and comments as markup annotations.
type Projection struct {
	Common
	Markup
}

var _ Annotation = (*Projection)(nil)

// AnnotationType returns "Projection".
func (p *Projection) AnnotationType() pdf.Name {
	return "Projection"
}

func decodeProjection(x *pdf.Extractor, dict pdf.Dict) (*Projection, error) {
	projection := &Projection{}

	// Extract common annotation fields
	if err := decodeCommon(x, &projection.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(x, dict, &projection.Markup); err != nil {
		return nil, err
	}

	return projection, nil
}

func (p *Projection) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "projection annotation", pdf.V2_0); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Projection"),
	}

	// Add common annotation fields
	if err := p.Common.fillDict(rm, dict, isMarkup(p), false); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := p.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// ExData (optional)

	return dict, nil
}
