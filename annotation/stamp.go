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
	"fmt"

	"seehuhn.de/go/pdf"
)

// Stamp represents a rubber stamp annotation that displays text or graphics
// intended to look as if they were stamped on the page with a rubber stamp.
// When opened, it displays a popup window containing the text of the associated note.
type Stamp struct {
	Common
	Markup

	// Name (optional) is the name of an icon that is used in displaying
	// the annotation. Standard names include:
	// Approved, Experimental, NotApproved, AsIs, Expired, NotForPublicRelease,
	// Confidential, Final, Sold, Departmental, ForComment, TopSecret, Draft,
	// ForPublicRelease.
	// Default value: "Draft".
	// This field is not present if IT is present and its value is not "Stamp".
	Name pdf.Name

	// IT (optional; PDF 2.0) describes the intent of the stamp annotation.
	// Valid values:
	// "StampSnapshot" - appearance taken from preexisting PDF content
	// "StampImage" - appearance is an image
	// "Stamp" - appearance is a rubber stamp
	// Default value: "Stamp"
	// Note: This field is inherited from Markup but has specific meaning for stamps.
}

var _ pdf.Annotation = (*Stamp)(nil)

// AnnotationType returns "Stamp".
// This implements the [pdf.Annotation] interface.
func (s *Stamp) AnnotationType() pdf.Name {
	return "Stamp"
}

func extractStamp(r pdf.Getter, dict pdf.Dict) (*Stamp, error) {
	stamp := &Stamp{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &stamp.Common); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &stamp.Markup); err != nil {
		return nil, err
	}

	// Extract stamp-specific fields
	// Per spec: "If the IT key is present and its value is not Stamp, this Name key is not present"
	// If IT is present and not "Stamp", ignore any Name field in the PDF (it's invalid)
	if stamp.Intent != "" && stamp.Intent != "Stamp" {
		// IT is present and not "Stamp" - ignore Name field, use default
		stamp.Name = "Draft"
	} else {
		// IT is empty or "Stamp" - Name can be present
		if name, err := pdf.GetName(r, dict["Name"]); err == nil && name != "" {
			stamp.Name = name
		} else {
			// Default value when Name is not present in PDF
			stamp.Name = "Draft"
		}
	}

	// Note: IT field is already handled by extractMarkup since it's part of the Markup struct

	return stamp, nil
}

func (s *Stamp) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	ref := rm.Out.Alloc()
	err := s.EmbedAt(rm, ref)
	return ref, pdf.Unused{}, err
}

func (s *Stamp) EmbedAt(rm *pdf.ResourceManager, ref pdf.Reference) error {
	if err := pdf.CheckVersion(rm.Out, "stamp annotation", pdf.V1_3); err != nil {
		return err
	}

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Stamp"),
	}

	// Add common annotation fields
	if err := s.Common.fillDict(rm, dict); err != nil {
		return err
	}

	// Add markup annotation fields
	if err := s.Markup.fillDict(rm, dict); err != nil {
		return err
	}

	// Add stamp-specific fields
	// Per spec: "If the IT key is present and its value is not Stamp, this Name key is not present"
	if s.Intent != "" && s.Intent != "Stamp" {
		// IT is present and not "Stamp" - Name is not present
		if s.Name != "" && s.Name != "Draft" {
			return fmt.Errorf("stamp annotation: Name field is not present when IT is %q", s.Intent)
		}
		// Don't write Name field
	} else {
		// IT is empty or "Stamp" - Name can be present
		// Name (optional) - only write if not the default value "Draft"
		if s.Name != "" && s.Name != "Draft" {
			dict["Name"] = s.Name
		}
	}

	// Note: IT field is already handled by fillDict in the Markup struct

	return rm.Out.Put(ref, dict)
}
