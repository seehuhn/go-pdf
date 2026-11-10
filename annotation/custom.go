// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package annotation

import (
	"errors"
	"maps"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.5.2

// Custom represents an annotation with an unknown or unsupported subtype.
type Custom struct {
	Common

	// Type is the subtype of the annotation.
	Type pdf.Name

	// Data contains the raw data of the annotation.
	// This must not include the /Type and /Subtype fields.
	// Any fields corresponding the entries in Common are ignored.
	Data pdf.Dict
}

var _ Annotation = (*Custom)(nil)

// AnnotationType returns the subtype of the custom annotation.
// This implements the [Annotation] interface.
func (c *Custom) AnnotationType() pdf.Name {
	return c.Type
}

func (c *Custom) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if c.Type == "" {
		return nil, errors.New("missing annotation subtype")
	}
	if _, present := c.Data["Type"]; present {
		return nil, errors.New("unexpected Type field")
	}
	if _, present := c.Data["Subtype"]; present {
		return nil, errors.New("unexpected Subtype field")
	}

	// Start with the raw data
	d := make(pdf.Dict)
	maps.Copy(d, c.Data)

	// Add/override common annotation fields
	d["Subtype"] = c.Type
	if err := c.Common.fillDict(rm, d, isMarkup(c), false); err != nil {
		return nil, err
	}

	return d, nil
}
