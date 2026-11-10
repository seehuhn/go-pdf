// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package acroform

import "seehuhn.de/go/pdf"

// shared array write helpers for the signature lock and seed value
// dictionaries. each writer omits the key for an empty slice.

func writeNameArray(dict pdf.Dict, key pdf.Name, vals []pdf.Name) {
	if len(vals) == 0 {
		return
	}
	arr := make(pdf.Array, len(vals))
	for i, v := range vals {
		arr[i] = v
	}
	dict[key] = arr
}

func writeTextStringArray(dict pdf.Dict, key pdf.Name, vals []pdf.TextString) {
	if len(vals) == 0 {
		return
	}
	arr := make(pdf.Array, len(vals))
	for i, s := range vals {
		arr[i] = s
	}
	dict[key] = arr
}

func writeASCIIStringArray(dict pdf.Dict, key pdf.Name, vals []string) {
	if len(vals) == 0 {
		return
	}
	arr := make(pdf.Array, len(vals))
	for i, s := range vals {
		arr[i] = pdf.String(s)
	}
	dict[key] = arr
}

func writeByteStringArray(dict pdf.Dict, key pdf.Name, vals [][]byte) {
	if len(vals) == 0 {
		return
	}
	arr := make(pdf.Array, len(vals))
	for i, b := range vals {
		arr[i] = pdf.String(b)
	}
	dict[key] = arr
}
