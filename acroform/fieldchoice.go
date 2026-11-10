// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package acroform

import (
	"errors"
	"slices"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.7.5.4

// ChoiceOption is a single selectable item of a [ChoiceField].
type ChoiceOption struct {
	// Export is the value exported when the option is selected.
	Export string

	// Display is the text shown to the user.
	Display pdf.TextString
}

// ChoiceField is a choice form field.
//
// A ChoiceField is a list box or a combo box.  The [FieldCombo] flag selects
// a combo box, and [FieldEdit] makes it editable, so that the user can type
// text which names no option.
//
// Use [seehuhn.de/go/pdf/annotation.AddWidget] to add a visual representation
// of the field to a page.
type ChoiceField struct {
	Common
	VariableText

	// Options (optional) holds the field's selectable options, in display
	// order.  With [FieldSort], the options should be sorted already; the
	// library does not reorder them.
	//
	// This corresponds to the /Opt entry in the PDF field dictionary.
	Options []ChoiceOption

	// TopIndex (optional) is the index in Options of the first option visible
	// in a scrollable list box.  It must be 0 or a valid index into Options.
	// The default, 0, shows the list from its first option.
	//
	// This corresponds to the /TI entry in the PDF field dictionary.
	TopIndex int

	// Selected (optional) holds the indices into Options of the selected
	// options, in ascending order.  It is needed only where Value alone does
	// not identify the options, that is where several options share a
	// display string.
	//
	// If both Value and Selected are set, they must name the same options.
	//
	// This corresponds to the /I entry in the PDF field dictionary.
	Selected []int

	// Value (optional) holds the display strings of the selected options.  A
	// field without [FieldMultiSelect] has at most one entry.  Each entry must
	// be the display string of an option, except in an editable combo box,
	// which may hold any text.
	//
	// On write, a nil Value is treated as the display strings of the options
	// listed in Selected.
	//
	// This corresponds to the /V entry in the PDF field dictionary.
	Value []pdf.TextString

	// DefaultValue (optional) is the field's default value, used when the
	// form is reset.  Like Value, it has at most one entry without
	// [FieldMultiSelect], and its entries must name options except in an
	// editable combo box.
	//
	// This corresponds to the /DV entry in the PDF field dictionary.
	DefaultValue []pdf.TextString
}

var _ Field = (*ChoiceField)(nil)

// NewChoiceField returns a new choice field (field type "Ch") with the given
// partial name.
func NewChoiceField(name string) *ChoiceField {
	f := &ChoiceField{}
	f.Name = name
	return f
}

// FieldType implements the [Field] interface.
func (f *ChoiceField) FieldType() pdf.Name { return "Ch" }

// ExportData returns the export values of the selected options, one per
// option.  If no option is selected, the result is empty.
//
// This implements the [Field] interface.
func (f *ChoiceField) ExportData() []string {
	if len(f.Selected) > 0 && (f.Value == nil || f.SelectionAgrees()) {
		res := make([]string, 0, len(f.Selected))
		for _, idx := range f.Selected {
			if idx >= 0 && idx < len(f.Options) {
				res = append(res, f.Options[idx].Export)
			}
		}
		return res
	}
	res := make([]string, 0, len(f.Value))
	for _, v := range f.Value {
		res = append(res, f.exportOf(v))
	}
	return res
}

// exportOf returns the export value of the first option with display string
// v, or the text of v if no option matches.
func (f *ChoiceField) exportOf(v pdf.TextString) string {
	for _, o := range f.Options {
		if o.Display == v {
			return o.Export
		}
	}
	return v.String()
}

func (f *ChoiceField) fillDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if f.DefaultAppearance == "" {
		return errors.New("choice field without a default appearance")
	}
	if err := f.VariableText.fillVarTextDict(rm, dict); err != nil {
		return err
	}
	if len(f.Options) > 0 {
		arr := make(pdf.Array, len(f.Options))
		for i, o := range f.Options {
			// the single-string form makes the display string the export
			// value, so it is used only where the two agree exactly
			if export := pdf.Text(o.Export); export == o.Display {
				arr[i] = o.Display
			} else {
				arr[i] = pdf.Array{export, o.Display}
			}
		}
		dict["Opt"] = arr
	}
	if f.TopIndex < 0 || (f.TopIndex > 0 && f.TopIndex >= len(f.Options)) {
		return errors.New("choice field TopIndex out of range")
	}
	if f.TopIndex > 0 {
		dict["TI"] = pdf.Integer(f.TopIndex)
	}
	if err := f.fillValue(rm, dict); err != nil {
		return err
	}
	multi := f.Flags&FieldMultiSelect != 0
	if !multi && len(f.DefaultValue) > 1 {
		return errors.New("multiple selections require the MultiSelect flag")
	}
	if err := f.checkOptions(f.DefaultValue); err != nil {
		return err
	}
	if dv := choiceValueObject(f.DefaultValue, multi); dv != nil {
		dict["DV"] = dv
	}
	return nil
}

