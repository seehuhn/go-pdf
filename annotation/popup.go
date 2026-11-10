// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package annotation

import "seehuhn.de/go/pdf"

// PDF 2.0 sections: 12.5.2 12.5.6.14

// Popup represents a popup annotation that displays text in a pop-up window
// for entry and editing. It does not appear alone but is associated with a
// markup annotation (its parent annotation) and is used for editing the
// parent's text.
type Popup struct {
	Common

	// Parent (optional; indirect reference) is the parent annotation with
	// which this popup annotation is associated. If this entry is present, the
	// parent's Common.Contents, Common.LastModified, Common.Color and
	// Markup.User entries override those of the popup annotation itself.
	Parent pdf.Reference

	// Open (optional) is a flag specifying whether the popup annotation is
	// initially displayed open. Default value: false (closed).
	Open bool
}

var _ Annotation = (*Popup)(nil)

// AnnotationType returns "Popup".
// This implements the [Annotation] interface.
func (p *Popup) AnnotationType() pdf.Name {
	return "Popup"
}

func (p *Popup) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "popup annotation", pdf.V1_3); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Popup"),
	}

	// Add common annotation fields
	if err := p.Common.fillDict(rm, dict, isMarkup(p), false); err != nil {
		return nil, err
	}

	// Add popup-specific fields
	// Parent (optional)
	if p.Parent != 0 {
		dict["Parent"] = p.Parent
	}

	// Open (optional) - only write if true (default is false)
	if p.Open {
		dict["Open"] = pdf.Boolean(p.Open)
	}

	return dict, nil
}
