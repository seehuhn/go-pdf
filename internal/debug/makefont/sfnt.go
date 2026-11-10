// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package makefont

import (
	"bytes"

	"golang.org/x/image/font/gofont/goregular"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/parser"
)

// TrueType returns a font with glyf outlines.
func TrueType() *sfnt.Font {
	r := bytes.NewReader(goregular.TTF)
	info, err := sfnt.Read(r, parser.NewBudget(int64(len(goregular.TTF))))
	if err != nil {
		panic(err)
	}
	return info
}

// OpenType returns a font with CFF outlines, not using CIDFont operators.
func OpenType() *sfnt.Font {
	info := TrueType()
	info, err := toCFF(info)
	if err != nil {
		panic(err)
	}
	return info
}

// OpenTypeCID returns a font with CFF outlines and CIDFont operators.
func OpenTypeCID() *sfnt.Font {
	info := OpenType()
	info, err := toCFFCID(info)
	if err != nil {
		panic(err)
	}
	return info
}

// OpenTypeCID2 returns a font with CFF outlines, CIDFont operators, and
// multiple private dictionaries.
func OpenTypeCID2() *sfnt.Font {
	info := OpenType()
	info, err := toCFFCID2(info)
	if err != nil {
		panic(err)
	}
	return info
}
