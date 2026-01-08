// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

// Package softclip provides types for PDF soft-mask dictionaries.
//
// Soft masks control the shape or opacity of objects during compositing,
// enabling effects such as gradual transitions between an object and its
// backdrop (soft clipping). The mask values are derived from a transparency
// group XObject, using either the group's alpha channel or its luminosity.
//
// A [Mask] can be set as the current soft mask in the graphics state via the
// SMask entry in an extended graphics state dictionary (ExtGState).
//
// See PDF 2.0 specification sections 11.5 and 11.6.5.1.
package softclip
