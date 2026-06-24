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

// Package formhooks lets the acroform and annotation packages call into each
// other without an import cycle.
//
// The function variables are set by the annotation package when it is
// initialised and called by the acroform package. This package must not import
// either of them, so the hooks pass values as untyped any.
package formhooks

import "seehuhn.de/go/pdf"

// EncodeWidgetEntries returns the widget annotation's own dictionary entries
// (everything except the form-field linkage: /Parent and the folded-in field
// entries). The acroform package calls it while encoding a form to assemble the
// merged or separate field/widget dictionary; the annotation package sets it
// when it is initialised. The widget is passed as an untyped value (the
// *annotation.Widget).
var EncodeWidgetEntries func(rm *pdf.ResourceManager, widget any) (pdf.Dict, error)
