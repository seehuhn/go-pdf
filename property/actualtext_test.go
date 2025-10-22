// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package property

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestActualTextRoundTrip(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	original := &ActualText{
		Text:      "replacement text",
		SingleUse: true,
	}

	// embed
	embedded, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("rm.Close failed: %v", err)
	}

	// extract
	x := pdf.NewExtractor(w)
	decoded, err := ExtractActualText(x, embedded)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if decoded.Text != original.Text {
		t.Errorf("Text = %q, want %q", decoded.Text, original.Text)
	}
}

func TestActualTextIsDirect(t *testing.T) {
	tests := []struct {
		name       string
		singleUse  bool
		wantDirect bool
	}{
		{"SingleUse true", true, true},
		{"SingleUse false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ActualText{
				Text:      "test",
				SingleUse: tt.singleUse,
			}
			if got := a.IsDirect(); got != tt.wantDirect {
				t.Errorf("IsDirect() = %v, want %v", got, tt.wantDirect)
			}
		})
	}
}
