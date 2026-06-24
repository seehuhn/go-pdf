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
	"seehuhn.de/go/pdf/action/triggers"
)

// Common holds the attributes shared by all terminal field types.
type Common struct {
	// Name (optional) is the partial field name.  If Name is empty, the field
	// does not contribute to fully qualified field names.
	//
	// This corresponds to the /T entry in the PDF field dictionary.
	Name string

	// AltName (optional) is an alternative field name used in the user
	// interface and for accessibility.
	//
	// This corresponds to the /TU entry in the PDF field dictionary.
	AltName string

	// ExportName (optional) is the mapping name used when exporting field data.
	//
	// This corresponds to the /TM entry in the PDF field dictionary.
	ExportName string

	// Flags holds the field flags.
	//
	// This corresponds to the /Ff entry in the PDF field dictionary.
	Flags FieldFlags

	// AA (optional) is the field's additional-actions dictionary.
	AA *triggers.Form

	// Widgets holds the field's widget annotations, one for each place the field
	// appears on a page.
	//
	// This corresponds to the /Kids entry in the PDF field dictionary.
	Widgets []Widget
}

// GetCommon implements the [Field] interface.
func (c *Common) GetCommon() *Common { return c }

// PartialName implements the [Node] interface.
func (c *Common) PartialName() string { return c.Name }
