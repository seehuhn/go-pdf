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
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestTrapNetEncodeValidCombinations verifies that Encode accepts valid
// field combinations for LastModified/Version/AnnotStates.
func TestTrapNetEncodeValidCombinations(t *testing.T) {
	for _, tc := range []struct {
		name string
		data TrapNet
	}{
		{
			name: "LastModified only",
			data: TrapNet{
				Common:       Common{Rect: pdf.Rectangle{URx: 612, URy: 792}},
				LastModified: "D:20231215103000Z",
			},
		},
		{
			name: "Version and AnnotStates",
			data: TrapNet{
				Common:      Common{Rect: pdf.Rectangle{URx: 612, URy: 792}},
				Version:     []pdf.Reference{pdf.NewReference(1, 0)},
				AnnotStates: []pdf.Name{"N"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			_, err := tc.data.Encode(rm)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestTrapNetEncodeInvalidCombinations verifies that Encode rejects invalid
// field combinations for LastModified/Version/AnnotStates.
func TestTrapNetEncodeInvalidCombinations(t *testing.T) {
	for _, tc := range []struct {
		name string
		data TrapNet
	}{
		{
			name: "none present",
			data: TrapNet{
				Common: Common{Rect: pdf.Rectangle{URx: 612, URy: 792}},
			},
		},
		{
			name: "all three present",
			data: TrapNet{
				Common:       Common{Rect: pdf.Rectangle{URx: 612, URy: 792}},
				LastModified: "D:20231215103000Z",
				Version:      []pdf.Reference{pdf.NewReference(1, 0)},
				AnnotStates:  []pdf.Name{"N"},
			},
		},
		{
			name: "LastModified and Version",
			data: TrapNet{
				Common:       Common{Rect: pdf.Rectangle{URx: 612, URy: 792}},
				LastModified: "D:20231215103000Z",
				Version:      []pdf.Reference{pdf.NewReference(1, 0)},
			},
		},
		{
			name: "LastModified and AnnotStates",
			data: TrapNet{
				Common:       Common{Rect: pdf.Rectangle{URx: 612, URy: 792}},
				LastModified: "D:20231215103000Z",
				AnnotStates:  []pdf.Name{"N"},
			},
		},
		{
			name: "Version only",
			data: TrapNet{
				Common:  Common{Rect: pdf.Rectangle{URx: 612, URy: 792}},
				Version: []pdf.Reference{pdf.NewReference(1, 0)},
			},
		},
		{
			name: "AnnotStates only",
			data: TrapNet{
				Common:      Common{Rect: pdf.Rectangle{URx: 612, URy: 792}},
				AnnotStates: []pdf.Name{"N"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			_, err := tc.data.Encode(rm)
			if err == nil {
				t.Fatal("expected error for invalid field combination")
			}
		})
	}
}

// TestTrapNetEncodeLastModifiedV13 verifies that LastModified-only fails on
// PDF 1.3 because LastModified requires PDF 1.4.
func TestTrapNetEncodeLastModifiedV13(t *testing.T) {
	tn := TrapNet{
		Common:       Common{Rect: pdf.Rectangle{URx: 612, URy: 792}},
		LastModified: "D:20231215103000Z",
	}
	w, _ := memfile.NewPDFWriter(pdf.V1_3, nil)
	rm := pdf.NewResourceManager(w)
	_, err := tn.Encode(rm)
	if err == nil {
		t.Fatal("expected error for LastModified on PDF 1.3")
	}
}
