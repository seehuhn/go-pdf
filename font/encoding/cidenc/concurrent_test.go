// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cidenc

import (
	"sync"
	"testing"
	"time"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/internal/debug/race"
)

// identityEncoder returns an encoder for the predefined Identity-H CMap.
func identityEncoder(t *testing.T) CIDEncoder {
	t.Helper()
	enc, err := NewFromCMap(cmap.IdentityH, cmap.IdentityH.ROS, 500, VMetrics{})
	if err != nil {
		t.Fatal(err)
	}
	return enc
}

// TestConcurrentReaders checks that readers can use an encoder while codes
// are still being allocated: an edit session types into a font while a
// renderer draws the text already set in it.
func TestConcurrentReaders(t *testing.T) {
	if !race.Enabled {
		// Only the race detector can see the reads and writes overlap, so
		// without it the test would burn CPU without being able to fail.
		t.Skip("needs the race detector; run go test -race")
	}

	encoders := map[string]CIDEncoder{
		"utf8":     NewCompositeUtf8(nil, 500, VMetrics{}, font.Horizontal),
		"identity": identityEncoder(t),
	}
	for name, enc := range encoders {
		t.Run(name, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				for i := range cid.CID(200) {
					text := string(rune('A' + i%26))
					_, _ = enc.Encode(i+1, text, float64(i), VMetrics{})

					// Pace the writer.  Without this it allocates every code
					// well inside a single reader pass, and the readers spend
					// nearly all their time on an encoder which is already
					// final.
					time.Sleep(50 * time.Microsecond)
				}
			}()

			var wg sync.WaitGroup
			for range 4 {
				wg.Go(func() {
					s := make(pdf.String, 64)
					for i := range s {
						s[i] = byte(i)
					}
					for {
						select {
						case <-done:
							return
						default:
						}
						for range enc.Codes(s) {
						}
						for range enc.MappedCodes() {
						}
						_ = enc.Width(7)
						_, _ = enc.GetCode(3, "C")
						_ = enc.CodesRemaining()
						_ = enc.ToUnicode()
						_ = enc.CMap(&cid.SystemInfo{Registry: "Adobe", Ordering: "Identity"})
					}
				})
			}
			wg.Wait()
		})
	}
}

// TestEncodeRepeat checks that encoding a CID a second time yields the code
// it already has.
func TestEncodeRepeat(t *testing.T) {
	for name, enc := range map[string]CIDEncoder{
		"utf8":     NewCompositeUtf8(nil, 500, VMetrics{}, font.Horizontal),
		"identity": identityEncoder(t),
	} {
		t.Run(name, func(t *testing.T) {
			c1, err := enc.Encode(5, "A", 600, VMetrics{})
			if err != nil {
				t.Fatal(err)
			}
			c2, err := enc.Encode(5, "A", 600, VMetrics{})
			if err != nil {
				t.Fatal(err)
			}
			if c1 != c2 {
				t.Errorf("second Encode gave code %d, want %d", c2, c1)
			}
		})
	}
}

// TestEncodeConflict checks that an encoder backed by a fixed CMap, which can
// hold only one text, one width and one set of vertical metrics per CID,
// rejects a conflicting value instead of encoding one it cannot represent.  A
// rejected call must leave the stored value alone.
func TestEncodeConflict(t *testing.T) {
	enc, err := NewFromCMap(cmap.IdentityV, cmap.IdentityV.ROS, 500, VMetrics{})
	if err != nil {
		t.Fatal(err)
	}
	vm := VMetrics{OffsY: 800, DeltaY: -900}
	code, err := enc.Encode(5, "A", 600, vm)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := enc.Encode(5, "B", 600, vm); err == nil {
		t.Error("conflicting text was accepted")
	}
	if _, err := enc.Encode(5, "A", 700, vm); err == nil {
		t.Error("conflicting width was accepted")
	}
	if _, err := enc.Encode(5, "A", 600, VMetrics{OffsY: 800, DeltaY: -1000}); err == nil {
		t.Error("conflicting advance was accepted")
	}
	if _, err := enc.Encode(5, "A", 600, VMetrics{OffsY: 880, DeltaY: -900}); err == nil {
		t.Error("conflicting origin was accepted")
	}
	if c, err := enc.Encode(5, "A", 600, vm); err != nil || c != code {
		t.Errorf("repeated call gave (%d, %v), want (%d, nil)", c, err, code)
	}

	found := false
	for c, info := range enc.MappedCodes() {
		if c != code {
			continue
		}
		found = true
		if info.Text != "A" || info.Width != 600 || info.VMetrics != vm {
			t.Errorf("stored entry is (%q, %g, %v), want (\"A\", 600, %v)",
				info.Text, info.Width, info.VMetrics, vm)
		}
	}
	if !found {
		t.Errorf("no entry for code %d", code)
	}
}

// TestEncodeHorizontalVMetrics checks that vertical metrics, which are not
// used in horizontal writing mode, cannot make Encode fail there.
func TestEncodeHorizontalVMetrics(t *testing.T) {
	enc := identityEncoder(t)
	code, err := enc.Encode(5, "A", 600, VMetrics{OffsY: 800, DeltaY: -900})
	if err != nil {
		t.Fatal(err)
	}
	c, err := enc.Encode(5, "A", 600, VMetrics{OffsY: 880, DeltaY: -1000})
	if err != nil || c != code {
		t.Errorf("second call gave (%d, %v), want (%d, nil)", c, err, code)
	}
	if _, err := enc.Encode(0, "", 500, VMetrics{OffsY: 1, DeltaY: 2}); err != nil {
		t.Errorf("CID 0: %v", err)
	}
}

// TestEncodeNewText checks that an encoder which allocates its own codes gives
// a second text for the same CID a code of its own, where an encoder backed by
// a fixed CMap has to reject it (see [TestEncodeConflict]).
func TestEncodeNewText(t *testing.T) {
	enc := NewCompositeUtf8(nil, 500, VMetrics{}, font.Horizontal)
	c1, err := enc.Encode(5, "A", 600, VMetrics{})
	if err != nil {
		t.Fatal(err)
	}
	c2, err := enc.Encode(5, "B", 600, VMetrics{})
	if err != nil {
		t.Fatal(err)
	}
	if c1 == c2 {
		t.Errorf("both texts got code %d, want distinct codes", c1)
	}
}
