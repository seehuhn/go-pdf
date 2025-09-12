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
// To add annotations to a PDF page, create an annotation object (e.g., [Link]
// or [Text]), configure its fields including the Common.Rect to position it on
// the page, then encode it using the Encode method. Write the encoded
// annotation as an indirect object to the PDF, and add its reference to the
// page dictionary's "Annots" array. When reading, annotations for a page can
// be found in the "Annots" entry of the page dictionary and can be decoded
// using the [Decode] function.
//
// There are several classes of annotations. Markup annotations correspond to
// comments and other markings a reviewer or editor might add to a manuscript.
// These annotations allow for tracking of replies and approvals. The most
// common type of markup annotation are [Text] annotations, which represent
// simple text notes which can be opened by clicking on an icon on the PDF
// page. The full list of markup annotations for PDF 2.0 is:
//   - [Caret]: indicate the presence of text edits
//   - [Circle]: an ellipse
//   - [FileAttachment]: embed a file as an icon that can be opened or saved
//   - [FreeText]: display text directly on the page without a popup
//   - [Ink]: freehand drawing or handwritten paths
//   - [Line]: straight line with optional start and end decorations
//   - [Polygon]: closed polygonal path
//   - [Polyline]: open polygonal path with optional start and end decorations
//   - [Projection]: (PDF 2.0) dimensional measurements and callouts from 3D models
//   - [Redact]: mark content for removal and specify replacement text
//   - [Sound]: (deprecated in PDF 2.0) a voice note
//   - [Square]: a rectangle
//   - [Stamp]: apply a rubber stamp appearance to the page
//   - [Text]: a clickable icon which opens a popup with text
//   - [TextMarkup]: highlight, underline, strikeout, or squiggly underline text
//
// Interactive annotations provide a way to interact with on-screen versions of
// the document.  The most common type of interactive annotation are [Link]
// annotations, which represent clickable areas in the PDF that can navigate to
// other pages or external URLs.  The full list of interactive annotations for
// PDF 2.0 is:
//   - [Annot3D]: includes 3D artwork in PDF documents
//   - [Link]: a clickable area which navigates to another page or an external URL
//   - [Movie]: (deprecated in PDF 2.0) embed video content
//   - [Popup]: the pop-up window for text associated with markup annotations
//   - [RichMedia]: (PDF 2.0) embed rich media content including Flash and video
//   - [Screen]: define a page region for playing media clips
//   - [Widget]: an interactive form field
//
// Finally, the PDF specification defines a few annotations to support
// document preparation and production workflows:
//   - [PrinterMark]: registration marks, color bars, cutting guides, ...
//   - [TrapNet]: (deprecated in PDF 2.0) compensate for potential misregistration during printing
//   - [Watermark]: watermark overlaid over the page content
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
