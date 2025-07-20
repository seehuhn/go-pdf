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

// Popup represents a popup annotation that displays text in a popup window
// for entry and editing. It does not appear alone but is associated with a
// markup annotation (its parent annotation) and is used for editing the
// parent's text.
type Popup struct {
	Common

	// Parent (optional; indirect reference) is the parent annotation
	// with which this popup annotation is associated. If this entry is
	// present, the parent annotation's Contents, M, C, and T entries override
	// those of the popup annotation itself.
	Parent pdf.Reference

	// Open (optional) is a flag specifying whether the popup annotation is
	// initially displayed open. Default value: false (closed).
	Open bool
}

var _ pdf.Annotation = (*Popup)(nil)

// AnnotationType returns "Popup".
// This implements the [pdf.Annotation] interface.
func (p *Popup) AnnotationType() pdf.Name {
	return "Popup"
}

func extractPopup(r pdf.Getter, dict pdf.Dict) (*Popup, error) {
	popup := &Popup{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &popup.Common); err != nil {
		return nil, err
	}

	// Extract popup-specific fields
	// Parent (optional)
	if parent, ok := dict["Parent"].(pdf.Reference); ok {
		popup.Parent = parent
	}

	// Open (optional)
	if open, err := pdf.GetBoolean(r, dict["Open"]); err == nil {
		popup.Open = bool(open)
	}

	return popup, nil
}

func (p *Popup) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	ref := rm.Out.Alloc()
	if _, err := p.EmbedAt(rm, ref); err != nil {
		return nil, zero, err
	}
	return ref, zero, nil
}

func (p *Popup) EmbedAt(rm *pdf.ResourceManager, ref pdf.Reference) (pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "popup annotation", pdf.V1_3); err != nil {
		return zero, err
	}

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Popup"),
	}

	// Add common annotation fields
	if err := p.Common.fillDict(rm, dict); err != nil {
		return zero, err
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

	err := rm.Out.Put(ref, dict)
	if err != nil {
		return zero, err
	}

	return zero, nil
}
