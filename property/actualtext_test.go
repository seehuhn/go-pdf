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
			decoded, err := ExtractList(x, embedded)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}

			// verify ActualText value
			val, err := decoded.Get("ActualText")
			if err != nil {
				t.Fatalf("Get(ActualText) failed: %v", err)
			}
			pdfStr, ok := val.AsPDF(0).(pdf.String)
			if !ok {
				t.Fatalf("ActualText value is %T, want pdf.String", val.AsPDF(0))
			}
			if string(pdfStr.AsTextString()) != tc.at.Text {
				t.Errorf("Text = %q, want %q", pdfStr.AsTextString(), tc.at.Text)
			}

			// verify MCID if set
			if mcid, ok := tc.at.MCID.Get(); ok {
				mcidVal, err := decoded.Get("MCID")
				if err != nil {
					t.Fatalf("Get(MCID) failed: %v", err)
				}
				if got := mcidVal.AsPDF(0).(pdf.Integer); uint(got) != mcid {
					t.Errorf("MCID = %d, want %d", got, mcid)
				}
			}
		})
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
