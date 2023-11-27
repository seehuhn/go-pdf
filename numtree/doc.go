// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

// Package numtree implements PDF number trees.
//
// Number trees map integers to PDF objects.  In PDF files these trees are used
// in two differnt contexts:
//   - The `PageLabels` entry in the document catalog is a number tree
//     defining page labels for the pages in the document.
//   - The `ParentTree` entry in the structure tree root dictionary
//     is a number tree used in finding the structure elements
//     to which content items belong.
package numtree
