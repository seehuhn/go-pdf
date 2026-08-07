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
	"fmt"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/internal/limits"
)

// MakeCIDToGID builds the CIDToGIDMap for a subset which holds the glyphs of
// the given CIDs and nothing else.  Subsetting places the glyph for cids[i] at
// glyph index i, so that index is what each CID maps to.
//
// The CIDs must be sorted in increasing order and must not repeat, which is
// how [Tag] expects the glyphs of a subset to be arranged.  Any other slice is
// invalid input, and this function may panic on it.
//
// The second return value reports whether the result is the identity mapping.
// A CIDFontType2 dictionary leaves its CIDToGID field nil in that case, and
// the map is omitted from the file.
//
// The result is the map the file carries, which holds an entry for every CID
// from zero up to the largest one used.  Its size therefore follows the largest
// CID rather than the number of glyphs: a font whose CIDs are few but widely
// spread needs a correspondingly large map.  A font which needs more entries
// than [limits.MaxCIDToGIDEntries] is rejected rather than allocated for, since
// the file could not be read back and a CID that large reaches no glyph in any
// case.
func MakeCIDToGID(cids []cid.CID) ([]glyph.ID, bool, error) {
	if len(cids) == 0 {
		return nil, true, nil
	}

	maxCID := cids[len(cids)-1]
	size := cidToGIDLen(maxCID)
	if size > limits.MaxCIDToGIDEntries {
		return nil, false, fmt.Errorf("CID %d too large for CIDToGIDMap", maxCID)
	}

	cidToGID := make([]glyph.ID, size)
	isIdentity := true
	for subsetGID, cidVal := range cids {
		if cidVal != cid.CID(subsetGID) {
			isIdentity = false
		}
		cidToGID[cidVal] = glyph.ID(subsetGID)
	}
	return cidToGID, isIdentity, nil
}

// cidToGIDLen returns the number of entries a CIDToGIDMap needs in order to
// describe every CID up to maxCID.
//
// The arithmetic is done in int64 rather than in [cid.CID], which is a 32-bit
// type: the largest CID of all would wrap to a length of zero, and a caller
// comparing that against a cap would take it for a map small enough to build.
func cidToGIDLen(maxCID cid.CID) int64 {
	return int64(maxCID) + 1
}
