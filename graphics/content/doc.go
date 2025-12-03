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

// Package content implements PDF content streams and resource dictionaries.
//
// A content stream is a sequence of PDF operators that describe the graphical
// elements to be painted on a page. Content streams depend on an associated
// resource dictionary that provides named resources (fonts, images, etc.)
// referenced by operators. Together, a content stream and its resource
// dictionary form a self-contained entity.
//
// # Building Content Streams
//
// Use [State] with [Writer] for constructing new content streams. State tracks
// graphics parameters using Set/Known bits:
//
//   - Set: Parameter has a value (either known or inherited)
//   - Known: Parameter has a known value (subset of Set)
//   - UsedUnknown: Set-Unknown parameters that were used (for dependency tracking)
//
// The three-state model (Unset, Set-Unknown, Known) enables proper elision of
// redundant operators and tracking of inherited dependencies in Form XObjects.
//
// For a high-level API, use the [builder.Builder] type from the builder
// sub-package.
package content
