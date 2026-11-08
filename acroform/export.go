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
	"iter"
	"slices"

	"seehuhn.de/go/pdf"
)

// ExportData returns an iterator over the data the form contributes when its
// field values are exported, for example by a submit-form action.  Each
// pair consists of a field's export name and one export value.  Fields are
// visited in depth-first tree order.
//
// Fields which share a fully qualified name are one field, and their data is
// exported once, in the position of the first of them.  They must hold the
// same value, as [InteractiveForm.Encode] requires.  Such a field is left
// out if one of them has the [FieldNoExport] flag, and its export name is
// the first [Common.ExportName] among them, or else the fully qualified
// name.  The value of a check box or radio button is exported as the export
// value of the button which is on, from whichever field has that button.
// The value of a choice field is mapped to export values through the options
// of the first field which lists all selected options.
//
// Fields without a value, check boxes and radio buttons which are off, push
// buttons and signature fields contribute no data.
func (f *InteractiveForm) ExportData() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		for lf := range f.LogicalFields() {
			exportName, ok := lf.exportName()
			if !ok {
				continue
			}
			for _, value := range lf.exportData() {
				if !yield(exportName, value) {
					return
				}
			}
		}
	}
}

// exportName returns the name under which the fields of lf are exported.
// The second return value is false if one of them has the [FieldNoExport]
// flag.
func (lf *LogicalField) exportName() (string, bool) {
	exportName := ""
	for _, field := range lf.Fields {
		c := field.GetCommon()
		if c.Flags&FieldNoExport != 0 {
			return "", false
		}
		if exportName == "" {
			exportName = c.ExportName
		}
	}
	if exportName == "" {
		exportName = lf.Name
	}
	return exportName, true
}

// exportData returns the export values of the fields of lf, which hold one
// value.  This is the value of the first field, mapped to export values by the
// field which has the selected button or options.
func (lf *LogicalField) exportData() []string {
	switch first := lf.Fields[0].(type) {
	case *ButtonField:
		state := first.State
		if state == "" || state == "Off" {
			return nil
		}
		for _, field := range lf.Fields {
			btn, ok := field.(*ButtonField)
			if !ok {
				continue
			}
			i := slices.Index(btn.OnStates(), state)
			if i < 0 {
				continue
			}
			if i < len(btn.ExportValues) {
				return []string{btn.ExportValues[i]}
			}
			break
		}
		return []string{string(state)}

	case *ChoiceField:
		for _, field := range lf.Fields {
			ch, ok := field.(*ChoiceField)
			if !ok {
				continue
			}
			if !slices.ContainsFunc(first.Value, func(v pdf.TextString) bool { return !ch.hasOption(v) }) {
				return ch.ExportData()
			}
		}
	}
	return lf.Fields[0].ExportData()
}
