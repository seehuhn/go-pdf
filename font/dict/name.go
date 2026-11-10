// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
