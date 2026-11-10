// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
