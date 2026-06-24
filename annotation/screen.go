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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation/appearance"
)

// PDF 2.0 sections: 12.5.2 12.5.6.18

// Screen specifies a region of a page upon which media clips may be played. It
// also serves as an object from which actions can be triggered. It is
// typically the target of a rendition action.
type Screen struct {
	Common

	// Title is the title of the screen annotation.
	//
	// This corresponds to the /T entry in the PDF annotation dictionary.
	Title string

	// Style (optional) is an appearance characteristics dictionary. Its Icon
	// provides the icon used in generating the appearance referred to by the
	// screen annotation's AP entry.
	//
	// This corresponds to the /MK entry in the PDF annotation dictionary.
	Style *appearance.Characteristics

	// Action (optional; PDF 1.1) is an action that is performed when the
	// annotation is activated.
	//
	// This corresponds to the /A entry in the PDF annotation dictionary.
	Action pdf.Action

	// AA (optional; PDF 1.2) is an additional-actions dictionary defining
	// the screen annotation's behaviour in response to various trigger events.
	//
	// This corresponds to the /AA entry in the PDF annotation dictionary.
	AA *triggers.Annotation
}

var _ Annotation = (*Screen)(nil)

// AnnotationType returns "Screen".
// This implements the [Annotation] interface.
func (s *Screen) AnnotationType() pdf.Name {
	return "Screen"
}

func (s *Screen) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "screen annotation", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Screen"),
	}

	// Add common annotation fields
	if err := s.Common.fillDict(rm, dict, isMarkup(s), false); err != nil {
		return nil, err
	}

	// Add screen-specific fields
	// T (optional)
	if s.Title != "" {
		dict["T"] = pdf.TextString(s.Title)
	}

	// MK (optional)
	if s.Style != nil {
		mk, err := rm.Embed(s.Style)
		if err != nil {
			return nil, err
		}
		dict["MK"] = mk
	}

	// A (optional)
	if s.Action != nil {
		encoded, err := s.Action.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["A"] = encoded
	}

	// AA (optional)
	if s.AA != nil {
		aa, err := s.AA.Encode(rm)
		if err != nil {
			return nil, err
		}
		if aa != nil {
			dict["AA"] = aa
		}
	}

	return dict, nil
}
