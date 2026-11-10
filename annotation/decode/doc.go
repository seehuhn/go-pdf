// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package decode reads PDF annotations and interactive form objects.
//
// The decoders for the annotation types ("seehuhn.de/go/pdf/annotation") and
// the form types ("seehuhn.de/go/pdf/acroform") are consolidated here because
// the two are mutually dependent: a widget annotation belongs to a form field,
// and a field merged with its single widget is one dictionary that is both.
// Keeping the readers in a separate package lets them depend on both type
// packages without an import cycle.
//
// There are two entry points:
//
//   - [Annotation] reads any annotation, dispatching on its subtype.  Invoke
//     it through [pdf.Cursor.Decode], so that indirect references are
//     resolved and reference cycles are detected.
//   - [DocumentForm] reads the document's interactive form, including its
//     field tree, once per extractor.
//
// The per-type decoders are unexported and reached through [Annotation]; this
// keeps a single entry point that handles the merged field/widget case
// correctly (such a dictionary yields a linked pair shared between the page's
// annotations and the field tree). Decoding is permissive, silently repairing
// malformed input rather than failing.
package decode
