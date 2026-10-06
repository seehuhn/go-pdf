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
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestFieldFlagsNormalize(t *testing.T) {
	const reserved FieldFlags = 1 << 31
	tests := []struct {
		name string
		ft   pdf.Name
		in   FieldFlags
		want FieldFlags
	}{
		{"common flags kept", "Sig", FieldReadOnly | FieldRequired | FieldNoExport, FieldReadOnly | FieldRequired | FieldNoExport},
		{"reserved bit cleared", "Tx", FieldMultiline | reserved, FieldMultiline},
		{"wrong type flag cleared", "Tx", FieldRadio | FieldPassword, FieldPassword},
		{"button flag on signature", "Sig", FieldPushbutton, 0},
		{"radio with pushbutton", "Btn", FieldRadio | FieldPushbutton, FieldPushbutton},
		{"no toggle without radio", "Btn", FieldNoToggleToOff, 0},
		{"unison without radio", "Btn", FieldRadiosInUnison, 0},
		{"unison on radio kept", "Btn", FieldRadio | FieldRadiosInUnison | FieldNoToggleToOff, FieldRadio | FieldRadiosInUnison | FieldNoToggleToOff},
		{"rich text on text kept", "Tx", FieldRichText, FieldRichText},
		{"edit without combo", "Ch", FieldEdit, 0},
		{"edit with combo kept", "Ch", FieldCombo | FieldEdit, FieldCombo | FieldEdit},
		{"spell check on list box", "Ch", FieldDoNotSpellCheck, 0},
		{"spell check on plain combo", "Ch", FieldCombo | FieldDoNotSpellCheck, FieldCombo},
		{"spell check on editable combo", "Ch", FieldCombo | FieldEdit | FieldDoNotSpellCheck, FieldCombo | FieldEdit | FieldDoNotSpellCheck},
		{"spell check on text kept", "Tx", FieldDoNotSpellCheck, FieldDoNotSpellCheck},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.Normalize(tc.ft); got != tc.want {
				t.Errorf("Normalize(%q, %#x) = %#x, want %#x", tc.ft, uint32(tc.in), uint32(got), uint32(tc.want))
			}
		})
	}
}

func TestEncodeFlagValidation(t *testing.T) {
	const reserved FieldFlags = 1 << 31
	button := func(ff FieldFlags) *ButtonField {
		f := NewButtonField("b")
		f.Flags = ff
		return f
	}
	text := func(ff FieldFlags) *TextField {
		f := textField("t")
		f.Flags = ff
		return f
	}
	choice := func(ff FieldFlags) *ChoiceField {
		f := NewChoiceField("c")
		f.DefaultAppearance = testDA
		f.Flags = ff
		return f
	}
	sig := func(ff FieldFlags) *SignatureField {
		f := NewSignatureField("s")
		f.Flags = ff
		return f
	}
	tests := []struct {
		name    string
		field   Field
		wantErr bool
	}{
		{"check box", button(0), false},
		{"radio", button(FieldRadio | FieldNoToggleToOff | FieldRadiosInUnison), false},
		{"push button", button(FieldPushbutton | FieldReadOnly), false},
		{"radio and push button", button(FieldRadio | FieldPushbutton), true},
		{"no toggle without radio", button(FieldNoToggleToOff), true},
		{"unison without radio", button(FieldRadiosInUnison), true},
		{"text flag on button", button(FieldMultiline), true},
		{"reserved bit on button", button(reserved), true},
		{"text", text(FieldMultiline | FieldDoNotSpellCheck | FieldRichText), false},
		{"button flag on text", text(FieldRadio), true},
		{"reserved bit on text", text(reserved), true},
		{"list box", choice(FieldMultiSelect | FieldSort), false},
		{"editable combo", choice(FieldCombo | FieldEdit | FieldDoNotSpellCheck), false},
		{"edit without combo", choice(FieldEdit), true},
		{"spell check on list box", choice(FieldDoNotSpellCheck), true},
		{"spell check on plain combo", choice(FieldCombo | FieldDoNotSpellCheck), true},
		{"text flag on choice", choice(FieldMultiline), true},
		{"signature", sig(FieldReadOnly | FieldRequired), false},
		{"button flag on signature", sig(FieldPushbutton), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			_, err := terminalEntries(rm, tc.field)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
