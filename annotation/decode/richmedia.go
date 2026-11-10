// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodeRichMedia(c pdf.Cursor, dict pdf.Dict) (*annotation.RichMedia, error) {
	richMedia := &annotation.RichMedia{}

	if err := decodeCommon(c, &richMedia.Common, dict); err != nil {
		return nil, err
	}

	// RichMediaContent (required)
	if content, ok := dict["RichMediaContent"].(pdf.Reference); ok {
		richMedia.RichMediaContent = content
	}

	// RichMediaSettings (optional)
	if settings, ok := dict["RichMediaSettings"].(pdf.Reference); ok {
		richMedia.RichMediaSettings = settings
	}

	return richMedia, nil
}
