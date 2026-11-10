// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodeProjection(c pdf.Cursor, dict pdf.Dict) (*annotation.Projection, error) {
	projection := &annotation.Projection{}

	if err := decodeCommon(c, &projection.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &projection.Markup); err != nil {
		return nil, err
	}

	return projection, nil
}
