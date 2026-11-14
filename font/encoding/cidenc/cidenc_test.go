// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cidenc

import (
	"strings"
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
)

// TestToUnicodeLongText checks that the ToUnicode method of both encoder
// types reports a glyph text which is too long for a ToUnicode CMap.
func TestToUnicodeLongText(t *testing.T) {
	for _, n := range []int{256, 257} {
		fixedEnc, err := NewFromCMap(cmap.IdentityH, cmap.IdentityH.ROS, 500, VMetrics{})
		if err != nil {
			t.Fatal(err)
		}
		utf8Enc := NewCompositeUtf8(nil, 500, VMetrics{}, font.Horizontal)
		for _, enc := range []CIDEncoder{fixedEnc, utf8Enc} {
			if _, err := enc.Encode(1, strings.Repeat("A", n), 500, VMetrics{}); err != nil {
				t.Fatal(err)
			}
			tu, err := enc.ToUnicode()
			if n <= 256 && (err != nil || tu == nil) {
				t.Errorf("%T, n=%d: got %v, %v", enc, n, tu, err)
			} else if n > 256 && err == nil {
				t.Errorf("%T, n=%d: over-long text accepted", enc, n)
			}
		}
	}
}
