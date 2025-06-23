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

// Package shading provides functionality for creating PDF shading objects.
// Shading objects define smooth color transitions and gradients in PDF documents.
// For a given shading, PDF viewers can compute the color of each output
// pixel at device resolution.
//
// This package supports multiple types of PDF shadings:
//   - Type1: Function-based shadings
//   - Type2: Axial shadings
//   - Type3: Radial shadings
//   - Type4: Free-form Gouraud-shaded triangle meshes
package shading
