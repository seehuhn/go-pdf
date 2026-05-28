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
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/form"
)

func TestScreenIcon(t *testing.T) {
	s := NewStyle()
	icon := &form.Form{BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 32, URy: 32}}
	a := &annotation.Screen{
		Common: annotation.Common{Rect: mediaRect},
		MK:     &appearance.Characteristics{Icon: icon},
	}

	f := s.addScreenAppearance(a)

	if f.BBox != mediaRect {
		t.Errorf("BBox = %v, want %v", f.BBox, mediaRect)
	}
	if got := len(f.Res.XObject); got != 1 {
		t.Fatalf("XObject count = %d, want 1 (the icon)", got)
	}
	for _, x := range f.Res.XObject {
		if x != icon {
			t.Error("icon was copied, not reused (breaks resource dedup)")
		}
	}
}

func TestScreenPlaceholder(t *testing.T) {
	s := NewStyle()
	cases := map[string]*appearance.Characteristics{
		"nil MK":      nil,
		"MK, no icon": {},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			a := &annotation.Screen{
				Common: annotation.Common{Rect: mediaRect},
				MK:     mk,
			}
			f := s.addScreenAppearance(a)

			if f.BBox != mediaRect {
				t.Errorf("BBox = %v, want %v", f.BBox, mediaRect)
			}
			if len(f.Res.XObject) != 0 {
				t.Errorf("placeholder must not reference an XObject, got %d", len(f.Res.XObject))
			}
			if f.Content == nil {
				t.Error("placeholder content is empty")
			}
		})
	}
}

func TestScreenZeroRect(t *testing.T) {
	s := NewStyle()
	zero := pdf.Rectangle{LLx: 5, LLy: 5, URx: 5, URy: 5}
	a := &annotation.Screen{Common: annotation.Common{Rect: zero}}

	f := s.addScreenAppearance(a)

	if f.Content != nil {
		t.Error("zero-area Rect should produce empty content")
	}
}
