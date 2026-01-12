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

// Package graphics provides the graphics state and supporting types for PDF
// content streams.
//
// # Content Streams and Graphics State
//
// PDF pages are described by content streams: sequences of operators that
// paint text, graphics, and images onto the page. These operators work within
// a graphics state - a collection of parameters that control appearance,
// including the current transformation matrix, colors, line styles, and text
// properties.
//
// The [State] type represents this complete graphics state as specified in
// PDF 32000-1:2008 section 8.4. The package also provides rendering constants
// ([LineCapStyle], [LineJoinStyle], [TextRenderingMode]) and interfaces for
// some external objects that can be painted in content streams ([XObject],
// [Shading], [Image]).
//
// # Related Packages
//
//   - [seehuhn.de/go/pdf/graphics/content/builder]: create content streams programmatically
//   - [seehuhn.de/go/pdf/document]: high-level page creation
//   - [seehuhn.de/go/pdf/reader]: read existing content streams
package graphics
