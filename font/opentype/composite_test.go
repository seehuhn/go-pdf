// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package opentype_test

import (
	"testing"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func TestCompositeRequiresCMap(t *testing.T) {
	for name, info := range map[string]func() *sfnt.Font{
		"cff":  makefont.OpenType,
		"glyf": makefont.TrueType,
	} {
		if _, err := opentype.NewComposite(info(), nil); err == nil {
			t.Errorf("%s: nil options accepted", name)
		}
		if _, err := opentype.NewComposite(info(), &opentype.OptionsComposite{}); err == nil {
			t.Errorf("%s: nil CMap accepted", name)
		}
	}
}
