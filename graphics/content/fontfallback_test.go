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

package content

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
)

// stubFont is a font.Instance which only carries a name.
type stubFont struct {
	font.Instance
	name string
}

func (f *stubFont) PostScriptName() string { return f.name }

func setFont(t *testing.T, s *State, name pdf.Name, size float64) {
	t.Helper()
	err := s.ApplyStateChanges(OpTextSetFont, []pdf.Object{name, pdf.Number(size)})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTfHit(t *testing.T) {
	F := &stubFont{name: "F1"}
	called := false
	res := &Resources{
		Font:         map[pdf.Name]font.Instance{"F1": F},
		FontFallback: func(pdf.Name) font.Instance { called = true; return nil },
	}
	s := NewState(Page, res)
	setFont(t, s, "F1", 12)

	if s.GState.TextFont != F || s.GState.TextFontSize != 12 {
		t.Errorf("font not set: got %v at %g", s.GState.TextFont, s.GState.TextFontSize)
	}
	if !s.IsSet(graphics.StateTextFont) || !s.IsUsable(graphics.StateTextFont) {
		t.Error("font bits not set after a hit")
	}
	if called {
		t.Error("fallback consulted although the font was found")
	}
}

func TestTfMissWithFallback(t *testing.T) {
	sub := &stubFont{name: "Substitute"}
	var asked pdf.Name
	res := &Resources{
		FontFallback: func(name pdf.Name) font.Instance { asked = name; return sub },
	}
	s := NewState(Page, res)
	setFont(t, s, "Missing", 9)

	if asked != "Missing" {
		t.Errorf("fallback asked for %q, want %q", asked, "Missing")
	}
	if s.GState.TextFont != sub || s.GState.TextFontSize != 9 {
		t.Errorf("substitute not set: got %v at %g", s.GState.TextFont, s.GState.TextFontSize)
	}
	if !s.IsSet(graphics.StateTextFont) || !s.IsUsable(graphics.StateTextFont) {
		t.Error("font bits not set after a substitution")
	}
}

func TestTfMissWithoutFallback(t *testing.T) {
	// no font in force: the miss leaves the state without one
	s := NewState(Page, &Resources{})
	setFont(t, s, "Missing", 9)
	if s.GState.TextFont != nil {
		t.Errorf("font set on a miss: %v", s.GState.TextFont)
	}
	if s.IsSet(graphics.StateTextFont) || s.IsUsable(graphics.StateTextFont) {
		t.Error("font bits set although no font is in force")
	}

	// a font in force stays in force, size included
	F := &stubFont{name: "F1"}
	s = NewState(Page, &Resources{Font: map[pdf.Name]font.Instance{"F1": F}})
	setFont(t, s, "F1", 12)
	setFont(t, s, "Missing", 9)
	if s.GState.TextFont != F || s.GState.TextFontSize != 12 {
		t.Errorf("previous font disturbed by a miss: got %v at %g", s.GState.TextFont, s.GState.TextFontSize)
	}
	if !s.IsSet(graphics.StateTextFont) || !s.IsUsable(graphics.StateTextFont) {
		t.Error("font bits lost on a miss")
	}

	// a fallback which declines leaves the inherited state of a form alone
	declined := func(pdf.Name) font.Instance { return nil }
	s = NewState(Form, &Resources{FontFallback: declined})
	setFont(t, s, "Missing", 9)
	if s.GState.TextFont != nil || s.IsSet(graphics.StateTextFont) {
		t.Error("declined fallback changed the state")
	}
	if !s.IsUsable(graphics.StateTextFont) {
		t.Error("form lost its inherited font on a miss")
	}
}
