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

func extractPopup(r pdf.Getter, dict pdf.Dict, singleUse bool) (*Popup, error) {
	popup := &Popup{}

	// Extract common annotation fields
	if err := extractCommon(r, &popup.Common, dict, singleUse); err != nil {
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
	dict, err := p.AsDict(rm)
	if err != nil {
		return nil, pdf.Unused{}, err
	}

	if p.SingleUse {
		return dict, pdf.Unused{}, nil
	}

	ref := rm.Out.Alloc()
	err = rm.Out.Put(ref, dict)
	return ref, pdf.Unused{}, err
}

func (p *Popup) AsDict(rm *pdf.ResourceManager) (pdf.Dict, error) {
	if err := pdf.CheckVersion(rm.Out, "popup annotation", pdf.V1_3); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Popup"),
	}

	// Add common annotation fields
	if err := p.Common.fillDict(rm, dict); err != nil {
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
