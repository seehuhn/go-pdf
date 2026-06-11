// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package acroform

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/formhooks"
)

// register the field plumbing used by the annotation package, so that it need
// not be part of the public API
func init() {
	formhooks.WidgetFieldInfo = func(rm *pdf.ResourceManager, field, widget any) (formhooks.WidgetInfo, error) {
		f, ok := field.(Field)
		if !ok {
			return formhooks.WidgetInfo{}, errors.New("widget parent is not a form field")
		}
		st := f.base().enc
		if st == nil || st.rm != rm {
			return formhooks.WidgetInfo{}, errors.New("interactive form must be stored before its widget annotations are written")
		}
		if st.entries != nil {
			// a merged field/widget: fold the field's entries in and point
			// /Parent at the field's own parent group
			return formhooks.WidgetInfo{ParentRef: st.parentRef, Entries: st.entries}, nil
		}
		// a widget of a multi-widget field: /Parent points at the field object
		return formhooks.WidgetInfo{ParentRef: st.fieldRef}, nil
	}
}
