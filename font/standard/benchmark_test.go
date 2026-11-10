// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package standard

import "testing"

// BenchmarkNewAll measures the time to make an instance of each of the 14
// standard fonts, once each and in the order of [All].
//
// The fonts are read before the loop starts, so this measures what a caller
// pays once that work is shared: the encoding state of a clone and nothing
// else.  Use [BenchmarkReadAll] for the cost of the reading itself.
func BenchmarkNewAll(b *testing.B) {
	for _, F := range All {
		if _, err := F.New(); err != nil {
			b.Fatal(err)
		}
	}
	for b.Loop() {
		for _, F := range All {
			if _, err := F.New(); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkReadAll measures the time to read all 14 standard fonts from the
// bundled font data, bypassing the sharing done by [Font.New].
func BenchmarkReadAll(b *testing.B) {
	for b.Loop() {
		for _, F := range All {
			if _, err := F.read(); err != nil {
				b.Fatal(err)
			}
		}
	}
}
