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
	f, err := cmap.Predefined("Identity-H")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := NewFromCMap(f, 500)
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
		"utf8":     NewCompositeUtf8(500, font.Horizontal),
		"identity": identityEncoder(t),
	}
	for name, enc := range encoders {
		t.Run(name, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				for i := range cid.CID(200) {
					text := string(rune('A' + i%26))
					_, _ = enc.Encode(i+1, text, float64(i))

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
		"utf8":     NewCompositeUtf8(500, font.Horizontal),
		"identity": identityEncoder(t),
	} {
		t.Run(name, func(t *testing.T) {
			c1, err := enc.Encode(5, "A", 600)
			if err != nil {
				t.Fatal(err)
			}
			c2, err := enc.Encode(5, "A", 600)
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
// hold only one text and one width per CID, rejects a conflicting value
// instead of encoding one it cannot represent.  A rejected call must leave the
// stored value alone.
func TestEncodeConflict(t *testing.T) {
	enc := identityEncoder(t)
	code, err := enc.Encode(5, "A", 600)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := enc.Encode(5, "B", 600); err == nil {
		t.Error("conflicting text was accepted")
	}
	if _, err := enc.Encode(5, "A", 700); err == nil {
		t.Error("conflicting width was accepted")
	}

	found := false
	for c, info := range enc.MappedCodes() {
		if c != code {
			continue
		}
		found = true
		if info.Text != "A" || info.Width != 600 {
			t.Errorf("stored entry is (%q, %g), want (\"A\", 600)", info.Text, info.Width)
		}
	}
	if !found {
		t.Errorf("no entry for code %d", code)
	}
}

// TestEncodeNewText checks that an encoder which allocates its own codes gives
// a second text for the same CID a code of its own, where an encoder backed by
// a fixed CMap has to reject it (see [TestEncodeConflict]).
func TestEncodeNewText(t *testing.T) {
	enc := NewCompositeUtf8(500, font.Horizontal)
	c1, err := enc.Encode(5, "A", 600)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := enc.Encode(5, "B", 600)
	if err != nil {
		t.Fatal(err)
	}
	if c1 == c2 {
		t.Errorf("both texts got code %d, want distinct codes", c1)
	}
}
