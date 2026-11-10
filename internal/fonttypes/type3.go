// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fonttypes

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

// Type3 is a Type 3 font.
var Type3 = func() font.Layouter {
	return font.Must(makefont.Type3())
}
