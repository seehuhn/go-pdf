// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package acroform

import (
	"errors"
	"strings"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.7.5.3

// TextField is a text input form field.
//
// Use [seehuhn.de/go/pdf/annotation.AddWidget] to add a visual representation
// of the field to a page.
type TextField struct {
	Common

	VariableText

	// Text (optional) is the field's text value.  For a field with the
	// [FieldPassword] flag set, Text and RichText are held in memory only:
	// they are never written to the file, and are nil when the field is
	// read.
	//
	// Without the [FieldMultiline] flag, Text and DefaultText must not
	// contain line breaks.
	//
	// This corresponds to the /V entry in the PDF field dictionary.
	Text *pdf.StringOrStream

	// DefaultText (optional) is the field's default text value, used when the
	// form is reset.  It must be nil for a field with the [FieldPassword] flag
	// set.
	//
	// This corresponds to the /DV entry in the PDF field dictionary.
	DefaultText *pdf.StringOrStream

	// MaxLen is the maximum length of the field's text in characters. A value
	// of zero indicates that no maximum is set. A field with the [FieldComb]
	// flag set must have a MaxLen.
	MaxLen int
}

var _ Field = (*TextField)(nil)

// NewTextField returns a new text field (field type "Tx") with the given
// partial name.
func NewTextField(name string) *TextField {
	f := &TextField{}
	f.Name = name
	return f
}

// FieldType implements the [Field] interface.
func (f *TextField) FieldType() pdf.Name { return "Tx" }

// ExportData returns the field's text inside a slice of length one.
// If the field has no value, nil is returned.
//
// This implements the [Field] interface.
func (f *TextField) ExportData() []string {
	if f.Text == nil {
		return nil
	}
	return []string{f.Text.Value.String()}
}

func (f *TextField) fillDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if f.DefaultAppearance == "" {
		return errors.New("text field without a default appearance")
	}
	if err := f.VariableText.fillVarTextDict(rm, dict); err != nil {
		return err
	}
	if err := f.fillValue(rm, dict); err != nil {
		return err
	}
	if f.DefaultText != nil {
		if f.Flags&FieldPassword != 0 {
			return errors.New("password field with a default value")
		}
		if err := f.checkLines(f.DefaultText); err != nil {
			return err
		}
		obj, err := rm.Embed(*f.DefaultText)
		if err != nil {
			return err
		}
		dict["DV"] = obj
	}
	// the Comb flag requires a MaxLen and may not be combined with the
	// Multiline, Password or FileSelect flags
	if f.Flags&FieldComb != 0 {
		if f.Flags&(FieldMultiline|FieldPassword|FieldFileSelect) != 0 {
			return errors.New("comb flag conflicts with Multiline, Password or FileSelect")
		}
		if f.MaxLen <= 0 {
			return errors.New("text field with Comb flag requires MaxLen")
		}
	}
	if f.MaxLen > 0 {
		dict["MaxLen"] = pdf.Integer(f.MaxLen)
	}
	return nil
}

// fillValue adds the /V and /RV entries to dict.  The value of a password
// field is never stored in the file, so for such a field it adds nothing.
func (f *TextField) fillValue(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if f.Flags&FieldPassword != 0 {
		return nil
	}
	if f.Text != nil {
		if err := f.checkLines(f.Text); err != nil {
			return err
		}
		obj, err := rm.Embed(*f.Text)
		if err != nil {
			return err
		}
		dict["V"] = obj
	}
	return f.fillRichText(rm, dict)
}

// checkLines returns an error if v contains a line break and the field does
// not have the [FieldMultiline] flag.
func (f *TextField) checkLines(v *pdf.StringOrStream) error {
	if f.Flags&FieldMultiline == 0 && strings.ContainsAny(v.Value.String(), "\r\n") {
		return errors.New("line break in single-line field")
	}
	return nil
}

// storedText returns the text value as it is written to the file: nil for a
// password field.
func (f *TextField) storedText() *pdf.StringOrStream {
	if f.Flags&FieldPassword != 0 {
		return nil
	}
	return f.Text
}
