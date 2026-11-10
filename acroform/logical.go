// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package acroform

import (
	"errors"
	"iter"
	"slices"

	"seehuhn.de/go/pdf"
)

// LogicalField is the set of terminal fields which share one fully qualified
// name.  PDF treats these fields as a single field with a single value.  The
// fields must all have the same type and hold the same value.
type LogicalField struct {
	// Name is the fully qualified field name.
	Name string

	// Fields are the terminal fields with this name, in tree order.
	// The logical fields of a form have at least one field each.
	Fields []Field
}

// NewLogicalField returns the logical field with the fully qualified name
// name which consists of the given terminal fields.  The setters change only
// the listed fields and keep them consistent with each other.  To fill in a
// form, list every terminal field of the form with this name, or use
// [InteractiveForm.LogicalField].
func NewLogicalField(name string, fields ...Field) *LogicalField {
	return &LogicalField{Name: name, Fields: fields}
}

// LogicalFields returns an iterator over the form's logical fields.  The
// fields are visited in the order in which their names first occur in a
// depth-first walk of the field tree.
func (f *InteractiveForm) LogicalFields() iter.Seq[*LogicalField] {
	return func(yield func(*LogicalField) bool) {
		var order []*LogicalField
		byName := make(map[string]*LogicalField)
		for name, field := range f.AllFields() {
			lf, seen := byName[name]
			if !seen {
				lf = &LogicalField{Name: name}
				byName[name] = lf
				order = append(order, lf)
			}
			lf.Fields = append(lf.Fields, field)
		}

		for _, lf := range order {
			if !yield(lf) {
				return
			}
		}
	}
}

// LogicalField returns the logical field with the given fully qualified name,
// or nil if the form has no terminal field with this name.
func (f *InteractiveForm) LogicalField(name string) *LogicalField {
	var fields []Field
	for n, field := range f.AllFields() {
		if n == name {
			fields = append(fields, field)
		}
	}
	if fields == nil {
		return nil
	}
	return &LogicalField{Name: name, Fields: fields}
}

// typedFields returns the fields of lf as values of type T.  It returns an
// error if lf has no fields or if one of them has a different type.
func typedFields[T Field](lf *LogicalField) ([]T, error) {
	if len(lf.Fields) == 0 {
		return nil, errors.New("logical field has no fields")
	}
	res := make([]T, len(lf.Fields))
	for i, f := range lf.Fields {
		t, ok := f.(T)
		if !ok {
			return nil, errors.New("wrong field type")
		}
		res[i] = t
	}
	return res, nil
}

// SetText sets the text value of every field in lf.  The stored values keep
// the string or stream form of the previous values, and any rich text value
// is cleared.  An empty s gives an explicit empty value, not an absent one.
//
// SetText returns an error and changes nothing if the fields are not text
// fields, if s has more than MaxLen characters for one of the fields, or if s
// contains a line break and one of the fields does not have the
// [FieldMultiline] flag.
func (lf *LogicalField) SetText(s string) error {
	fields, err := typedFields[*TextField](lf)
	if err != nil {
		return err
	}
	for _, f := range fields {
		if err := f.checkText(s); err != nil {
			return err
		}
	}
	for _, f := range fields {
		f.applyText(s)
	}
	return nil
}

// Select selects the options with the given indices into src.Options, sets
// the value of every field in lf to their display strings, and clears any
// rich text value.  The field src must be one of the fields of lf: the field
// whose option list the user chose from.  The order of the indices and
// duplicates are ignored.  The other fields select options with the same
// display strings, where a string which occurs several times selects as many
// options.  Every field takes the value of src, with the strings in the same
// order.
//
// An empty list of indices clears the selection.  With [FieldMultiSelect],
// Value is then an explicit empty value; without it, Value is absent.
//
// Selected is set only where Value alone does not identify the options: if
// several options are selected, or if a selected option shares its display
// string with another.  Otherwise Selected is nil, so that the field can be
// written in PDF versions before 1.4, which have no /I entry.
//
// Select returns an error and changes nothing if the fields are not choice
// fields, if src is not one of them, if an index is out of range, if several
// options are selected in a field without the [FieldMultiSelect] flag, or if
// one of the other fields lacks an option with a required display string.
func (lf *LogicalField) Select(src *ChoiceField, indices []int) error {
	fields, err := typedFields[*ChoiceField](lf)
	if err != nil {
		return err
	}
	k := slices.Index(fields, src)
	if k < 0 {
		return errors.New("field is not part of the logical field")
	}
	sel := slices.Clone(indices)
	slices.Sort(sel)
	sel = slices.Compact(sel)
	if err := src.checkSelection(sel); err != nil {
		return err
	}
	v := make([]pdf.TextString, len(sel))
	for i, idx := range sel {
		v[i] = src.Options[idx].Display
	}

	sels := make([][]int, len(fields))
	for j, f := range fields {
		if j == k {
			sels[j] = sel
			continue
		}
		s, err := f.matchDisplay(v)
		if err != nil {
			return err
		}
		if err := f.checkSelection(s); err != nil {
			return err
		}
		sels[j] = s
	}

	src.applySelection(sel)
	for j, f := range fields {
		if j == k {
			continue
		}
		f.applySelection(sels[j])
		if len(sel) > 0 {
			// same value as src, whatever the option order; an empty
			// selection follows the field's own MultiSelect flag
			f.Value = slices.Clone(src.Value)
		}
	}
	return nil
}

