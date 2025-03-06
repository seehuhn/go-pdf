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

package dict

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestVMetricsRoundtrip(t *testing.T) {
	testCases := []struct {
		name    string
		metrics map[cid.CID]VMetrics
	}{
		{
			name:    "missing map",
			metrics: nil,
		},
		{
			name:    "empty map",
			metrics: map[cid.CID]VMetrics{},
		},
		{
			name: "single entry",
			metrics: map[cid.CID]VMetrics{
				42: {DeltaY: -1000, OffsX: 250, OffsY: 750},
			},
		},
		{
			name: "two consecutive CIDs with different metrics",
			metrics: map[cid.CID]VMetrics{
				20: {DeltaY: -900, OffsX: 400, OffsY: 800},
				21: {DeltaY: -900, OffsX: 400, OffsY: 0},
			},
		},
		{
			name: "two consecutive CIDs with same metrics",
			metrics: map[cid.CID]VMetrics{
				20: {DeltaY: -900, OffsX: 400, OffsY: 800},
				21: {DeltaY: -900, OffsX: 400, OffsY: 800},
			},
		},
		{
			name: "consecutive CIDs with different metrics",
			metrics: map[cid.CID]VMetrics{
				10: {DeltaY: -1000, OffsX: 250, OffsY: 750},
				11: {DeltaY: -1000, OffsX: 300, OffsY: 760},
				12: {DeltaY: -1000, OffsX: 350, OffsY: 770},
			},
		},
		{
			name: "consecutive CIDs with same metrics",
			metrics: map[cid.CID]VMetrics{
				20: {DeltaY: -900, OffsX: 400, OffsY: 800},
				21: {DeltaY: -900, OffsX: 400, OffsY: 800},
				22: {DeltaY: -900, OffsX: 400, OffsY: 800},
			},
		},
		{
			name: "non-consecutive CIDs",
			metrics: map[cid.CID]VMetrics{
				30: {DeltaY: -900, OffsX: 400, OffsY: 800},
				35: {DeltaY: -950, OffsX: 450, OffsY: 850},
				40: {DeltaY: -1000, OffsX: 500, OffsY: 900},
			},
		},
		{
			name: "mixed patterns",
			metrics: map[cid.CID]VMetrics{
				50: {DeltaY: -800, OffsX: 300, OffsY: 700},
				51: {DeltaY: -800, OffsX: 300, OffsY: 700},
				52: {DeltaY: -800, OffsX: 300, OffsY: 700},
				60: {DeltaY: -900, OffsX: 400, OffsY: 800},
				61: {DeltaY: -950, OffsX: 450, OffsY: 850},
				62: {DeltaY: -1000, OffsX: 500, OffsY: 900},
				70: {DeltaY: -1100, OffsX: 550, OffsY: 950},
				80: {DeltaY: -1100, OffsX: 550, OffsY: 950},
				81: {DeltaY: -1100, OffsX: 550, OffsY: 950},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			ref := buf.Alloc()

			// Encode
			encoded := encodeVMetrics(tc.metrics)
			buf.Put(ref, encoded)

			// Decode
			decoded, err := decodeVMetrics(buf, ref)
			if err != nil {
				t.Fatal(err)
			}

			// Compare original and decoded
			if d := cmp.Diff(tc.metrics, decoded); d != "" {
				t.Error(d)
			}
		})
	}
}
