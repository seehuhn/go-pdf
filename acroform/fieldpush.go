// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package acroform

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.7.5.2.2

// PushButtonField is a push button form field: a button which responds to
// clicks through its actions and retains no value.
//
// The field must have the [FieldPushbutton] flag set; [NewPushButtonField]
// sets it.
//
// Use [seehuhn.de/go/pdf/annotation.AddWidget] to add a visual representation
// for each of the field's buttons.
type PushButtonField struct {
	Common

	VariableText
}

var _ Field = (*PushButtonField)(nil)

// NewPushButtonField returns a new push button field (field type "Btn") with
// the given partial name.
func NewPushButtonField(name string) *PushButtonField {
	f := &PushButtonField{}
	f.Name = name
	f.Flags = FieldPushbutton
	return f
}

// FieldType implements the [Field] interface.
func (f *PushButtonField) FieldType() pdf.Name { return "Btn" }

// ExportData returns nil, since a push button has no value.
//
// This implements the [Field] interface.
func (f *PushButtonField) ExportData() []string { return nil }

func (f *PushButtonField) fillDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if f.Flags&FieldPushbutton == 0 {
		return errors.New("push button field without the Pushbutton flag")
	}
	if err := f.VariableText.fillVarTextDict(rm, dict); err != nil {
		return err
	}
	return f.fillValue(rm, dict)
}

// fillValue adds the /RV entry to dict.
func (f *PushButtonField) fillValue(rm *pdf.ResourceManager, dict pdf.Dict) error {
	return f.fillRichText(rm, dict)
}
