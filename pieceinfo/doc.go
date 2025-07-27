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

// Package pieceinfo implements support for page-piece dictionaries.
//
// Page-piece dictionaries allow PDF processors to store private,
// application-specific data associated with documents, pages, or form
// XObjects. Each entry in a page-piece dictionary has a key which identifies
// the application the data is for, and the corresponding value has a
// LastModified date and contains private data, specific to the application.
//
// This package provides a generic framework for reading and writing page-piece
// dictionaries, with support for registering custom handlers for specific PDF
// processor types.
package pieceinfo
