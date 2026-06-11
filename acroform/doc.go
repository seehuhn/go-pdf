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

// Package acroform implements PDF interactive forms (AcroForm), described in
// the PDF specification chapter 12.7.
//
// A document's interactive form is an [InteractiveForm], referenced from the
// AcroForm entry in the document catalog. It holds a tree of form fields. Each
// field is a [Field]: a [FieldTx] (text), [FieldBtn] (button), [FieldChoice]
// (choice), or [FieldSig] (signature) for a terminal field with its own type,
// or a [FieldCommon] for a non-terminal field or one whose type is inherited.
//
// Build a field tree with the builder methods on [InteractiveForm] for root
// fields and the [NewField] family of functions for sub-fields; both wire the
// parent links automatically. A terminal field's
// on-page appearance is a widget annotation, "seehuhn.de/go/pdf/annotation".Widget;
// add one with that package's AddWidget function. After assembling a field's
// children by hand (rather than with the builders), set each child's parent
// link ([FieldCommon.Parent], or the Parent field of a widget child) so that
// the inheritance helpers (see [ResolvedFT]) and
// [FieldCommon.FullyQualifiedName] resolve correctly.
//
// Forms and fields are read with the "seehuhn.de/go/pdf/annotation/decode"
// package.
package acroform
