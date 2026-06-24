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

package decode

import (
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

// decodeMarkup extracts fields common to all markup annotations from a PDF dictionary.
func decodeMarkup(c pdf.Cursor, dict pdf.Dict, markup *annotation.Markup) error {
	// T (optional)
	if t, err := pdf.Optional(c.TextString(dict["T"])); err != nil {
		return err
	} else {
		markup.User = string(t)
	}

	// Popup (optional)
	if popup, ok := dict["Popup"].(pdf.Reference); ok {
		markup.Popup = popup
	}

	// RC (optional)
	if rc, err := stringOrStreamPtr(c, dict["RC"]); err != nil {
		return err
	} else {
		markup.RC = rc
	}

	// CreationDate (optional)
	if creationDate, err := pdf.Optional(c.Date(dict["CreationDate"])); err != nil {
		return err
	} else if !creationDate.IsZero() {
		markup.CreationDate = time.Time(creationDate)
	}

	// IRT (optional)
	if irt, ok := dict["IRT"].(pdf.Reference); ok {
		markup.InReplyTo = irt
	}

	// Subj (optional)
	if subj, err := pdf.Optional(c.TextString(dict["Subj"])); err != nil {
		return err
	} else {
		markup.Subject = string(subj)
	}

	// RT (optional); only "R" and "Group" are valid, and RT is meaningless
	// without IRT — snap anything else to the default so the annotation can be
	// written back
	if rt, err := pdf.Optional(c.Name(dict["RT"])); err != nil {
		return err
	} else if (rt == "R" || rt == "Group") && markup.InReplyTo != 0 {
		markup.RT = rt
	}

	// IT (optional; per spec, IT equal to Subtype means no explicit intent)
	if it, err := pdf.Optional(c.Name(dict["IT"])); err != nil {
		return err
	} else if it != dict["Subtype"] {
		markup.Intent = it
	}

	// ExData (optional)
	markup.ExData = dict["ExData"]

	return nil
}
