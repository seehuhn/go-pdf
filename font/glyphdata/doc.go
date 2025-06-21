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

// Package glyphdata provides support for embedding and extracting font glyph data in PDF files.
//
// This package handles different font formats including Type 1, TrueType, CFF, and OpenType fonts.
// Each format has its own subpackage that implements format-specific embedding and extraction logic.
//
// The main Type enumeration defines the supported font types, and each subpackage provides
// Embed and Extract functions for handling font data streams in PDF files.
package glyphdata
