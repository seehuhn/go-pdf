// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package acroform implements PDF interactive forms (AcroForm).
// A PDF document can contain at most one interactive form, referenced from the
// AcroForm entry in the document catalog. Interactive forms are represented by
// the [InteractiveForm] type.
//
// Internally, the interactive form is structured as a tree. Internal nodes are
// represented by the [*Group] type, while leaf nodes are represented by five
// concrete field types:
//   - [TextField] for text input,
//   - [ButtonField] for check boxes and radio buttons,
//   - [PushButtonField] for push buttons,
//   - [ChoiceField] for list boxes and combo boxes, and
//   - [SignatureField] for digital signatures.
//
// The [Field] interface is implemented by all five leaf types, while the
// [Node] interface represents both internal and leaf nodes in the tree.
//
// A field tree is plain top-down data. Build it from [Group] literals and the
// field constructors ([NewTextField] and friends), placing children directly
// in a [Group.Children] slice or in [InteractiveForm.Fields]. Attach a field's
// on-page appearance with "seehuhn.de/go/pdf/annotation".AddWidget. Iterate
// the terminal fields, with their fully qualified names, using
// [InteractiveForm.AllFields].
//
// To fill in a form, find a field with [InteractiveForm.LogicalField] or
// [InteractiveForm.LogicalFields] and change its value with
// [LogicalField.Toggle] and [LogicalField.SetState] for buttons,
// [LogicalField.SetText] for text fields, and [LogicalField.Select] and
// [LogicalField.SetEditText] for choice fields.  These keep the field value,
// the selection and the widgets' appearance states consistent, also across
// all fields which share the name, and return an error for values the field
// cannot hold.
// They ignore [FieldReadOnly]: a viewer which lets the user fill in a form
// must refuse changes to read-only fields itself.  [InteractiveForm.ExportData]
// lists the name-value pairs which the form's fields contribute when the form
// is submitted.
//
// Interactive forms are read using
// [seehuhn.de/go/pdf/annotation/decode.DocumentForm] and written using
// [InteractiveForm.Encode].
package acroform
