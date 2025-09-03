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

func decodeRichMedia(x *pdf.Extractor, dict pdf.Dict) (*RichMedia, error) {
	richMedia := &RichMedia{}

	// Extract common annotation fields
	if err := decodeCommon(x, &richMedia.Common, dict); err != nil {
		return nil, err
	}

	// RichMediaContent (required)
	if content, ok := dict["RichMediaContent"].(pdf.Reference); ok {
		richMedia.RichMediaContent = content
	}

	// RichMediaSettings (optional)
	if settings, ok := dict["RichMediaSettings"].(pdf.Reference); ok {
		richMedia.RichMediaSettings = settings
	}

	return richMedia, nil
}

func (r *RichMedia) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "rich media annotation", pdf.V2_0); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("RichMedia"),
	}

	// Add common annotation fields
	if err := r.Common.fillDict(rm, dict, isMarkup(r)); err != nil {
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
