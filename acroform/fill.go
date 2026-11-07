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

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"seehuhn.de/go/pdf"
)

// OnState returns the on-state name of the i-th widget of a check box or
// radio button field, or the empty string for a push button or if i is out
// of range.
//
// The name is taken from the widget's normal appearance, if that has a state
// other than "Off".  Otherwise the field's Opt entry is used, then (for a
// check box) the field value or the default value, and finally "On".
func (f *ButtonField) OnState(i int) pdf.Name {
	if f.Flags&FieldPushbutton != 0 || i < 0 || i >= f.numOnStates() {
		return ""
	}
	if i < len(f.Widgets) && f.Widgets[i] != nil {
		for _, name := range f.Widgets[i].AppearanceStateNames() {
			if name != "Off" {
				return name
			}
		}
	}
	if i < len(f.Opt) {
		return pdf.Name(f.Opt[i])
	}
	// a single check box may name its on-state via the field value, or via
	// the default value once the field is off
	if f.Flags&FieldRadio == 0 {
		for _, n := range []pdf.Name{f.V, f.DV} {
			if n != "" && n != "Off" {
				return n
			}
		}
	}
	return "On"
}

// numOnStates returns the number of indices [ButtonField.OnState] accepts:
// one per widget or Opt entry, and at least one.
func (f *ButtonField) numOnStates() int {
	return max(len(f.Widgets), len(f.Opt), 1)
}

// Toggle performs a click on widget i of a check box or radio button field.
//
// For a check box, the field switches between the widget's on-state and
// "Off".  For a radio button, the click selects widget i, or clears the
// field if widget i is already on, unless [FieldNoToggleToOff] is set.
// Where [FieldRadiosInUnison] is set, all widgets sharing the on-state of
// widget i are selected together.
func (f *ButtonField) Toggle(i int) error {
	if f.Flags&FieldPushbutton != 0 {
		return errors.New("push button has no value")
	}
	if i < 0 || i >= f.numOnStates() {
		return errors.New("widget index out of range")
	}
	on := f.OnState(i)
	isOn := f.V == on
	if i < len(f.Widgets) && f.Widgets[i] != nil {
		isOn = f.Widgets[i].CurrentAppearanceState() == on
	}

	if f.Flags&FieldRadio == 0 {
		if isOn {
			return f.SetValue("Off")
		}
		return f.SetValue(on)
	}

	if isOn {
		if f.Flags&FieldNoToggleToOff != 0 {
			return nil
		}
		return f.SetValue("Off")
	}
	f.V = on
	unison := f.Flags&FieldRadiosInUnison != 0
	for j, w := range f.Widgets {
		if w == nil {
			continue
		}
		if j == i || (unison && f.OnState(j) == on) {
			w.SetAppearanceState(on)
		} else {
			w.SetAppearanceState("Off")
		}
	}
	return nil
}

// SetValue sets the field value to v and updates the appearance state of
// every widget: a widget is on if its on-state equals v, and off otherwise.
// Use "Off" to clear the field.
//
// If the field has widgets or an Opt entry, v must be "Off" or one of the
// on-states [ButtonField.OnState] returns for them; otherwise SetValue
// returns an error and changes nothing.
func (f *ButtonField) SetValue(v pdf.Name) error {
	if f.Flags&FieldPushbutton != 0 {
		return errors.New("push button has no value")
	}
	if v == "" {
		return errors.New("empty button value")
	}
	if v != "Off" && (len(f.Widgets) > 0 || len(f.Opt) > 0) {
		known := false
		for j := range f.numOnStates() {
			if f.OnState(j) == v {
				known = true
				break
			}
		}
		if !known {
			return errors.New("value matches no on-state")
		}
	}
	f.V = v
	for j, w := range f.Widgets {
		if w == nil {
			continue
		}
		if v != "Off" && f.OnState(j) == v {
			w.SetAppearanceState(v)
		} else {
			w.SetAppearanceState("Off")
		}
	}
	return nil
}

