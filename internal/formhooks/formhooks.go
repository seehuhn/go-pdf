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

// Package formhooks gives the annotation and annotation/decode packages
// access to form field plumbing of the acroform package, without making it
// part of the public acroform API.
//
// The function variables are set by the acroform package when it is
// initialised. This package must not import acroform (every consumer of a
// hook imports acroform anyway, for its types), so the hooks pass fields as
// untyped values.
package formhooks

import "seehuhn.de/go/pdf"

// NewField returns an empty concrete field (an acroform.Field) for the given
// field type, one of "Tx", "Ch", "Btn", or "Sig". Any other type yields a
// typeless field, a bare *acroform.FieldCommon.
var NewField func(fieldType pdf.Name) any

// FieldEntries builds the field-level dictionary entries (FT, T, the flags,
// AA, and the type-specific entries) of an acroform.Field, excluding /Parent
// and /Kids. The annotation package uses it to fold a terminal field's
// entries into the field's single widget annotation.
var FieldEntries func(rm *pdf.ResourceManager, field any) (pdf.Dict, error)
