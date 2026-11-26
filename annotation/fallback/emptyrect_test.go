// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// emptyRectCases lists annotation types of several kinds, built for a given
// rectangle.
var emptyRectCases = []struct {
	name string
	make func(r pdf.Rectangle) annotation.Annotation
}{
	{name: "text field", make: func(r pdf.Rectangle) annotation.Annotation {
		f := acroform.NewTextField("t")
		f.DefaultAppearance = "/Helv 0 Tf 0 g"
		f.Text = &pdf.StringOrStream{Value: pdf.Text("abc")}
		return annotation.AddWidget(f, r)
	}},
	{name: "push button", make: func(r pdf.Rectangle) annotation.Annotation {
		f := acroform.NewPushButtonField("b")
		return annotation.AddWidget(f, r)
	}},
	{name: "check box", make: func(r pdf.Rectangle) annotation.Annotation {
		f := acroform.NewButtonField("c", acroform.ButtonCheckbox)
		return annotation.AddWidget(f, r)
	}},
	{name: "stamp", make: func(r pdf.Rectangle) annotation.Annotation {
		return &annotation.Stamp{Rect: r}
	}},
	{name: "square", make: func(r pdf.Rectangle) annotation.Annotation {
		return &annotation.Square{Rect: r}
	}},
	{name: "link", make: func(r pdf.Rectangle) annotation.Annotation {
		return &annotation.Link{Rect: r}
	}},
	{name: "redact", make: func(r pdf.Rectangle) annotation.Annotation {
		return &annotation.Redact{Rect: r}
	}},
}

// An annotation whose rectangle has zero area gets no appearance, one with a
// visible area gets one, and the result can always be embedded.
func TestEmptyRect(t *testing.T) {
	rects := []struct {
		name    string
		r       pdf.Rectangle
		visible bool
	}{
		{"zero", pdf.Rectangle{}, false},
		{"zero height", pdf.Rectangle{LLx: 10, LLy: 10, URx: 60, URy: 10}, false},
		{"normal", pdf.Rectangle{LLx: 10, LLy: 10, URx: 110, URy: 40}, true},
	}
	for _, c := range emptyRectCases {
		for _, rc := range rects {
			t.Run(c.name+"/"+rc.name, func(t *testing.T) {
				a := c.make(rc.r)
				g := newGen(t, pdf.V2_0)
				if err := g.AddAppearance(a); err != nil {
					t.Fatal(err)
				}
				ap := a.GetCommon().Appearance
				var forms []*form.Form
				if ap != nil {
					if ap.Normal != nil {
						forms = append(forms, ap.Normal)
					}
					for _, f := range ap.NormalMap {
						forms = append(forms, f)
					}
				}
				has := len(forms) > 0
				if has != rc.visible {
					t.Errorf("has appearance = %t, want %t", has, rc.visible)
				}

				w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
				rm := pdf.NewResourceManager(w)
				for _, f := range forms {
					if _, err := rm.Embed(f); err != nil {
						t.Fatal(err)
					}
				}
				if err := rm.Close(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// An annotation with an empty rectangle keeps the appearance it already has.
func TestEmptyRectKeepsAppearance(t *testing.T) {
	f := &form.Form{BBox: pdf.Rectangle{URx: 10, URy: 10}}
	for _, c := range emptyRectCases {
		t.Run(c.name, func(t *testing.T) {
			a := c.make(pdf.Rectangle{LLx: 10, LLy: 10, URx: 60, URy: 10})
			ap := &appearance.Dict{}
			ap.SetNormal(f, nil)
			a.GetCommon().Appearance = ap

			g := newGen(t, pdf.V2_0)
			if err := g.AddAppearance(a); err != nil {
				t.Fatal(err)
			}
			got := a.GetCommon().Appearance
			if got != ap || got.Normal != f {
				t.Errorf("existing appearance was changed")
			}
		})
	}
}
