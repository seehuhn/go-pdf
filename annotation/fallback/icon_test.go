// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

// TestIconKeepsFlags checks that generating the appearance of a text, file
// attachment or sound annotation leaves its flags as the caller set them,
// with or without NoZoom and NoRotate.
func TestIconKeepsFlags(t *testing.T) {
	rect := pdf.Rectangle{LLx: 100, LLy: 200, URx: 124, URy: 224}
	types := []struct {
		name string
		make func(annotation.Flags) annotation.Annotation
	}{
		{"Text", func(f annotation.Flags) annotation.Annotation {
			return &annotation.Text{Rect: rect, Flags: f}
		}},
		{"FileAttachment", func(f annotation.Flags) annotation.Annotation {
			return &annotation.FileAttachment{Rect: rect, Flags: f}
		}},
		{"Sound", func(f annotation.Flags) annotation.Annotation {
			return &annotation.Sound{Rect: rect, Flags: f}
		}},
	}
	for _, tp := range types {
		for _, flags := range []annotation.Flags{
			annotation.FlagPrint,
			annotation.FlagPrint | annotation.FlagNoZoom | annotation.FlagNoRotate,
		} {
			a := tp.make(flags)
			gen := newGen(t, pdf.V2_0)
			if err := gen.AddAppearance(a); err != nil {
				t.Fatal(err)
			}
			if got := a.GetCommon().Flags; got != flags {
				t.Errorf("%s: flags = %v, want %v", tp.name, got, flags)
			}
		}
	}
}
