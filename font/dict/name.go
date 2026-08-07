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

package dict

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/postscript/type1"
)

// validateFontName checks how a font dictionary names the font it describes:
// the PostScript name and the subset tag must together spell the /FontName
// entry of the descriptor, which must not be nil.
//
// The name must be one a font program can carry, since an embedded program
// names itself the same way, and since a name this library refused to write
// would be one it could not read back unchanged.  A name read from a file is
// repaired as it is read; one which reaches here unrepaired came from the
// caller, and is reported rather than quietly altered.
func validateFontName(desc *font.Descriptor, psName, subsetTag string) error {
	if psName == "" {
		return errors.New("missing PostScript name")
	}
	if subsetTag != "" && !subset.IsValidTag(subsetTag) {
		return fmt.Errorf("invalid subset tag: %s", subsetTag)
	}

	baseFont := subset.Join(subsetTag, psName)
	if err := type1.CheckFontName(baseFont); err != nil {
		return fmt.Errorf("invalid font name %q: %w", baseFont, err)
	}
	if desc.FontName != baseFont {
		return fmt.Errorf("font name mismatch: %s != %s",
			baseFont, desc.FontName)
	}
	return nil
}
