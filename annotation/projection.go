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

// Projection represents a projection annotation (PDF 2.0).
// Projection annotations are markup annotation subtypes that provide a way to
// save 3D and other specialised measurements and comments as markup annotations.
type Projection struct {
	Common
	Markup

	// ExData (optional) is an external data dictionary. When used in conjunction
	// with a 3D measurement, it has a Subtype of "3DM" and contains an M3DREF
	// entry that references a 3D measurement dictionary.
	ExData pdf.Reference
}

var _ pdf.Annotation = (*Projection)(nil)

// AnnotationType returns "Projection".
func (p *Projection) AnnotationType() pdf.Name {
	return "Projection"
}

func extractProjection(r pdf.Getter, dict pdf.Dict) (*Projection, error) {
	projection := &Projection{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &projection.Common); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &projection.Markup); err != nil {
		return nil, err
	}

	// ExData (optional)
	if exData, ok := dict["ExData"].(pdf.Reference); ok {
		projection.ExData = exData
	}

	return projection, nil
}

func (p *Projection) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	ref := rm.Out.Alloc()
	err := p.EmbedAt(rm, ref)
	return ref, pdf.Unused{}, err
}

func (p *Projection) EmbedAt(rm *pdf.ResourceManager, ref pdf.Reference) error {
	if err := pdf.CheckVersion(rm.Out, "projection annotation", pdf.V2_0); err != nil {
		return err
	}

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Projection"),
	}

	// Add common annotation fields
	if err := p.Common.fillDict(rm, dict); err != nil {
		return err
	}

	// Add markup annotation fields
	if err := p.Markup.fillDict(rm, dict); err != nil {
		return err
	}

	// ExData (optional)
	if p.ExData != 0 {
		dict["ExData"] = p.ExData
	}

	return rm.Out.Put(ref, dict)
}
