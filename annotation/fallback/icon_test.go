// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
