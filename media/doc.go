// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package media implements the PDF multimedia objects: renditions, media
// clips, media play and screen parameters, and the supporting players,
// software identifiers and offsets.
//
// These objects are file-independent: they contain no [pdf.Reference] values
// and can be moved between PDF files.  They are referenced from rendition
// actions and screen annotations, which are file-specific.
package media

// PDF 2.0 section: 13.2
