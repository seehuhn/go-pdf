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
	"strconv"
	"strings"
	"unicode/utf8"

	"seehuhn.de/go/pdf"
)

// OnState returns the on-state name of the i-th button of a check box or
// radio button field, or the empty string if i is out of range.  See
// [ButtonField.OnStates] for how the name is found.
//
// For a radio button whose widget names no on-state, OnState takes time
// linear in the number of buttons.  Use [ButtonField.OnStates] to obtain the
// names of all buttons.
func (f *ButtonField) OnState(i int) pdf.Name {
	if i < 0 || i >= f.numOnStates() {
		return ""
	}
	if name := f.widgetOnState(i); name != "" {
		return name
	}
	if f.Flags&FieldRadio != 0 {
		return f.OnStates()[i]
	}
	return f.checkBoxOnState(i)
}

// OnStates returns the on-state names of the buttons of a check box or radio
// button field, in the order of the Widgets slice.  The result has one entry
// per widget or ExportValues entry, and at least one.
//
// A widget names its own on-state through the first state of its normal
// appearance other than "Off", or else through its current appearance
// state, if that is not "Off".  For a widget which names none, or a button
// without a widget:
//   - A radio button gets a decimal number.  These buttons take, in order,
//     the smallest numbers which no widget uses as its own name, so that the
//     on-states of all buttons are distinct.
//   - A check box uses its ExportValues entry, if any.  Otherwise it uses
//     State, or else DefaultState, if that is not "Off", and finally "Yes".
func (f *ButtonField) OnStates() []pdf.Name {
	n := f.numOnStates()
	names := make([]pdf.Name, n)
	for i := range n {
		names[i] = f.widgetOnState(i)
	}

	if f.Flags&FieldRadio == 0 {
		for i, name := range names {
			if name == "" {
				names[i] = f.checkBoxOnState(i)
			}
		}
		return names
	}

	taken := make(map[pdf.Name]bool, n)
	for _, name := range names {
		if name != "" {
			taken[name] = true
		}
	}
	// k only increases, so the search takes linear time overall
	k := 0
	for i, name := range names {
		if name != "" {
			continue
		}
		for taken[pdf.Name(strconv.Itoa(k))] {
			k++
		}
		names[i] = pdf.Name(strconv.Itoa(k))
		k++
	}
	return names
}

// widgetOnState returns the on-state name which the i-th widget names
// itself, through its normal appearance or its current appearance state, or
// the empty string if it names none.
func (f *ButtonField) widgetOnState(i int) pdf.Name {
	if i >= len(f.Widgets) || f.Widgets[i] == nil {
		return ""
	}
	w := f.Widgets[i]
	for _, name := range w.AppearanceStateNames() {
		if name != "Off" {
			return name
		}
	}
	if as := w.CurrentAppearanceState(); as != "" && as != "Off" {
		return as
	}
	return ""
}

// checkBoxOnState returns the on-state name of the i-th button of a check box
// whose widget names none.
func (f *ButtonField) checkBoxOnState(i int) pdf.Name {
	// "" and "Off" are valid export values, but no on-state names
	if i < len(f.ExportValues) && f.ExportValues[i] != "" && f.ExportValues[i] != "Off" {
		return pdf.Name(f.ExportValues[i])
	}
	// a single check box may name its on-state via the field value, or via
	// the default value once the field is off
	for _, n := range []pdf.Name{f.State, f.DefaultState} {
		if n != "" && n != "Off" {
			return n
		}
	}
	return "Yes"
}

// numOnStates returns the number of indices [ButtonField.OnState] accepts:
// one per widget or ExportValues entry, and at least one.
func (f *ButtonField) numOnStates() int {
	return max(len(f.Widgets), len(f.ExportValues), 1)
}

// toggle performs a click on widget i of a check box or radio button field.
// f.Widgets[i] must be a non-nil widget.
func (f *ButtonField) toggle(i int) {
	states := f.OnStates()
	on := states[i]
	isOn := f.Widgets[i].CurrentAppearanceState() == on

	if f.Flags&FieldRadio == 0 {
		if isOn {
			f.setValue("Off")
		} else {
			f.setValue(on)
		}
		return
	}

	if isOn {
		if f.Flags&FieldNoToggleToOff == 0 {
			f.setValue("Off")
		}
		return
	}
	f.State = on
	unison := f.Flags&FieldRadiosInUnison != 0
	for j, w := range f.Widgets {
		if w == nil {
			continue
		}
		if j == i || (unison && states[j] == on) {
			w.SetAppearanceState(on)
		} else {
			w.SetAppearanceState("Off")
		}
	}
}

