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

// Package cmap implements CMap files for embedding in PDF files.
//
// When composite fonts are used in PDF files, glyphs are selected using a
// two-step process: first, character codes are mapped to character identifiers
// (CIDs), and then CIDs are mapped to glyph identifiers (GIDs). A CMap file
// describes the first step of this process, i.e. the mapping from character
// codes to CIDs.
package cmap
