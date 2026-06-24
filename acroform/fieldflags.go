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

package acroform

import "seehuhn.de/go/pdf"

// FieldFlags is a set of flags describing characteristics of a form field.
//
// This type holds the flags common to all field types. Flags specific to a
// particular field type are defined alongside that type, using the same
// underlying representation so that they compose with the bitwise OR operator.
type FieldFlags uint32

// Flags common to all field types.
const (
	// FieldReadOnly indicates that the user may not change the value of the
	// field, and that associated widget annotations should not interact with
	// the user.
	FieldReadOnly FieldFlags = 1 << 0

	// FieldRequired indicates that the field must have a value when the form
	// is submitted by a submit-form action.
	FieldRequired FieldFlags = 1 << 1

	// FieldNoExport indicates that the field is not exported by a submit-form
	// action.
	FieldNoExport FieldFlags = 1 << 2
)

// Flags specific to [TextField].
const (
	// FieldMultiline indicates that the field may contain multiple lines of
	// text. If clear, the text is restricted to a single line.
	FieldMultiline FieldFlags = 1 << 12

	// FieldPassword indicates that the field is intended for secure password
	// entry; its value should not be echoed visibly or stored in cleartext.
	FieldPassword FieldFlags = 1 << 13

	// FieldFileSelect indicates that the field's text represents the pathname
	// of a file whose contents are submitted as the field's value.
	FieldFileSelect FieldFlags = 1 << 20

	// FieldDoNotScroll indicates that the field does not scroll to accommodate
	// more text than fits within its rectangle. Once the field is full, no
	// further text is accepted.
	FieldDoNotScroll FieldFlags = 1 << 23

	// FieldComb lays the text out into [TextField.MaxLen] equally spaced cells.
	// It may be set only if MaxLen is set and the Multiline, Password, and
	// FileSelect flags are all clear.
	FieldComb FieldFlags = 1 << 24
)

// Flags specific to [ButtonField].
const (
	// FieldNoToggleToOff (radio buttons only) indicates that exactly one button
	// is always selected; clicking the selected button does not deselect it.
	FieldNoToggleToOff FieldFlags = 1 << 14

	// FieldRadio indicates that the button field is a set of radio buttons. If
	// clear, the field is a check box. It is mutually exclusive with Pushbutton.
	FieldRadio FieldFlags = 1 << 15

	// FieldPushbutton indicates that the field is a push button that retains no
	// permanent value.
	FieldPushbutton FieldFlags = 1 << 16

	// FieldRadiosInUnison indicates that radio buttons in the field with the
	// same on-state value turn on and off together. If clear, the buttons are
	// mutually exclusive.
	FieldRadiosInUnison FieldFlags = 1 << 25
)

// Flags specific to [ChoiceField].
const (
	// FieldCombo indicates that the field is a combo box. If clear, the field
	// is a list box.
	FieldCombo FieldFlags = 1 << 17

	// FieldEdit indicates that the combo box includes an editable text box. It
	// applies only when Combo is set.
	FieldEdit FieldFlags = 1 << 18

	// FieldSort indicates that the field's option items are sorted
	// alphabetically. This flag is meaningful to PDF writers; readers display
	// the items in the order given by the option array.
	FieldSort FieldFlags = 1 << 19

	// FieldMultiSelect indicates that more than one option item may be selected
	// at once.
	FieldMultiSelect FieldFlags = 1 << 21

	// FieldCommitOnSelChange indicates that a new value is committed as soon as
	// a selection is made, rather than when the user leaves the field.
	FieldCommitOnSelChange FieldFlags = 1 << 26
)

// FieldDoNotSpellCheck indicates that text entered in the field is not
// spell-checked. It applies to text fields ([TextField]) and to editable combo
// boxes ([ChoiceField] with Combo and Edit set).
const FieldDoNotSpellCheck FieldFlags = 1 << 22

// FieldRichText indicates that the text field's value is a rich text string.
// It applies to text fields ([TextField]).
const FieldRichText FieldFlags = 1 << 25

// flagVersions lists the minimum PDF version of each field flag introduced
// after the form fields themselves; unlisted flags carry no extra version
// requirement. FieldRichText and FieldRadiosInUnison share a bit and a
// version, so one entry covers both.
var flagVersions = []struct {
	flag FieldFlags
	name string
	v    pdf.Version
}{
	{FieldFileSelect, "FileSelect", pdf.V1_4},
	{FieldMultiSelect, "MultiSelect", pdf.V1_4},
	{FieldDoNotSpellCheck, "DoNotSpellCheck", pdf.V1_4},
	{FieldDoNotScroll, "DoNotScroll", pdf.V1_4},
	{FieldComb, "Comb", pdf.V1_5},
	{FieldRichText, "RichText/RadiosInUnison", pdf.V1_5},
	{FieldCommitOnSelChange, "CommitOnSelChange", pdf.V1_5},
}

// checkFlagVersions verifies that every flag set in ff is allowed in the
// output version.
func checkFlagVersions(w *pdf.Writer, ff FieldFlags) error {
	for _, fv := range flagVersions {
		if ff&fv.flag != 0 {
			if err := pdf.CheckVersion(w, "field "+fv.name+" flag", fv.v); err != nil {
				return err
			}
		}
	}
	return nil
}
