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

package annotation

import (
	"seehuhn.de/go/pdf"
)

// Annotation represents a PDF annotation.
//
// Use [seehuhn.de/go/pdf/annotation/decode.Annotation] to decode annotations
// from a PDF file.
type Annotation interface {
	pdf.Encoder

	// AnnotationType returns the type of the annotation as defined in the PDF
	// specification, e.g. "Text", "Link" or "Widget".
	AnnotationType() pdf.Name

	// GetCommon returns the common annotation fields.
	GetCommon() *Common
}

var (
	_ Annotation = (*Text)(nil)
	_ Annotation = (*Link)(nil)
	_ Annotation = (*FreeText)(nil)
	_ Annotation = (*Line)(nil)
	_ Annotation = (*Square)(nil)
	_ Annotation = (*Circle)(nil)
	_ Annotation = (*Polygon)(nil)
	_ Annotation = (*PolyLine)(nil)
	_ Annotation = (*TextMarkup)(nil) // Highlight, Underline, Squiggly, StrikeOut
	_ Annotation = (*Caret)(nil)
	_ Annotation = (*Stamp)(nil)
	_ Annotation = (*Ink)(nil)
	_ Annotation = (*Popup)(nil)
	_ Annotation = (*FileAttachment)(nil)
	_ Annotation = (*Sound)(nil)
	_ Annotation = (*Movie)(nil)
	_ Annotation = (*Screen)(nil)
	_ Annotation = (*Widget)(nil)
	_ Annotation = (*PrinterMark)(nil)
	_ Annotation = (*TrapNet)(nil)
	_ Annotation = (*Watermark)(nil)
	_ Annotation = (*Annot3D)(nil) // 3D
	_ Annotation = (*Redact)(nil)
	_ Annotation = (*Projection)(nil)
	_ Annotation = (*RichMedia)(nil)
)
