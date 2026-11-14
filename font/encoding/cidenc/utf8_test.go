// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cidenc

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
)

func TestUTF8CS(t *testing.T) {
	var seen [4]bool
	cs := charcode.CodeSpaceRange{
		{Low: []byte{0}, High: []byte{0}},
		{Low: []byte{0, 0}, High: []byte{0, 0}},
		{Low: []byte{0, 0, 0}, High: []byte{0, 0, 0}},
		{Low: []byte{0, 0, 0, 0}, High: []byte{0, 0, 0, 0}},
	}
	for r := rune(0); r <= 0x10_FFFF; r++ {
		// avoid surrogates
		if 0xD800 <= r && r <= 0xDFFF {
			continue
		}

		b := []byte(string(r))
		k := len(b) - 1
		if !seen[k] {
			seen[k] = true
			for i := 0; i <= k; i++ {
				cs[k].Low[i] = b[i]
				cs[k].High[i] = b[i]
			}
		} else {
			for i := 0; i <= k; i++ {
				if b[i] < cs[k].Low[i] {
					cs[k].Low[i] = b[i]
				}
				if b[i] > cs[k].High[i] {
					cs[k].High[i] = b[i]
				}
			}
		}
	}
	if d := cmp.Diff(charcode.UTF8, cs); d != "" {
		t.Error(d)
	}
}

// TestUTF8VMetrics checks that the vertical metrics given to Encode are
// stored with the code, reported by MappedCodes, and turned into the
// vertical fields of font.Code by Codes in vertical writing mode only.  Unmapped codes
// carry the metrics of CID 0.
func TestUTF8VMetrics(t *testing.T) {
	cid0V := VMetrics{OffsY: 900, DeltaY: -1100}
	vm := VMetrics{OffsY: 750, DeltaY: -800}
	for _, tc := range []struct {
		wMode font.WritingMode
		want  [][3]float64 // VerticalAdvance, VerticalOriginX, VerticalOriginY
	}{
		{font.Horizontal, [][3]float64{{0, 0, 0}, {0, 0, 0}}},
		{font.Vertical, [][3]float64{{-0.8, 0.3, 0.75}, {-1.1, 0.25, 0.9}}},
	} {
		enc := NewCompositeUtf8(nil, 500, cid0V, tc.wMode)
		code, err := enc.Encode(7, "x", 600, vm)
		if err != nil {
			t.Fatal(err)
		}
		if c, err := enc.Encode(7, "x", 600, vm); err != nil || c != code {
			t.Errorf("repeated call gave (%d, %v), want (%d, nil)", c, err, code)
		}

		for c, info := range enc.MappedCodes() {
			if c == code && info.VMetrics != vm {
				t.Errorf("MappedCodes: got %v, want %v", info.VMetrics, vm)
			}
		}

		s := enc.Codec().AppendCode(nil, code)
		s = append(s, "y"...) // not allocated
		var got [][3]float64
		for c := range enc.Codes(pdf.String(s)) {
			got = append(got, [3]float64{c.VerticalAdvance, c.VerticalOriginX, c.VerticalOriginY})
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("mode %v: vertical metrics %v, want %v", tc.wMode, got, tc.want)
		}
	}
}

// TestUTF8EncodeConflict checks that the UTF-8 encoder, which gives a CID a
// new code for every text, still holds one width and one set of vertical
// metrics per CID, and rejects a conflicting value.  A rejected call must
// leave the encoder unchanged.
func TestUTF8EncodeConflict(t *testing.T) {
	cid0V := VMetrics{OffsY: 880, DeltaY: -1000}
	vm := VMetrics{OffsY: 800, DeltaY: -900}
	enc := NewCompositeUtf8(nil, 500, cid0V, font.Vertical)
	code, err := enc.Encode(5, "A", 600, vm)
	if err != nil {
		t.Fatal(err)
	}
	remaining := enc.CodesRemaining()

	if _, err := enc.Encode(5, "B", 700, vm); err == nil {
		t.Error("conflicting width was accepted")
	}
	if _, err := enc.Encode(5, "B", 600, VMetrics{OffsY: 800, DeltaY: -1000}); err == nil {
		t.Error("conflicting advance was accepted")
	}
	if _, err := enc.Encode(5, "A", 700, vm); err == nil {
		t.Error("conflicting width for a repeat was accepted")
	}
	if _, err := enc.Encode(0, "x", 600, cid0V); err == nil {
		t.Error("conflicting width for CID 0 was accepted")
	}
	if _, err := enc.Encode(0, "x", 500, vm); err == nil {
		t.Error("conflicting vertical metrics for CID 0 were accepted")
	}
	if got := enc.CodesRemaining(); got != remaining {
		t.Errorf("rejected calls allocated %d codes", remaining-got)
	}

	if c, err := enc.Encode(5, "A", 600, vm); err != nil || c != code {
		t.Errorf("repeated call gave (%d, %v), want (%d, nil)", c, err, code)
	}
	if _, err := enc.Encode(5, "B", 600, vm); err != nil {
		t.Errorf("second text with the same metrics: %v", err)
	}
	if _, err := enc.Encode(0, "x", 500, cid0V); err != nil {
		t.Errorf("CID 0 with its own metrics: %v", err)
	}
}

// TestUTF8EncodeHorizontalVMetrics checks that vertical metrics, which are
// not used in horizontal writing mode, cannot make Encode fail there.
func TestUTF8EncodeHorizontalVMetrics(t *testing.T) {
	enc := NewCompositeUtf8(nil, 500, VMetrics{}, font.Horizontal)
	if _, err := enc.Encode(5, "A", 600, VMetrics{OffsY: 800, DeltaY: -900}); err != nil {
		t.Fatal(err)
	}
	if _, err := enc.Encode(5, "B", 600, VMetrics{OffsY: 880, DeltaY: -1000}); err != nil {
		t.Errorf("second text: %v", err)
	}
	if _, err := enc.Encode(0, "x", 500, VMetrics{OffsY: 1, DeltaY: 2}); err != nil {
		t.Errorf("CID 0: %v", err)
	}
}
