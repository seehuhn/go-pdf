// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fonttypes

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
)

// Standard makes one of the 14 standard PDF fonts.
var Standard = func() font.Layouter {
	return font.Must(standard.Helvetica.New())
}
