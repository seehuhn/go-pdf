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

package mapping

import (
	"errors"
	"io/fs"
	"testing"
)

func TestKnownOrderings(t *testing.T) {
	knownOrderings := []string{
		"CNS1",
		"GB1",
		"Japan1",
		"KR",
		"Korea1",
	}

	// to exercise the caching paths, we run the tests twice
	var cacheSize [2]int
	for i := range 2 {
		for _, ordering := range knownOrderings {
			mapping, err := GetCIDTextMapping("Adobe", ordering)
			if err != nil {
				t.Errorf("Failed to get mapping for %s: %v", ordering, err)
				continue
			}
			if len(mapping) == 0 {
				t.Errorf("Mapping for %s is empty", ordering)
			}
		}

		resourceMutex.Lock()
		cacheSize[i] = len(cache)
		resourceMutex.Unlock()
	}
	if cacheSize[0] != len(knownOrderings) {
		t.Errorf("Cache size mismatch: expected %d, got %d", len(knownOrderings), cacheSize[0])
	}
	if cacheSize[1] != len(knownOrderings) {
		t.Errorf("Cache size mismatch: expected %d, got %d", len(knownOrderings), cacheSize[1])
	}
}

func TestUnknownOrdering(t *testing.T) {
	m, err := GetCIDTextMapping("Test", "DoesNotExist")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Expected fs.ErrNotExist for unknown ordering, got %v", err)
	}
	if m != nil {
		t.Errorf("Expected nil mapping for unknown ordering, got %v", m)
	}
}

func TestGetTextToCIDMapping(t *testing.T) {
	forward, err := GetCIDTextMapping("Adobe", "Japan1")
	if err != nil {
		t.Fatal(err)
	}
	reverse, err := GetTextToCIDMapping("Adobe", "Japan1")
	if err != nil {
		t.Fatal(err)
	}

	// every reverse entry points at a CID whose forward text matches
	for text, c := range reverse {
		if forward[c] != text {
			t.Errorf("reverse[%q] = %d, but forward[%d] = %q", text, c, c, forward[c])
		}
	}
	// the reverse maps every text and always to the smallest matching CID
	for c, text := range forward {
		got, ok := reverse[text]
		if !ok {
			t.Errorf("text %q (CID %d) missing from reverse map", text, c)
			continue
		}
		if got > c {
			t.Errorf("reverse[%q] = %d, but smaller CID %d has the same text", text, got, c)
		}
	}
}

func TestGetTextToCIDMappingUnknown(t *testing.T) {
	m, err := GetTextToCIDMapping("Test", "DoesNotExist")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist for unknown ordering, got %v", err)
	}
	if m != nil {
		t.Errorf("expected nil mapping for unknown ordering, got %v", m)
	}
}
