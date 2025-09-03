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

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.12

// Stamp represents a rubber stamp annotation that displays text or graphics
// intended to look as if they were stamped on the page with a rubber stamp.
// When opened, it displays a pop-up window containing the text of the associated note.
type Stamp struct {
	Common
	Markup

	// Icon is the name of an icon that is used in displaying the annotation.
	// The standard icon names are Approved, Experimental, NotApproved, AsIs,
	// Expired, NotForPublicRelease, Confidential, Final, Sold, Departmental,
	// ForComment, TopSecret, Draft, and ForPublicRelease.
	//
	// When writing annotations, an empty Icon name can be used as a shorthand
	// for [StampIconDraft].
	//
	// If the [Markup.Intent] field is present and its value is not
	// [StampIntentStamp], the Icon field must be empty.
	//
	// This corresponds to the /Name entry in the PDF annotation dictionary.
	Icon StampIcon
}

var _ Annotation = (*Stamp)(nil)

// AnnotationType returns "Stamp".
// This implements the [Annotation] interface.
func (s *Stamp) AnnotationType() pdf.Name {
	return "Stamp"
}

func decodeStamp(x *pdf.Extractor, dict pdf.Dict) (*Stamp, error) {
	r := x.R
	stamp := &Stamp{}

	// Extract common annotation fields
	if err := decodeCommon(x, &stamp.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(x, dict, &stamp.Markup); err != nil {
		return nil, err
	}

	if stamp.Intent == "" {
		stamp.Intent = StampIntentStamp
	}

	if stamp.Intent == StampIntentStamp {
		if icon, err := pdf.Optional(pdf.GetName(r, dict["Name"])); err != nil {
			return nil, err
		} else if icon != "" {
			stamp.Icon = StampIcon(icon)
		} else {
			stamp.Icon = StampIconDraft
		}
	}

	return stamp, nil
}

func (s *Stamp) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "stamp annotation", pdf.V1_3); err != nil {
		return nil, err
	}

	if s.Intent != "" {
		if err := pdf.CheckVersion(rm.Out, "stamp annotation with IT field", pdf.V2_0); err != nil {
			return nil, err
		}
		if s.Intent != StampIntentStamp && s.Icon != "" {
			return nil, fmt.Errorf("unexpected Icon name %q", s.Icon)
		}
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Stamp"),
	}

	if err := s.Common.fillDict(rm, dict, isMarkup(s)); err != nil {
		return nil, err
	}
	if err := s.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	if s.Icon != "" && s.Icon != StampIconDraft {
		dict["Name"] = pdf.Name(s.Icon)
	}
	if s.Intent == StampIntentStamp {
		// default value for stamp annotations
		delete(dict, "IT")
	}

	return dict, nil
}

// StampIcon represents the name of an icon used in displaying a stamp annotation.
// The standard names defined by the PDF specification are provided as constants.
// Other names may be used, but support is viewer dependent.
type StampIcon pdf.Name

const (
	StampIconApproved            = "Approved"
	StampIconExperimental        = "Experimental"
	StampIconNotApproved         = "NotApproved"
	StampIconAsIs                = "AsIs"
	StampIconExpired             = "Expired"
	StampIconNotForPublicRelease = "NotForPublicRelease"
	StampIconConfidential        = "Confidential"
	StampIconFinal               = "Final"
	StampIconSold                = "Sold"
	StampIconDepartmental        = "Departmental"
	StampIconForComment          = "ForComment"
	StampIconTopSecret           = "TopSecret"
	StampIconDraft               = "Draft"
	StampIconForPublicRelease    = "ForPublicRelease"
)

// These constants represent the allowed values for the Markup.Intent
// field in stamp annotations.
const (
	StampIntentStamp    pdf.Name = "Stamp"
	StampIntentImage    pdf.Name = "StampImage"
	StampIntentSnapshot pdf.Name = "StampSnapshot"
)
