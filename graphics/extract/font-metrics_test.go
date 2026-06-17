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

package extract

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/postscript/cid"
)

func TestCompositeWidthsValid(t *testing.T) {
	// range form for CIDs 1..3, individual form for CIDs 10..11
	w := pdf.Array{
		pdf.Integer(1), pdf.Integer(3), pdf.Integer(500),
		pdf.Integer(10), pdf.Array{pdf.Integer(700), pdf.Integer(800)},
	}
	got, err := decodeCompositeWidths(mock.Getter, w)
	if err != nil {
		t.Fatal(err)
	}
	want := map[cid.CID]float64{
		1: 500, 2: 500, 3: 500,
		10: 700, 11: 800,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}
}

// the maximal valid range covers all 65536 CIDs and must be accepted
func TestCompositeWidthsFullRange(t *testing.T) {
	w := pdf.Array{pdf.Integer(0), pdf.Integer(65535), pdf.Integer(1000)}
	got, err := decodeCompositeWidths(mock.Getter, w)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 65536 {
		t.Errorf("got %d entries, want 65536", len(got))
	}
}

// assignments summing to exactly the 65536 budget across several W entries must
// be accepted; the cap rejects only what exceeds it
func TestCompositeWidthsBudgetBoundary(t *testing.T) {
	// two disjoint ranges of 32768 CIDs each, totalling exactly 65536
	w := pdf.Array{
		pdf.Integer(0), pdf.Integer(32767), pdf.Integer(500),
		pdf.Integer(32768), pdf.Integer(65535), pdf.Integer(600),
	}
	got, err := decodeCompositeWidths(mock.Getter, w)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 65536 {
		t.Errorf("got %d entries, want 65536", len(got))
	}
	if got[0] != 500 || got[65535] != 600 {
		t.Errorf("got widths %v/%v, want 500/600", got[0], got[65535])
	}
}

// overlapping ranges within the budget are tolerated, with later entries
// overriding earlier ones for the shared CIDs
func TestCompositeWidthsOverlapLastWins(t *testing.T) {
	w := pdf.Array{
		pdf.Integer(0), pdf.Integer(10), pdf.Integer(500),
		pdf.Integer(5), pdf.Integer(15), pdf.Integer(600),
	}
	got, err := decodeCompositeWidths(mock.Getter, w)
	if err != nil {
		t.Fatal(err)
	}
	want := map[cid.CID]float64{
		0: 500, 1: 500, 2: 500, 3: 500, 4: 500,
		5: 600, 6: 600, 7: 600, 8: 600, 9: 600, 10: 600,
		11: 600, 12: 600, 13: 600, 14: 600, 15: 600,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}
}

// CIDs above 65535 and total widths beyond 65536 must be rejected, bounding
// both the map size and the loop work for crafted W arrays
func TestCompositeWidthsReject(t *testing.T) {
	cases := map[string]pdf.Array{
		// range form, high absolute CID with a small span
		"range high CID": {pdf.Integer(70000), pdf.Integer(70100), pdf.Integer(1000)},
		// range form, end just past the limit
		"range end past limit": {pdf.Integer(65535), pdf.Integer(65536), pdf.Integer(1000)},
		// individual form, high start CID
		"indiv high CID": {pdf.Integer(70000), pdf.Array{pdf.Integer(1000)}},
		// individual form crossing the limit mid-array
		"indiv crosses limit": {pdf.Integer(65535), pdf.Array{pdf.Integer(1), pdf.Integer(2)}},
		// disjoint maximal ranges (the memory-bomb shape)
		"disjoint ranges": {
			pdf.Integer(0), pdf.Integer(65535), pdf.Integer(1000),
			pdf.Integer(65536), pdf.Integer(131071), pdf.Integer(1000),
		},
		// repeated maximal range (the CPU-only shape)
		"repeated range": {
			pdf.Integer(0), pdf.Integer(65535), pdf.Integer(1000),
			pdf.Integer(0), pdf.Integer(65535), pdf.Integer(1000),
		},
	}
	for name, w := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeCompositeWidths(mock.Getter, w); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestVMetricsValid(t *testing.T) {
	// individual form for CIDs 1..2, range form for CIDs 5..6
	a := pdf.Array{
		pdf.Integer(1), pdf.Array{
			pdf.Integer(-1000), pdf.Integer(250), pdf.Integer(880),
			pdf.Integer(-1000), pdf.Integer(260), pdf.Integer(890),
		},
		pdf.Integer(5), pdf.Integer(6), pdf.Integer(-900), pdf.Integer(300), pdf.Integer(800),
	}
	got, err := decodeVMetrics(mock.Getter, a)
	if err != nil {
		t.Fatal(err)
	}
	want := map[cid.CID]dict.VMetrics{
		1: {DeltaY: -1000, OffsX: 250, OffsY: 880},
		2: {DeltaY: -1000, OffsX: 260, OffsY: 890},
		5: {DeltaY: -900, OffsX: 300, OffsY: 800},
		6: {DeltaY: -900, OffsX: 300, OffsY: 800},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("vmetrics mismatch (-want +got):\n%s", diff)
	}
}

// the maximal valid range covers all 65536 CIDs and must be accepted
func TestVMetricsFullRange(t *testing.T) {
	a := pdf.Array{
		pdf.Integer(0), pdf.Integer(65535),
		pdf.Integer(-1000), pdf.Integer(0), pdf.Integer(880),
	}
	got, err := decodeVMetrics(mock.Getter, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 65536 {
		t.Errorf("got %d entries, want 65536", len(got))
	}
}

// CIDs above 65535 and total entries beyond 65536 must be rejected, bounding
// both the map size and the loop work for crafted W2 arrays
func TestVMetricsReject(t *testing.T) {
	cases := map[string]pdf.Array{
		// range form, high absolute CID with a small span
		"range high CID": {
			pdf.Integer(70000), pdf.Integer(70100),
			pdf.Integer(-1000), pdf.Integer(0), pdf.Integer(880),
		},
		// range form, end just past the limit
		"range end past limit": {
			pdf.Integer(65535), pdf.Integer(65536),
			pdf.Integer(-1000), pdf.Integer(0), pdf.Integer(880),
		},
		// individual form crossing the limit mid-array
		"indiv crosses limit": {
			pdf.Integer(65535), pdf.Array{
				pdf.Integer(-1000), pdf.Integer(0), pdf.Integer(880),
				pdf.Integer(-1000), pdf.Integer(0), pdf.Integer(880),
			},
		},
		// disjoint maximal ranges (the memory-bomb shape)
		"disjoint ranges": {
			pdf.Integer(0), pdf.Integer(65535), pdf.Integer(-1000), pdf.Integer(0), pdf.Integer(880),
			pdf.Integer(65536), pdf.Integer(131071), pdf.Integer(-1000), pdf.Integer(0), pdf.Integer(880),
		},
		// repeated maximal range (the CPU-only shape)
		"repeated range": {
			pdf.Integer(0), pdf.Integer(65535), pdf.Integer(-1000), pdf.Integer(0), pdf.Integer(880),
			pdf.Integer(0), pdf.Integer(65535), pdf.Integer(-1000), pdf.Integer(0), pdf.Integer(880),
		},
	}
	for name, a := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeVMetrics(mock.Getter, a); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
