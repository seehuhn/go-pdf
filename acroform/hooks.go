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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/formhooks"
)

// register the field plumbing used by the annotation and annotation/decode
// packages, so that it need not be part of the public API
func init() {
	formhooks.NewField = func(fieldType pdf.Name) any {
		return newField(fieldType)
	}
	formhooks.FieldEntries = func(rm *pdf.ResourceManager, field any) (pdf.Dict, error) {
		return fieldEntries(rm, field.(Field))
	}
}
