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

package subset

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/internal/limits"
)

// A CID is a 32-bit value, so the length of the map must not be worked out in
// that type: the largest CID of all would wrap to a map of no entries, which
// the size check would take for a map small enough to build, and the CIDs it is
// meant to describe would then be written past its end.
func TestCIDToGIDLenAtTopOfRange(t *testing.T) {
	if got, want := cidToGIDLen(math.MaxUint32), int64(1)<<32; got != want {
		t.Errorf("the map for CIDs up to %d holds %d entries, want %d",
			uint32(math.MaxUint32), got, want)
	}
}

// Every CID the map describes must be inside it, whatever the largest one is.
func TestCIDToGIDLenHoldsEveryCID(t *testing.T) {
	for _, maxCID := range []cid.CID{0, 1, 0xFF, 0x100, 0xFFFF, 0x1_0000, math.MaxUint32} {
		if got := cidToGIDLen(maxCID); got <= int64(maxCID) {
			t.Errorf("the map for CIDs up to %d holds %d entries, too few to hold that CID",
				maxCID, got)
		}
	}
}

// The map holds an entry for every CID up to the largest one used, so a font
// with a widely spread CID range asks for far more entries than it has glyphs.
// Beyond the cap the map is refused rather than allocated: a file carrying it
// could not be read back, and a CID that large reaches no glyph in any case.
func TestMakeCIDToGIDRefusesHugeCID(t *testing.T) {
	for _, maxCID := range []cid.CID{limits.MaxCIDToGIDEntries, math.MaxUint32} {
		got, _, err := MakeCIDToGID([]cid.CID{0, maxCID})
		if err == nil {
			t.Errorf("a map for CIDs up to %d was built, with %d entries",
				maxCID, len(got))
		}
		if got != nil {
			t.Errorf("a map was returned alongside the error for CID %d", maxCID)
		}
	}
}

// The largest CID which still fits must be served, so that the cap turns away
// nothing a font can use: glyph indices are 16-bit, so every CID a TrueType
// font can resolve is inside the map.
func TestMakeCIDToGIDServesLargestCID(t *testing.T) {
	maxCID := cid.CID(limits.MaxCIDToGIDEntries - 1)

	got, _, err := MakeCIDToGID([]cid.CID{0, maxCID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != limits.MaxCIDToGIDEntries {
		t.Errorf("the map holds %d entries, want %d", len(got), limits.MaxCIDToGIDEntries)
	}
	if got[maxCID] != 1 {
		t.Errorf("CID %d maps to glyph %d, want 1", maxCID, got[maxCID])
	}
}

func TestMakeCIDToGID(t *testing.T) {
	for _, tc := range []struct {
		label      string
		cids       []cid.CID
		want       []glyph.ID
		isIdentity bool
	}{
		{
			// A font which uses no glyph at all needs no map: the identity
			// mapping describes it as well as anything.
			label:      "no glyphs",
			isIdentity: true,
		},
		{
			label:      "the CIDs are the glyph indices",
			cids:       []cid.CID{0, 1, 2, 3},
			want:       []glyph.ID{0, 1, 2, 3},
			isIdentity: true,
		},
		{
			// Subsetting closes the gaps, so a CID past the first gap no longer
			// names the glyph index it did.  The CIDs in between are covered by
			// no glyph and map to zero, which draws ".notdef".
			label: "gaps in the CIDs",
			cids:  []cid.CID{0, 3, 7},
			want:  []glyph.ID{0, 0, 0, 1, 0, 0, 0, 2},
		},
		{
			// The map covers CID 0 whether the font uses it or not, since it
			// starts there.
			label: "the first CID is not zero",
			cids:  []cid.CID{2, 5},
			want:  []glyph.ID{0, 0, 0, 0, 0, 1},
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			got, isIdentity, err := MakeCIDToGID(tc.cids)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("wrong map (-want +got):\n%s", diff)
			}
			if isIdentity != tc.isIdentity {
				t.Errorf("identity reported as %v, want %v", isIdentity, tc.isIdentity)
			}
			for i, gid := range got {
				if gid != 0 && tc.cids[gid] != cid.CID(i) {
					t.Errorf("CID %d maps to glyph %d, which belongs to CID %d",
						i, gid, tc.cids[gid])
				}
			}
		})
	}
}
