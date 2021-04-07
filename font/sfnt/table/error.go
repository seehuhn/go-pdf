// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package table

// ErrNoTable indicates that a required table is missing from a TrueType or
// OpenType font file.
type ErrNoTable struct {
	Name string
}

func (err *ErrNoTable) Error() string {
	return "missing " + err.Name + " table in font"
}

// IsMissing returns true, if err indicates a missing sfnt table.
func IsMissing(err error) bool {
	_, missing := err.(*ErrNoTable)
	return missing
}
