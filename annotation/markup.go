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
	"time"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.5.6.2

// Markup contains fields common to all markup annotations.
type Markup struct {
	// User (optional) is the text label that is displayed in the
	// title bar of the annotation's pop-up window when open and active. This
	// entry identifies the user who added the annotation.
	//
	// This corresponds to the /T entry in the PDF annotation dictionary.
	User string

	// Popup (optional) is used for displaying the annotation's contents in a
	// pop-up window when the annotation is open.
	Popup pdf.Reference

	// RC (optional) is a rich-text value providing a formatted representation
	// of the annotation's contents. When both Contents and RC are present,
	// their textual content should be equivalent.
	RC *pdf.StringOrStream

	// CreationDate (optional) is the date and time when the
	// annotation was created.
	CreationDate time.Time

	// InReplyTo (required if RT is present) is a reference to the
	// annotation that this annotation is "in reply to". Both annotations
	// are on the same page.
	//
	// This corresponds to the /IRT entry in the PDF annotation dictionary.
	InReplyTo pdf.Reference

	// Subject (optional) is the subject of the annotation, typically displayed
	// in the title bar of the annotation's pop-up window.
	//
	// This corresponds to the /Subj entry in the PDF annotation dictionary.
	Subject string

	// RT specifies the relationship between this annotation and the annotation
	// specified by InReplyTo. Valid values are "R" (Reply) and "Group".
	//
	// When writing annotations, an empty RT value can be used as a shorthand
	// for "R".
	RT pdf.Name

	// Intent (optional) describes the intent of the markup annotation. Valid
	// values vary by annotation type and allow processors to distinguish
	// between different uses of the same annotation type.
	//
	// This corresponds to the /IT entry in the PDF annotation dictionary.
	Intent pdf.Name

	// ExData (optional) specifies an external data dictionary
	// associated with the annotation.
	ExData pdf.Object
}

// GetMarkup returns the markup fields.
func (m *Markup) GetMarkup() *Markup {
	return m
}

// MarkupAnnotation is the interface implemented by all markup annotations
// (§12.5.6.2).  A type assertion to MarkupAnnotation tests whether a given
// [Annotation] is a markup annotation; GetMarkup gives access to the fields
// shared by all markup annotations.
type MarkupAnnotation interface {
	Annotation
	GetMarkup() *Markup
}

// fillDict adds the fields corresponding to the Markup struct
// to the given PDF dictionary.  If fields are not valid for the PDF version
// corresponding to the ResourceManager, an error is returned.
func (m *Markup) fillDict(rm *pdf.ResourceManager, d pdf.Dict) error {
	w := rm.Out

	if m.User != "" {
		if err := pdf.CheckVersion(w, "markup annotation T entry", pdf.V1_1); err != nil {
			return err
		}
		d["T"] = pdf.TextString(m.User)
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
		obj, err := rm.Embed(*m.RC)
		if err != nil {
			return err
		}
		d["RC"] = obj
	}

	if !m.CreationDate.IsZero() {
		if err := pdf.CheckVersion(w, "markup annotation CreationDate entry", pdf.V1_5); err != nil {
			return err
		}
		d["CreationDate"] = pdf.Date(m.CreationDate)
	}

	if m.InReplyTo != 0 {
		if err := pdf.CheckVersion(w, "markup annotation IRT entry", pdf.V1_5); err != nil {
			return err
		}
		d["IRT"] = m.InReplyTo
	}

	if m.Subject != "" {
		if err := pdf.CheckVersion(w, "markup annotation Subj entry", pdf.V1_5); err != nil {
			return err
		}
		d["Subj"] = pdf.TextString(m.Subject)
	}

	if m.RT != "" {
		if err := pdf.CheckVersion(w, "markup annotation RT entry", pdf.V1_6); err != nil {
			return err
		}
		if m.RT != "R" && m.RT != "Group" {
			return fmt.Errorf("invalid RT value %q", m.RT)
		}
		// Only write RT to PDF if IRT is present (RT is meaningless without IRT)
		if m.InReplyTo != 0 {
			d["RT"] = m.RT
		}
	}

	if m.Intent != "" && m.Intent != d["Subtype"] {
		if err := pdf.CheckVersion(w, "markup annotation IT entry", pdf.V1_6); err != nil {
			return err
		}
		d["IT"] = m.Intent
	}

	if m.ExData != nil {
		if err := pdf.CheckVersion(w, "markup annotation ExData entry", pdf.V1_7); err != nil {
			return err
		}
		d["ExData"] = m.ExData
	}

	return nil
}

func (m *Markup) isMarkup() {}

// isMarkup returns true if a is a markup annotation.
func isMarkup(a Annotation) bool {
	_, ok := a.(interface{ isMarkup() })
	return ok
}
