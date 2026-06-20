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
	testCases := []struct {
		name string
		at   *ActualText
	}{
		{
			name: "SingleUse true, no MCID",
			at: &ActualText{
				Text:      "replacement text",
				SingleUse: true,
			},
		},
		{
			name: "SingleUse false, no MCID",
			at: &ActualText{
				Text:      "indirect text",
				SingleUse: false,
			},
		},
		{
			name: "with MCID",
			at: func() *ActualText {
				a := &ActualText{Text: "text with mcid"}
				a.MCID.Set(42)
				return a
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)

			// embed
			embedded, err := rm.Embed(tc.at)
			if err != nil {
				t.Fatalf("embed failed: %v", err)
			}

			err = rm.Close()
			if err != nil {
				t.Fatalf("rm.Close failed: %v", err)
			}

			// extract using generic ExtractList
			x := pdf.NewExtractor(w)
			decoded, err := ExtractList(pdf.CursorAt(x, nil), embedded, tc.at.SingleUse)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}

			// verify via AsDirectDict for inline case
			if tc.at.SingleUse {
				dict := decoded.AsDirectDict()
				if dict == nil {
					t.Fatal("AsDirectDict() returned nil for inline list")
				}
				obj := dict["ActualText"]
				if obj == nil {
					t.Fatal("ActualText not found in dict")
				}
				// the value may be pdf.TextString or pdf.String depending on path
				got := obj.AsPDF(0).(pdf.String).AsTextString()
				if string(got) != tc.at.Text {
					t.Errorf("Text = %q, want %q", got, tc.at.Text)
				}
			} else {
				if decoded.AsDirectDict() != nil {
					t.Error("AsDirectDict() should be nil for indirect list")
				}
			}
		})
	}
}

func TestActualTextAsDirectDict(t *testing.T) {
	tests := []struct {
		name     string
		at       *ActualText
		wantNil  bool
		wantKeys []pdf.Name
	}{
		{
			name:     "SingleUse true",
			at:       &ActualText{Text: "test", SingleUse: true},
			wantNil:  false,
			wantKeys: []pdf.Name{"ActualText"},
		},
		{
			name:    "SingleUse false",
			at:      &ActualText{Text: "test", SingleUse: false},
			wantNil: true,
		},
		{
			name: "SingleUse true with MCID",
			at: func() *ActualText {
				a := &ActualText{Text: "test", SingleUse: true}
				a.MCID.Set(7)
				return a
			}(),
			wantNil:  false,
			wantKeys: []pdf.Name{"ActualText", "MCID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dict := tt.at.AsDirectDict()
			if tt.wantNil {
				if dict != nil {
					t.Error("AsDirectDict() should return nil")
				}
				return
			}
			if dict == nil {
				t.Fatal("AsDirectDict() returned nil")
			}
			for _, key := range tt.wantKeys {
				if _, ok := dict[key]; !ok {
					t.Errorf("missing key %q", key)
				}
			}
		})
	}
}
