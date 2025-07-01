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

// Package nametree implements PDF name trees.
//
// Name trees serve a similar purpose to dictionaries, associating keys and
// values, but using string keys that are ordered lexicographically. The data
// structure can represent an arbitrarily large collection of key-value pairs
// with efficient lookup without requiring the entire structure to be read
// from the PDF file.
//
// This package provides two implementations:
//   - InMemory: loads the entire tree into memory for fast access
//   - FromFile: reads nodes on demand for memory-efficient access to large trees
//
// The Write function creates balanced name trees using a streaming B-tree
// algorithm that processes entries incrementally with O(log n) memory usage.
package nametree