// SetText sets the text value of the field.  The stored value keeps the
// string or stream form of the previous value, and any rich text value is
// cleared.  An empty s gives an explicit empty value, not an absent one.
//
// SetText returns an error and changes nothing if the field is a password
// field, if s has more than MaxLen characters, or if s contains a line break
// and the field does not have the [FieldMultiline] flag.
func (f *TextField) SetText(s string) error {
	if f.Flags&FieldPassword != 0 {
		return errors.New("password field value must not be stored")
	}
	if f.MaxLen > 0 && utf8.RuneCountInString(s) > f.MaxLen {
		return errors.New("text exceeds MaxLen")
	}
	if f.Flags&FieldMultiline == 0 && strings.ContainsAny(s, "\r\n") {
		return errors.New("line break in single-line field")
	}
	f.V = &pdf.StringOrStream{
		Value:    pdf.Text(s),
		IsStream: f.V != nil && f.V.IsStream,
	}
	f.RichValue = nil
	return nil
}

// Select selects the options with the given indices into Opt, sets V to
// their display strings, and clears any rich text value.  The order of the
// indices and duplicates are ignored.
//
// An empty list of indices clears the selection.  With [FieldMultiSelect],
// V is then an explicit empty value; without it, V is absent.
//
// Selected is set only where V alone does not identify the options: if
// several options are selected, or if a selected option shares its display
// string with another.  Otherwise Selected is nil, so that the field can be
// written in PDF versions before 1.4, which have no /I entry.
//
// Select returns an error and changes nothing if an index is out of range,
// or if several options are selected in a field without the
// [FieldMultiSelect] flag.
func (f *ChoiceField) Select(indices []int) error {
	sel := slices.Clone(indices)
	slices.Sort(sel)
	sel = slices.Compact(sel)
	for _, idx := range sel {
		if idx < 0 || idx >= len(f.Opt) {
			return errors.New("option index out of range")
		}
	}
	if len(sel) > 1 && f.Flags&FieldMultiSelect == 0 {
		return errors.New("multiple selections require the MultiSelect flag")
	}
	v := make([]pdf.TextString, len(sel))
	for i, idx := range sel {
		v[i] = f.Opt[idx].Display
	}
	f.V = v
	if len(sel) == 0 && f.Flags&FieldMultiSelect == 0 {
		f.V = nil
	}
	f.recordSelection(sel)
	f.RichValue = nil
	return nil
}

// recordSelection sets Selected to the ascending indices sel where V alone
// does not identify the selected options, and to nil otherwise; see
// [ChoiceField.Select].
func (f *ChoiceField) recordSelection(sel []int) {
	f.Selected = nil
	need := len(sel) > 1
	for _, idx := range sel {
		for j, o := range f.Opt {
			if j != idx && o.Display == f.Opt[idx].Display {
				need = true
			}
		}
	}
	if need {
		f.Selected = sel
	}
}

// SetEditText sets the text of an editable combo box and clears any rich
// text value.  If s is the display string of an option, the first such
// option becomes the selection; otherwise nothing is selected and V holds s
// alone.  Selected is set as for [ChoiceField.Select].
//
// SetEditText returns an error if the field does not have both the
// [FieldCombo] and [FieldEdit] flags.
func (f *ChoiceField) SetEditText(s string) error {
	const both = FieldCombo | FieldEdit
	if f.Flags&both != both {
		return errors.New("not an editable combo box")
	}
	text := pdf.Text(s)
	f.V = []pdf.TextString{text}
	var sel []int
	for i, o := range f.Opt {
		if o.Display == text {
			sel = []int{i}
			break
		}
	}
	f.recordSelection(sel)
	f.RichValue = nil
	return nil
}

// ValueEntries returns the value entries of f's field dictionary as
// Encode writes them.  It governs the keys /V, /I and /RV: a caller
// patching a stored dictionary sets the keys returned and deletes the
// governed keys not returned.
//
// Only the value is encoded, so the other entries of the field are neither
// checked nor embedded.
func ValueEntries(rm *pdf.ResourceManager, f Field) (pdf.Dict, error) {
	dict := pdf.Dict{}
	var err error
	switch f := f.(type) {
	case *TextField:
		err = f.fillValue(rm, dict)
	case *ButtonField:
		f.fillValue(dict)
	case *ChoiceField:
		err = f.fillValue(rm, dict)
	case *SignatureField:
		err = f.fillValue(rm, dict)
	}
	if err != nil {
		return nil, err
	}
	if vt, ok := f.(VariableTextField); ok {
		if err := vt.GetVariableText().fillRichValue(rm, dict); err != nil {
			return nil, err
		}
	}
	return dict, nil
}
