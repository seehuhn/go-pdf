// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

// Package shading provides functionality for creating and manipulating PDF shading objects.
// Shading objects define smooth color transitions and gradients in PDF documents.
// From the shading, PDF viewers can compute the color of each output
// pixel at device resolution.
//
// This package supports multiple types of PDF shadings:
//   - Type1: Function-based shadings
//   - Type3: Radial shadings
//   - Type4: Free-form Gouraud-shaded triangle meshes
//
// Each shading type is represented by a corresponding struct (Type1, Type3, Type4)
// that implements the Shading interface. This interface provides an Embed method
// for incorporating the shading into a PDF document.
//
// Shadings can be used to create complex color effects, gradients, and smooth
// transitions in PDF graphics. They are typically used in conjunction with the
// graphics package to create shading patterns or to directly paint areas.
package shading
