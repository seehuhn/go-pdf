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
// There are two classes of annotations, markup annotations and interactive
// annotations. Markup annotations correspond to comments and other markings a
// reviewer or editor might add to a manuscript. These annotations allow for
// tracking of relies and approvals. The most common type of markup annotation
// are [Text] annotations, which represent simple text notes which can be
// opened by clicking on an icon on the PDF page. The full list of markup
// annotations for PDF 2.0 is:
//   - [Caret]: indicate the presence of text edits
//   - [Circle]: draw an ellipse onto the page
//   - [FileAttachment]:
//   - [FreeText]:
//   - [Ink]:
//   - [Line]:
//   - [Polygon]:
//   - [Polyline]:
//   - [Projection]: (PDF 2.0)
//   - [Redact]:
//   - [Sound]: (deprecated in PDF 2.0)
//   - [Square]: draw a rectangle onto the page
//   - [Stamp]:
//   - [Text]: a clickable icon which opens a popup with text
//   - [TextMarkup]:
//
// Interactive annotations provide a way to interact with on-screen versions of
// the document.  The most common type of interactive annotation are [Link]
// annotations, which represent clickable areas in the PDF that can navigate to
// other pages or external URLs.  The full list of interactive annotations for
// PDF 2.0 is:
//   - [Annot3D]: includes 3D artwork in PDF documents
//   - [Link]: a clickable area which navigates to another page or an external URL
//   - [Movie]: (deprecated in PDF 2.0)
//   - [Popup]:
//   - [PrinterMark]:
//   - [RichMedia]: (PDF 2.0)
//   - [Screen]:
//   - [TrapNet]: (deprecated in PDF 2.0)
//   - [Watermark]:
//   - [Widget]:
//
// The list of annotations is extensible; PDF viewers may support additional
// annotation types, and possibly a plugin system for custom annotations. The
// following type is used by the go-pdf library to represent custom
// annotations:
//   - [Custom]: any annotation not shown in the list above
//
// PDF annotations are stored outside the page content stream (in the Annots
// entry of the page dictionary), in a structured format.  This makes it
// relatively easy to add/edit/remove annotations without having to rewrite the
// entire page.
package annotation
