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

// Package group implements PDF group XObject attributes.
//
// A group XObject is a form XObject with a Group entry in its dictionary.
// The Group entry contains a group attributes dictionary that specifies
// the properties of the group.
//
// Currently, the only defined group subtype is Transparency, represented
// by [TransparencyAttributes]. Transparency groups control how objects
// are composited together before being blended with the backdrop.
//
// Transparency groups can be associated with:
//   - Pages (via the Group entry in the page dictionary)
//   - Form XObjects (via the Group entry in the form dictionary)
package group