// matchDisplay returns the ascending indices of options whose display strings
// make up v.  A string which occurs several times in v selects as many
// options with that display string.
func (f *ChoiceField) matchDisplay(v []pdf.TextString) ([]int, error) {
	used := make([]bool, len(f.Options))
	sel := make([]int, 0, len(v))
	for _, text := range v {
		idx := -1
		for i, o := range f.Options {
			if !used[i] && o.Display == text {
				idx = i
				break
			}
		}
		if idx < 0 {
			return nil, errors.New("option missing in field")
		}
		used[idx] = true
		sel = append(sel, idx)
	}
	slices.Sort(sel)
	return sel, nil
}

// SetEditText sets the text of every field in lf, which must be editable combo
// boxes, and clears any rich text value.  If s is the display string of an
// option of a field, the first such option becomes the selection of that
// field; otherwise nothing is selected and Value holds s alone.  Selected is
// set as for [LogicalField.Select].
//
// SetEditText returns an error and changes nothing if the fields are not
// choice fields, or if one of them does not have both the [FieldCombo] and
// [FieldEdit] flags.
func (lf *LogicalField) SetEditText(s string) error {
	fields, err := typedFields[*ChoiceField](lf)
	if err != nil {
		return err
	}
	for _, f := range fields {
		if err := f.checkEditText(); err != nil {
			return err
		}
	}
	for _, f := range fields {
		f.applyEditText(s)
	}
	return nil
}

// SetState sets the value of every field in lf, which must be check boxes or
// radio buttons, to v and updates the appearance state of every widget: a
// widget is on if its on-state equals v, and off otherwise.  A field none of
// whose on-states is v takes the value with all its widgets off.  Use "Off"
// to clear the fields.
//
// v must be "Off" or one of the on-states [ButtonField.OnState] returns for at
// least one of the fields; otherwise SetState returns an error and changes
// nothing.  The one exception is a check box without widgets or ExportValues
// entries, which accepts any v: this names the on-state of the widgets added
// later.
func (lf *LogicalField) SetState(v pdf.Name) error {
	fields, err := typedFields[*ButtonField](lf)
	if err != nil {
		return err
	}
	if v == "" {
		return errors.New("empty button value")
	}
	if !slices.ContainsFunc(fields, func(f *ButtonField) bool { return f.accepts(v) }) {
		return errors.New("value matches no on-state")
	}
	for _, f := range fields {
		f.setValue(v)
	}
	return nil
}

// Toggle performs a click on the widget w, which must belong to one of the
// fields of lf, and gives the resulting value to the other fields.  The
// fields must be check boxes or radio buttons.
//
// For a check box, the field switches between the widget's on-state and
// "Off".  For a radio button, the click selects the widget, or clears the
// field if the widget is already on, unless [FieldNoToggleToOff] is set.
// Where [FieldRadiosInUnison] is set, all widgets sharing the on-state of the
// clicked widget are selected together.
//
// Toggle returns an error and changes nothing if the fields are not check
// boxes or radio buttons, or if w is nil or belongs to none of the fields.
func (lf *LogicalField) Toggle(w Widget) error {
	fields, err := typedFields[*ButtonField](lf)
	if err != nil {
		return err
	}
	if w == nil {
		return errors.New("nil widget")
	}
	for k, f := range fields {
		i := slices.Index(f.Widgets, w)
		if i < 0 {
			continue
		}
		f.toggle(i)
		for j, other := range fields {
			if j != k {
				other.setValue(f.State)
			}
		}
		return nil
	}
	return errors.New("widget belongs to no field")
}
