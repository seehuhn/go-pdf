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

// Package annotation provides types and functions for reading and writing PDF
// annotations following the PDF specification.
//
// This package supports various annotation types including text annotations,
// links, markup annotations (highlighting, underlining, strikeouts, etc.),
// geometric shapes (squares, circles, polygons), and form widgets.
//
// The most common annotation type is [Link], which represents clickable
// areas in the PDF that can navigate to other pages or external URLs.
// Other commonly used annotation types include [Widget] for form fields,
// [Popup] for pop-up notes, and [Text] for simple text notes.
//
// All annotation types implement the [pdf.Annotation] interface. The function
// [Extract] can be used to extract annotations from a PDF document.
package annotation
