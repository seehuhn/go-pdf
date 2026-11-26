// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package daparse reads the text size and colour from the default-appearance
// string of an annotation or form field.
package daparse

import (
	"io"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
)

// Parse extracts the text size and colour from a default-appearance string.
// Unrecognised input yields a zero size (auto) and the colour def.
//
// The font name is deliberately ignored: it names an entry in the form's
// default resources, which is a font dictionary in the file rather than
// something the caller can lay text out with.
//
// The string is tokenised as a content-stream fragment.  As in a content
// stream, operands are taken from the start of an operator's operand list,
// surplus operands are ignored, and colour components are clipped to [0, 1].
func Parse(da string, def color.Color) (size float64, col color.Color) {
	col = def
	stream := content.NewScanner(func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(da)), nil
	})
	for op, args := range stream.NewIter().All() {
		switch op {
		case content.OpTextSetFont:
			if len(args) < 2 {
				continue
			}
			_, isName := args[0].(pdf.Name)
			if v, ok := number(args, 1); isName && ok && v >= 0 {
				size = v
			}
		case content.OpSetFillGray:
			if g, ok := number(args, 0); ok {
				col = color.DeviceGray(clip01(g))
			}
		case content.OpSetFillRGB:
			r, ok1 := number(args, 0)
			g, ok2 := number(args, 1)
			b, ok3 := number(args, 2)
			if ok1 && ok2 && ok3 {
				col = color.DeviceRGB{clip01(r), clip01(g), clip01(b)}
			}
		case content.OpSetFillCMYK:
			c, ok1 := number(args, 0)
			m, ok2 := number(args, 1)
			y, ok3 := number(args, 2)
			k, ok4 := number(args, 3)
			if ok1 && ok2 && ok3 && ok4 {
				col = color.DeviceCMYK{clip01(c), clip01(m), clip01(y), clip01(k)}
			}
		}
	}
	return size, col
}

// number returns operand idx of a default-appearance operator as a number.
// The operands come from the content-stream scanner, which gives numbers as
// [pdf.Integer] or [pdf.Real].
func number(args []pdf.Object, idx int) (float64, bool) {
	if idx >= len(args) {
		return 0, false
	}
	switch v := args[idx].(type) {
	case pdf.Integer:
		return float64(v), true
	case pdf.Real:
		return float64(v), true
	}
	return 0, false
}

func clip01(x float64) float64 {
	return color.ClipComponent(x, 0, 1)
}
