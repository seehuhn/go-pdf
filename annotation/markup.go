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
	"time"

	"seehuhn.de/go/pdf"
)

// Markup contains fields common to all markup annotations.
type Markup struct {
	// T (optional; PDF 1.1) is the text label that is displayed in the
	// title bar of the annotation's popup window when open and active. This
	// entry identifies the user who added the annotation.
	T string

	// Popup (optional; PDF 1.3) is an indirect reference to a popup annotation
	// for displaying the annotation's contents in a popup window.
	Popup pdf.Reference

	// RC (optional; PDF 1.5) is a rich contents stream or string, providing a
	// formatted representation of the annotation's contents that may be
	// displayed in place of the Contents entry.
	RC pdf.Object

	// CreationDate (optional; PDF 1.5) is the date and time when the
	// annotation was created.
	CreationDate time.Time

	// IRT (required if RT is present; PDF 1.5) is a reference to the
	// annotation that this annotation is "in reply to". Both annotations
	// are on the same page.
	IRT pdf.Reference

	// Subj (optional; PDF 1.5) is the subject of the annotation, typically
	// displayed in the title bar of the annotation's popup window.
	Subj string

	// RT (optional; PDF 1.6) specifies the relationship between this
	// annotation and the annotation specified by IRT. Valid values are
	// "R" (Reply) and "Group". Default value: "R".
	RT pdf.Name

	// IT (optional; PDF 1.6) describes the intent of the markup annotation.
	// Valid values vary by annotation type.
	IT pdf.Name
}

// fillDict adds the fields corresponding to the Markup struct
// to the given PDF dictionary.  If fields are not valid for the PDF version
// corresponding to the ResourceManager, an error is returned.
func (m *Markup) fillDict(rm *pdf.ResourceManager, d pdf.Dict) error {
	w := rm.Out

	if m.T != "" {
		if err := pdf.CheckVersion(w, "markup annotation T entry", pdf.V1_1); err != nil {
			return err
		}
		d["T"] = pdf.TextString(m.T)
	}

	if m.Popup != 0 {
		if err := pdf.CheckVersion(w, "markup annotation Popup entry", pdf.V1_3); err != nil {
			return err
		}
		d["Popup"] = m.Popup
	}

	if m.RC != nil {
		if err := pdf.CheckVersion(w, "markup annotation RC entry", pdf.V1_5); err != nil {
			return err
		}
		d["RC"] = m.RC
	}

	if !m.CreationDate.IsZero() {
		if err := pdf.CheckVersion(w, "markup annotation CreationDate entry", pdf.V1_5); err != nil {
			return err
		}
		d["CreationDate"] = pdf.Date(m.CreationDate)
	}

	if m.IRT != 0 {
		if err := pdf.CheckVersion(w, "markup annotation IRT entry", pdf.V1_5); err != nil {
			return err
		}
		d["IRT"] = m.IRT
	}

	if m.Subj != "" {
		if err := pdf.CheckVersion(w, "markup annotation Subj entry", pdf.V1_5); err != nil {
			return err
		}
		d["Subj"] = pdf.TextString(m.Subj)
	}

	if m.RT != "" {
		if err := pdf.CheckVersion(w, "markup annotation RT entry", pdf.V1_6); err != nil {
			return err
		}
		d["RT"] = m.RT
	}

	if m.IT != "" {
		if err := pdf.CheckVersion(w, "markup annotation IT entry", pdf.V1_6); err != nil {
			return err
		}
		d["IT"] = m.IT
	}

	return nil
}

// extractMarkup extracts fields common to all markup annotations from a PDF dictionary.
func extractMarkup(r pdf.Getter, dict pdf.Dict, markup *Markup) error {
	// T (optional)
	if t, err := pdf.GetTextString(r, dict["T"]); err == nil {
		markup.T = string(t)
	}

	// Popup (optional)
	if popup, ok := dict["Popup"].(pdf.Reference); ok {
		markup.Popup = popup
	}

	// RC (optional)
	if rc := dict["RC"]; rc != nil {
		markup.RC = rc
	}

	// CreationDate (optional)
	if creationDate, err := pdf.GetDate(r, dict["CreationDate"]); err == nil {
		markup.CreationDate = time.Time(creationDate)
	}

	// IRT (optional)
	if irt, ok := dict["IRT"].(pdf.Reference); ok {
		markup.IRT = irt
	}

	// Subj (optional)
	if subj, err := pdf.GetTextString(r, dict["Subj"]); err == nil {
		markup.Subj = string(subj)
	}

	// RT (optional)
	if rt, err := pdf.GetName(r, dict["RT"]); err == nil {
		markup.RT = rt
	}

	// IT (optional)
	if it, err := pdf.GetName(r, dict["IT"]); err == nil {
		markup.IT = it
	}

	return nil
}
