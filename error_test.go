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

package pdf_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestVersionErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  pdf.VersionError
		want string
	}{
		{
			name: "earliest only",
			err:  pdf.VersionError{Operation: "feature X", Earliest: pdf.V1_4},
			want: "feature X requires PDF version 1.4 or later",
		},
		{
			name: "latest only",
			err:  pdf.VersionError{Operation: "feature Y", Latest: pdf.V1_7},
			want: "feature Y requires PDF version 1.7 or earlier",
		},
		{
			name: "earliest equals latest",
			err:  pdf.VersionError{Operation: "feature Z", Earliest: pdf.V1_5, Latest: pdf.V1_5},
			want: "feature Z requires PDF version 1.5",
		},
		{
			name: "range",
			err:  pdf.VersionError{Operation: "feature W", Earliest: pdf.V1_3, Latest: pdf.V1_7},
			want: "feature W requires PDF version between 1.3 and 1.7",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
			if !pdf.IsWrongVersion(&tc.err) {
				t.Error("IsWrongVersion returned false for *VersionError")
			}
		})
	}

	// degenerate case: neither bound set — verify type recognition,
	// not message text
	t.Run("neither", func(t *testing.T) {
		err := &pdf.VersionError{Operation: "feature Q"}
		if !pdf.IsWrongVersion(err) {
			t.Error("IsWrongVersion returned false for *VersionError")
		}
	})
}

func TestCheckVersionAtMost(t *testing.T) {
	w20, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	if err := pdf.CheckVersionAtMost(w20, "op", pdf.V1_7); err == nil {
		t.Fatal("expected VersionError for V2.0 writer + max V1.7")
	} else if !pdf.IsWrongVersion(err) {
		t.Errorf("got %T, want *VersionError", err)
	}

	w17, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := pdf.CheckVersionAtMost(w17, "op", pdf.V1_7); err != nil {
		t.Errorf("V1.7 writer + max V1.7 should pass, got %v", err)
	}

	w14, _ := memfile.NewPDFWriter(pdf.V1_4, nil)
	if err := pdf.CheckVersionAtMost(w14, "op", pdf.V1_7); err != nil {
		t.Errorf("V1.4 writer + max V1.7 should pass, got %v", err)
	}
}
