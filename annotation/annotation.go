// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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

// AppearanceGenerator generates a normal appearance stream for an annotation
// that lacks one, storing it in the annotation.  It is implemented by
// [seehuhn.de/go/pdf/annotation/fallback.Generator]; callers that flatten or
// render annotations can accept this interface to inject a custom generator.
type AppearanceGenerator interface {
	AddAppearance(Annotation) error
}

// AppearanceBatcher is an [AppearanceGenerator] which can share work between
// the annotations it gives appearances to, such as the widgets of one form
// field.  Callers which generate appearances for many annotations, such as
// all annotations of a page, use a batch to keep the total work linear.
type AppearanceBatcher interface {
	AppearanceGenerator

	// NewBatch returns a generator for one batch of annotations.  The form
	// fields of the annotations, and the widgets in them, must not change
	// while the batch is in use, except through the batch itself.
	NewBatch() AppearanceGenerator
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
