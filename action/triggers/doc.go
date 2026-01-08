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

// Package triggers implements additional-actions dictionaries for PDF documents.
//
// Additional-actions dictionaries extend the set of events that can trigger
// action execution. There are four types of additional-actions dictionaries:
//
//   - [Annotation] for annotations (Table 197 in the PDF spec)
//   - [Page] for page objects (Table 198)
//   - [Form] for interactive form fields (Table 199)
//   - [Catalog] for document-level events (Table 200)
//
// Each type corresponds to the AA entry in its respective dictionary type.
package triggers
