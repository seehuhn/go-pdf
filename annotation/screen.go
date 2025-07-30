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

// Screen specifies a region of a page upon which media clips may be played. It
// also serves as an object from which actions can be triggered.
type Screen struct {
	Common

	// T (optional) is the title of the screen annotation.
	T pdf.TextString

	// MK (optional) is an appearance characteristics dictionary. The I entry
	// of this dictionary provides the icon used in generating the appearance
	// referred to by the screen annotation's AP entry.
	MK pdf.Reference

	// A (optional; PDF 1.1) is an action that is performed when the
	// annotation is activated.
	A pdf.Reference

	// AA (optional; PDF 1.2) is an additional-actions dictionary defining
	// the screen annotation's behaviour in response to various trigger events.
	AA pdf.Reference
}

var _ Annotation = (*Screen)(nil)

// AnnotationType returns "Screen".
// This implements the [Annotation] interface.
func (s *Screen) AnnotationType() pdf.Name {
	return "Screen"
}

func extractScreen(r pdf.Getter, dict pdf.Dict) (*Screen, error) {
	screen := &Screen{}

	// Extract common annotation fields
	if err := decodeCommon(r, &screen.Common, dict); err != nil {
		return nil, err
	}

	// Extract screen-specific fields
	// T (optional)
	if t, err := pdf.GetTextString(r, dict["T"]); err == nil && t != "" {
		screen.T = t
	}

	// MK (optional)
	if mk, ok := dict["MK"].(pdf.Reference); ok {
		screen.MK = mk
	}

	// A (optional)
	if a, ok := dict["A"].(pdf.Reference); ok {
		screen.A = a
	}

	// AA (optional)
	if aa, ok := dict["AA"].(pdf.Reference); ok {
		screen.AA = aa
	}

	return screen, nil
}

func (s *Screen) Encode(rm *pdf.ResourceManager) (pdf.Dict, error) {
	if err := pdf.CheckVersion(rm.Out, "screen annotation", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Screen"),
	}

	// Add common annotation fields
	if err := s.Common.fillDict(rm, dict, isMarkup(s)); err != nil {
		return nil, err
	}

	// Add screen-specific fields
	// T (optional)
	if s.T != "" {
		dict["T"] = s.T
	}

	// MK (optional)
	if s.MK != 0 {
		dict["MK"] = s.MK
	}

	// A (optional)
	if s.A != 0 {
		dict["A"] = s.A
	}

	// AA (optional)
	if s.AA != 0 {
		dict["AA"] = s.AA
	}

	return dict, nil
}
