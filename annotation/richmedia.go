// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package annotation

import "seehuhn.de/go/pdf"

// PDF 2.0 sections: 12.5.2 12.5.6.25

// RichMedia represents a rich media annotation (PDF 2.0).
// Rich media annotations share structural similarities with 3D artwork and can
// contain various types of rich media content like 3D models, sound, and video.
type RichMedia struct {
	Common

	// RichMediaContent (required) is a RichMediaContent dictionary that stores
	// the rich media artwork and information as to how it should be configured and viewed.
	RichMediaContent pdf.Reference

	// RichMediaSettings (optional) is a RichMediaSettings dictionary that stores
	// conditions and responses that determine when the annotation should be activated
	// and deactivated by an interactive PDF processor.
	RichMediaSettings pdf.Reference
}

var _ Annotation = (*RichMedia)(nil)

// AnnotationType returns "RichMedia".
func (r *RichMedia) AnnotationType() pdf.Name {
	return "RichMedia"
}

func (r *RichMedia) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "rich media annotation", pdf.V2_0); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("RichMedia"),
	}

	// Add common annotation fields
	if err := r.Common.fillDict(rm, dict, isMarkup(r), false); err != nil {
		return nil, err
	}

	// RichMediaContent (required)
	if r.RichMediaContent != 0 {
		dict["RichMediaContent"] = r.RichMediaContent
	}

	// RichMediaSettings (optional)
	if r.RichMediaSettings != 0 {
		dict["RichMediaSettings"] = r.RichMediaSettings
	}

	return dict, nil
}
