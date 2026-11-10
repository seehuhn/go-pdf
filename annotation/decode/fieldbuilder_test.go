// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
)

// a group nesting terminal fields with values round-trips, with the field type
// and value attributes flattened back onto each terminal
func TestTreeRoundTrip(t *testing.T) {
	text := newTextField("text")
	text.Text = &pdf.StringOrStream{Value: pdf.Text("hello")}
	text.DefaultAppearance = "/Helv 12 Tf 0 g"
	other := newTextField("note")
	other.DefaultAppearance = "/Helv 12 Tf 0 g"

	root := &acroform.Group{Name: "request", Children: []acroform.Node{text, other}}

	got := roundTripRoots(t, pdf.V1_7, root)
	if diff := cmp.Diff(snapNodes(t, []acroform.Node{root}), snapNodes(t, got), fieldCmpOptions()...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}
