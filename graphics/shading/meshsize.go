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

package shading

import "unsafe"

// In-memory sizes of the per-element mesh structures, excluding the color
// backing arrays.  They give the per-element cost charged against the
// membudget.Budget in [parseType4Vertices] and friends.  unsafe is confined to
// these declarations so the use is easy to audit.
const (
	type4VertexSize = int64(unsafe.Sizeof(Type4Vertex{}))
	type5VertexSize = int64(unsafe.Sizeof(Type5Vertex{}))
	type6PatchSize  = int64(unsafe.Sizeof(Type6Patch{}))
	type7PatchSize  = int64(unsafe.Sizeof(Type7Patch{}))
)
