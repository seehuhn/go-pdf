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
// AcroForm entry in the document catalog.  At most one form can be associated
// with a PDF document.
//
// Internally, the interactive form is structured as a tree. Internal nodes are
// represented by the [*Group] type, while leaf nodes are represented by four
// concrete field types:
//   - [TextField] for text input,
//   - [ButtonField] for push buttons, check boxes, and radio buttons,
//   - [ChoiceField] for list boxes and combo boxes, and
//   - [SignatureField] for digital signatures.
//
// The [Field] interface is implemented by all four leaf types, while the
// [Node] interface represents both internal and leaf nodes in the tree.
//
// A field tree is plain top-down data. Build it from [Group] literals and the
// field constructors ([NewTextField] and friends), placing children directly
// in a [Group.Kids] slice or in [InteractiveForm.Fields]. Attach a field's
// on-page appearance with "seehuhn.de/go/pdf/annotation".AddWidget. Iterate
// the terminal fields, with their fully qualified names, using
// [InteractiveForm.AllFields].
//
// Forms and fields are read with the "seehuhn.de/go/pdf/annotation/decode"
// package.
package acroform
