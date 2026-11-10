// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package thumbnail_test

import (
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/image/thumbnail"
)

var _ graphics.Image = (*thumbnail.Thumbnail)(nil)
