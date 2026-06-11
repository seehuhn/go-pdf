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
	"testing"

	"seehuhn.de/go/pdf"
)

// the field constructors set the partial name and the field type
func TestConstructors(t *testing.T) {
	tests := []struct {
		field Field
		want  pdf.Name
	}{
		{NewTextField("a"), "Tx"},
		{NewButtonField("b"), "Btn"},
		{NewChoiceField("c"), "Ch"},
		{NewSignatureField("d"), "Sig"},
	}
	for _, tc := range tests {
		if got := tc.field.FieldType(); got != tc.want {
			t.Errorf("FieldType = %q, want %q", got, tc.want)
		}
		if got := tc.field.PartialName(); got == "" {
			t.Errorf("%s field has no partial name", tc.want)
		}
	}
}
