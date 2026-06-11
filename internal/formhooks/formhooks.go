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

// Package formhooks gives the annotation package access to form field plumbing
// of the acroform package, without making it part of the public acroform API.
//
// The function variables are set by the acroform package when it is
// initialised. This package must not import acroform (every consumer of a
// hook imports acroform anyway, for its types), so the hooks pass fields as
// untyped values.
package formhooks

import "seehuhn.de/go/pdf"

// WidgetInfo tells a widget annotation how to tie itself to its form field when
// the widget is written.
type WidgetInfo struct {
	// ParentRef is the value for the widget's /Parent entry, or 0 to omit it.
	// For a merged field/widget it is the field's enclosing group; for a widget
	// of a multi-widget field it is the field's own object.
	ParentRef pdf.Reference

	// Entries holds the field's own dictionary entries to fold into a merged
	// field/widget dictionary. It is nil when the widget is not merged with its
	// field.
	Entries pdf.Dict
}

// WidgetFieldInfo reports how a widget annotation should be tied to its form
// field. The field and widget are passed as untyped values (an acroform.Field
// and the *annotation.Widget). The acroform package sets this when it is
// initialised; it returns an error if the form has not been stored yet.
var WidgetFieldInfo func(rm *pdf.ResourceManager, field, widget any) (WidgetInfo, error)
