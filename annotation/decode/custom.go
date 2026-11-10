// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodeCustom(c pdf.Cursor, dict pdf.Dict) (*annotation.Custom, error) {
	subtype, err := c.Name(dict["Subtype"])
	if err != nil {
		return nil, err
	} else if subtype == "" {
		return nil, pdf.Error("missing annotation subtype")
	}

	cust := &annotation.Custom{
		Type: subtype,
		Data: dict,
	}

	if err := decodeCommon(c, &cust.Common, dict); err != nil {
		return nil, err
	}

	// Remove the entries for cust.Common from the Data dict.
	all := []pdf.Name{
		"Type",
		"Subtype",
		"Rect",
		"Contents",
		"P",
		"NM",
		"M",
		"F",
		"AP",
		"AS",
		"Border",
		"C",
		"StructParent",
		"OC",
		"AF",
		"ca",
		"CA",
		"BM",
		"Lang",
	}
	for _, key := range all {
		delete(cust.Data, key)
	}

	return cust, nil
}
