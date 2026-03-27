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

package annotation

import (
	"testing"

	"seehuhn.de/go/pdf"
)

// TestGetMarkup verifies that GetMarkup is accessible via interface assertion
// on all annotation types that embed Markup.
func TestGetMarkup(t *testing.T) {
	type hasMarkup interface {
		GetMarkup() *Markup
	}

	markupAnnotations := []Annotation{
		&Text{},
		&FreeText{},
		&Line{},
		&Square{},
		&Circle{},
		&Polygon{},
		&PolyLine{},
		&TextMarkup{Type: TextMarkupTypeHighlight},
		&Stamp{},
		&Caret{},
		&Ink{},
		&FileAttachment{},
		&Sound{},
		&Redact{},
		&Projection{},
	}

	for _, a := range markupAnnotations {
		m, ok := a.(hasMarkup)
		if !ok {
			t.Errorf("%T does not implement hasMarkup", a)
			continue
		}
		markup := m.GetMarkup()
		if markup == nil {
			t.Errorf("%T.GetMarkup() returned nil", a)
			continue
		}

		// verify that setting a field through GetMarkup is visible
		ref := pdf.NewReference(42, 0)
		markup.InReplyTo = ref
		got := m.GetMarkup().InReplyTo
		if got != ref {
			t.Errorf("%T: InReplyTo = %v, want %v", a, got, ref)
		}
	}

	// non-markup annotations must not satisfy the interface
	nonMarkup := []Annotation{
		&Link{},
		&Widget{},
	}
	for _, a := range nonMarkup {
		if _, ok := a.(hasMarkup); ok {
			t.Errorf("%T should not implement hasMarkup", a)
		}
	}
}