// fillValue adds the /V, /I and /RV entries to dict.
func (f *ChoiceField) fillValue(rm *pdf.ResourceManager, dict pdf.Dict) error {
	for i, idx := range f.Selected {
		if idx < 0 || idx >= len(f.Options) {
			return errors.New("choice field selection index out of range")
		}
		if i > 0 && idx <= f.Selected[i-1] {
			return errors.New("choice field selection indices must be ascending")
		}
	}
	if f.Flags&FieldMultiSelect == 0 && (len(f.Value) > 1 || len(f.Selected) > 1) {
		return errors.New("multiple selections require the MultiSelect flag")
	}
	v := f.Value
	if v == nil {
		for _, idx := range f.Selected {
			v = append(v, f.Options[idx].Display)
		}
	} else if len(f.Selected) > 0 && !f.SelectionAgrees() {
		return errors.New("choice field Value and Selected disagree")
	}
	if err := f.checkOptions(v); err != nil {
		return err
	}
	if len(f.Selected) > 0 {
		if err := pdf.CheckVersion(rm.Out, "choice field I entry", pdf.V1_4); err != nil {
			return err
		}
		arr := make(pdf.Array, len(f.Selected))
		for i, idx := range f.Selected {
			arr[i] = pdf.Integer(idx)
		}
		dict["I"] = arr
	}
	if v := choiceValueObject(v, f.Flags&FieldMultiSelect != 0); v != nil {
		dict["V"] = v
	}
	return f.fillRichText(rm, dict)
}

// checkOptions returns an error if an entry of v is not the display string of
// an option, unless the field is an editable combo box.
func (f *ChoiceField) checkOptions(v []pdf.TextString) error {
	if f.IsEditable() {
		return nil
	}
	for _, s := range v {
		if !f.hasOption(s) {
			return errors.New("choice field value names no option")
		}
	}
	return nil
}

// hasOption reports whether v is the display string of an option.
func (f *ChoiceField) hasOption(v pdf.TextString) bool {
	return slices.ContainsFunc(f.Options, func(o ChoiceOption) bool { return o.Display == v })
}

// IsEditable reports whether the field is an editable combo box: one with
// both the [FieldCombo] and [FieldEdit] flags set, whose value may be text
// which names no option.
func (f *ChoiceField) IsEditable() bool {
	const both = FieldCombo | FieldEdit
	return f.Flags&both == both
}

// SelectionAgrees reports whether Value and Selected name the same options:
// the display strings of the options at Selected, taken as a multiset, must
// equal Value.  The order of Value is not significant, since Selected is kept
// ascending.
func (f *ChoiceField) SelectionAgrees() bool {
	if len(f.Value) != len(f.Selected) {
		return false
	}
	count := make(map[pdf.TextString]int, len(f.Value))
	for _, s := range f.Value {
		count[s]++
	}
	for _, idx := range f.Selected {
		s := f.Options[idx].Display
		if count[s] == 0 {
			return false
		}
		count[s]--
	}
	return true
}

// choiceValueObject converts a choice field's selection to its PDF
// representation: nil when nothing is selected, a bare text string for a
// single selection, and an array of text strings for multiple selections.
// Only a multi-select field can hold an explicit empty selection, which is
// written as an empty array; for other fields an empty selection is nil.
func choiceValueObject(vals []pdf.TextString, multi bool) pdf.Object {
	switch len(vals) {
	case 0:
		if vals != nil && multi {
			return pdf.Array{}
		}
		return nil
	case 1:
		return vals[0]
	default:
		arr := make(pdf.Array, len(vals))
		for i, s := range vals {
			arr[i] = s
		}
		return arr
	}
}
