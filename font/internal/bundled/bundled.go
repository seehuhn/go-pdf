// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package bundled serves the font data bundled with this library to the
// packages which name it.  It reads each file from the font loader and hands
// out the parsed result.  A [Cache] builds each font instance at most once and
// hands out clones of it.
package bundled

import (
	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/parser"

	"seehuhn.de/go/pdf/font/loader"
)

// maxFontSize bounds the memory used when parsing a bundled font.
const maxFontSize = 1 << 26

// builtin locates the font files bundled with this library.
var builtin = loader.NewFontLoader()

// ReadAFM reads the metrics bundled under the given PostScript name.  They are
// returned as they stand in the file.
func ReadAFM(name string) (*afm.Metrics, error) {
	r, err := builtin.Open(name, loader.FontTypeAFM)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return afm.Read(r)
}

// ReadSfnt reads the TrueType or OpenType font bundled under the given
// PostScript name.  Each call parses the file afresh and returns a new
// [sfnt.Font].
func ReadSfnt(name string) (*sfnt.Font, error) {
	r, err := builtin.Open(name, loader.FontTypeSfnt)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return sfnt.Read(r, parser.NewBudget(maxFontSize))
}
