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
	text := acroform.NewTextField("text")
	text.V = pdf.String("hello")
	text.DefaultAppearance = "/Helv 12 Tf 0 g"
	other := acroform.NewTextField("note")
	other.DefaultAppearance = "/Helv 12 Tf 0 g"

	root := &acroform.Group{Name: "request", Kids: []acroform.TreeNode{text, other}}

	got := roundTripRoots(t, pdf.V1_7, root)
	if diff := cmp.Diff(snapNodes([]acroform.TreeNode{root}), snapNodes(got), fieldCmpOptions()...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}
