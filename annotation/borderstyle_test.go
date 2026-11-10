// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package annotation

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

// TestBorderStyleInvalidWidth checks that a reference to a nonexistent
// object, and a value of the wrong type, are read like an absent /W entry.
func TestBorderStyleInvalidWidth(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)

	want, err := ExtractBorderStyle(c, pdf.Dict{}, true)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]pdf.Object{
		"dangling": pdf.NewReference(99, 0),
		"name":     pdf.Name("x"),
	} {
		got, err := ExtractBorderStyle(c, pdf.Dict{"W": value}, true)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s value read differently from absent entry (-absent +got):\n%s", name, diff)
		}
	}
}
