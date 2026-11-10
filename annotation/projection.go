// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
