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

// Package annotation provides support for reading and writing PDF annotations.
//
// PDF annotations are additions to a PDF page which are stored outside the
// page content stream (in the Annots entry of the page dictionary), in a
// structured format.  This makes it relatively easy to add/edit/remove
// annotations without having to rewrite the entire page.
//
// PDF 2.0 defines 28 different annotation types. The most common type is
// [Link], which represents clickable areas in the PDF that can navigate to
// other pages or external URLs. Other commonly used annotation types include
// [Widget] for form fields and [Text] for simple text notes. All annotation
// types implement the [Annotation] interface. The full list is:
//   - [Annot3D]: (PDF 1.6) include 3D artwork in PDF documents
//   - [Caret]: (PDF 1.5) indicate the presence of text edits
//   - [Circle]: draw an ellipse onto the page
//   - [FileAttachment]: (PDF 1.3)
//   - [FreeText]: (PDF 1.3)
//   - [Highlight]: (PDF 1.3)
//   - [Ink]: (PDF 1.3)
//   - [Line]: (PDF 1.3)
//   - [Link]: a clickable area which navigates to another page or an external URL
//   - [Movie]: (PDF 1.2; deprecated in PDF 2.0)
//   - [Polygon]: (PDF 1.5)
//   - [Polyline]: (PDF 1.5)
//   - [Popup]: (PDF 1.3)
//   - [PrinterMark]: (PDF 1.4)
//   - [Projection]: (PDF 2.0)
//   - [Redact]: (PDF 1.7)
//   - [RichMedia]: (PDF 2.0)
//   - [Screen]: (PDF 1.5)
//   - [Sound]: (PDF 1.2; deprecated in PDF 2.0)
//   - [Square]: (PDF 1.3) draw a rectangle onto the page
//   - [Squiggly]: (PDF 1.4)
//   - [Stamp]: (PDF 1.3)
//   - [StrikeOut]: (PDF 1.3)
//   - [Text]: a clickable icon which opens a popup with text
//   - [TrapNet]: (PDF 1.3; deprecated in PDF 2.0)
//   - [Underline]: (PDF 1.3)
//   - [Watermark]: (PDF 1.6)
//   - [Widget]: (PDF 1.2)
//
// The list of annotations is extensible; PDF viewers may support additional
// annotation types, and possibly a plugin system for custom annotations. The
// following type is used by the go-pdf library to represent custom
// annotations:
//   - [Custom]: any annotation not shown in the list above
package annotation
