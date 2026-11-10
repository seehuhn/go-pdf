// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package makefont

import (
	"bytes"
	_ "embed"

	"seehuhn.de/go/postscript/type1"
)

//go:embed testdata/mm.pfa
var mmType1Data []byte

// MMType1 returns a multiple master Type 1 font for use in tests.
// The font has two design axes, "Weight" and "Width".
//
// The testdata fixture mm.pfa was generated deterministically by go-postscript's
// internal MM font builder (go-postscript/internal/debug.MakeMMFont).
// To regenerate the fixture, run a small program in the go-postscript module that
// calls debug.MakeMMFont and writes its output to this testdata path.
// The fixture is a committed exception to the no-committed-fonts rule because Go
// forbids cross-module internal imports and type1.Instantiate needs data that only
// type1.Read can populate.
func MMType1() *type1.Font {
	psFont, err := type1.Read(bytes.NewReader(mmType1Data))
	if err != nil {
		panic(err)
	}
	return psFont
}
