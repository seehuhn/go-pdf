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

// FieldFlags is a set of flags describing characteristics of a form field.
//
// This type holds the flags common to all field types. Flags specific to a
// particular field type are defined alongside that type, using the same
// underlying representation so that they compose with the bitwise OR operator.
type FieldFlags uint32

const (
	// FieldReadOnly indicates that the user may not change the value of the
	// field, and that associated widget annotations should not interact with
	// the user.
	FieldReadOnly FieldFlags = 1 << 0

	// FieldRequired indicates that the field must have a value when the form
	// is submitted by a submit-form action.
	FieldRequired FieldFlags = 1 << 1

	// FieldNoExport indicates that the field is not exported by a submit-form
	// action.
	FieldNoExport FieldFlags = 1 << 2
)
