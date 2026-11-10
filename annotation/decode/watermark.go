// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodeWatermark(c pdf.Cursor, dict pdf.Dict) (*annotation.Watermark, error) {
	watermark := &annotation.Watermark{}

	if err := decodeCommon(c, &watermark.Common, dict); err != nil {
		return nil, err
	}

	// FixedPrint (optional)
	if fp, err := c.DecodeOptional(dict["FixedPrint"], annotation.ExtractFixedPrint); err != nil {
		return nil, err
	} else {
		watermark.FixedPrint = fp
	}

	return watermark, nil
}
