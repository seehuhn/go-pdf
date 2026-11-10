// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package measure

import (
	"seehuhn.de/go/pdf"
)

// Measure represents a measurement coordinate system.
// This is implemented by [RectilinearMeasure] and [GeospatialMeasure].
type Measure interface {
	pdf.Embedder
	MeasureType() pdf.Name // Returns "RL" or "GEO"
}