// accepts reports whether the field accepts the non-empty value v.
func (f *ButtonField) accepts(v pdf.Name) bool {
	unnamed := f.Flags&FieldRadio == 0 && len(f.Widgets) == 0 && len(f.ExportValues) == 0
	return v == "Off" || unnamed || slices.Contains(f.OnStates(), v)
}

// setValue sets the field value to v and turns on the widgets whose on-state
// is v, without checking v.
func (f *ButtonField) setValue(v pdf.Name) {
	// the names are taken before any widget changes state, since a widget
	// may name its on-state only through its current state
	states := f.OnStates()
	f.State = v
	for j, w := range f.Widgets {
		if w == nil {
			continue
		}
		if v != "Off" && states[j] == v {
			w.SetAppearanceState(v)
		} else {
			w.SetAppearanceState("Off")
		}
	}
}

// checkText returns an error if s is not a valid value for the field.
func (f *TextField) checkText(s string) error {
	if f.MaxLen > 0 && utf8.RuneCountInString(s) > f.MaxLen {
		return errors.New("text exceeds MaxLen")
	}
	if f.Flags&FieldMultiline == 0 && strings.ContainsAny(s, "\r\n") {
		return errors.New("line break in single-line field")
	}
	return nil
}

// applyText sets the text value without checking s.  The stored value keeps
// the string or stream form of the previous value, and any rich text value is
// cleared.
func (f *TextField) applyText(s string) {
	f.Text = &pdf.StringOrStream{
		Value:    pdf.Text(s),
		IsStream: f.Text != nil && f.Text.IsStream,
	}
	f.RichText = nil
}

// checkSelection returns an error if sel, a sorted list of option indices
// without duplicates, is not a valid selection for the field.
func (f *ChoiceField) checkSelection(sel []int) error {
	for _, idx := range sel {
		if idx < 0 || idx >= len(f.Options) {
			return errors.New("option index out of range")
		}
	}
	if len(sel) > 1 && f.Flags&FieldMultiSelect == 0 {
		return errors.New("multiple selections require the MultiSelect flag")
	}
	return nil
}

// applySelection selects the options sel, a checked sorted list of option
// indices without duplicates.
func (f *ChoiceField) applySelection(sel []int) {
	v := make([]pdf.TextString, len(sel))
	for i, idx := range sel {
		v[i] = f.Options[idx].Display
	}
	f.Value = v
	if len(sel) == 0 && f.Flags&FieldMultiSelect == 0 {
		f.Value = nil
	}
	f.recordSelection(sel)
	f.RichText = nil
}

// recordSelection sets Selected to the ascending indices sel where Value alone
// does not identify the selected options, and to nil otherwise; see
// [LogicalField.Select].
func (f *ChoiceField) recordSelection(sel []int) {
	f.Selected = nil
	need := len(sel) > 1
	for _, idx := range sel {
		for j, o := range f.Options {
			if j != idx && o.Display == f.Options[idx].Display {
				need = true
			}
		}
	}
	if need {
		f.Selected = sel
	}
}

// checkEditText returns an error if the field is not an editable combo box.
func (f *ChoiceField) checkEditText() error {
	if !f.IsEditable() {
		return errors.New("not an editable combo box")
	}
	return nil
}

// applyEditText sets the text of an editable combo box without checking the
// field.
func (f *ChoiceField) applyEditText(s string) {
	text := pdf.Text(s)
	f.Value = []pdf.TextString{text}
	var sel []int
	for i, o := range f.Options {
		if o.Display == text {
			sel = []int{i}
			break
		}
	}
	f.recordSelection(sel)
	f.RichText = nil
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
		err = f.fillValue(rm, dict)
	case *PushButtonField:
		err = f.fillValue(rm, dict)
	case *ChoiceField:
		err = f.fillValue(rm, dict)
	case *SignatureField:
		err = f.fillValue(rm, dict)
	}
	if err != nil {
		return nil, err
	}
	return dict, nil
}
