// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// Package decode reads PDF annotations and interactive form objects.
//
// The decoders for the annotation types ("seehuhn.de/go/pdf/annotation") and
// the form types ("seehuhn.de/go/pdf/acroform") are consolidated here because
// the two are mutually dependent: a widget annotation belongs to a form field,
// and a field merged with its single widget is one dictionary that is both.
// Keeping the readers in a separate package lets them depend on both type
// packages without an import cycle.
//
// There are two entry points, each to be invoked through [pdf.Decode]
// so that indirect references are resolved and reference cycles are detected:
//
//   - [Annotation] reads any annotation, dispatching on its subtype.
//   - [Form] reads the document's interactive form dictionary, including its
//     field tree.
//
// The per-type decoders are unexported and reached through [Annotation]; this
// keeps a single entry point that handles the merged field/widget case
// correctly (such a dictionary yields a linked pair shared between the page's
// annotations and the field tree). Decoding is permissive, silently repairing
// malformed input rather than failing.
package decode
