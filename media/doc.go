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

// Package media implements the PDF multimedia objects: renditions, media
// clips, media play and screen parameters, and the supporting players,
// software identifiers and offsets.
//
// These objects are file-independent: they contain no [pdf.Reference] values
// and can be moved between PDF files.  They are referenced from rendition
// actions and screen annotations, which are file-specific.
package media

// PDF 2.0 section: 13.2
