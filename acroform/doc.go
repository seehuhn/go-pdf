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
// AcroForm entry in the document catalog. It holds a tree of nodes ([TreeNode]):
// terminal fields ([Field]) and interior [Group]s that merely group their
// descendants under a common name. Each terminal field has a concrete type: a
// [FieldTx] (text), [FieldBtn] (button), [FieldChoice] (choice), or [FieldSig]
// (signature).
//
// A field tree is plain top-down data. Build it from [Group] literals and the
// field constructors ([NewTextField] and friends), placing children directly in
// a [Group.Kids] slice or in [InteractiveForm.Fields]. Attach a field's on-page
// appearance with "seehuhn.de/go/pdf/annotation".AddWidget. Iterate the terminal
// fields, with their fully qualified names, using [InteractiveForm.AllFields].
//
// In a PDF file many field attributes are inheritable: a field without its own
// value takes its nearest ancestor's (12.7.4.1). This package hides that. A
// decoded field carries fully resolved values, and the encoder re-creates the
// inheritance where it shrinks the file, invisibly. There are therefore no
// inheritance helpers.
//
// Forms and fields are read with the "seehuhn.de/go/pdf/annotation/decode"
// package.
package acroform
