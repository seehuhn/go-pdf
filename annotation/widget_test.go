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

package annotation

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// a widget whose Field back-reference disagrees with the field's Widgets slice
// is rejected by Encode.
func TestWidgetFieldConsistency(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	f := acroform.NewTextField("f0")
	f.DefaultAppearance = "/Helv 0 Tf 0 g"
	wid := AddWidget(f, pdf.Rectangle{URx: 10, URy: 10})

	// break the link: the field no longer lists this widget
	f.GetCommon().Widgets = nil

	if _, err := wid.Encode(rm); err == nil {
		t.Error("expected an error when the field does not list the widget")
	}
}

// a form widget reserves its reference when its page is written and the form
// fills it in at Close. If the form is never encoded, the reservation dangles
// and Close reports it.
func TestWidgetReservation(t *testing.T) {
	// build reserves a widget's reference (as a page write would) and returns
	// the resource manager and the form that owns the widget.
	build := func() (*pdf.ResourceManager, *acroform.InteractiveForm) {
		// V1_7: a widget without an appearance stream is valid (PDF 2.0 would
		// require /AP, which is irrelevant to what this test exercises)
		w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
		rm := pdf.NewResourceManager(w)
		f := acroform.NewTextField("f0")
		f.DefaultAppearance = "/Helv 0 Tf 0 g"
		wid := AddWidget(f, pdf.Rectangle{URx: 10, URy: 10})
		if _, err := rm.Store(wid); err != nil {
			t.Fatal(err)
		}
		return rm, &acroform.InteractiveForm{Fields: []acroform.Node{f}}
	}

	t.Run("filled by form", func(t *testing.T) {
		rm, form := build()
		rm.StoreDeferred(form)
		if err := rm.Close(); err != nil {
			t.Errorf("Close failed though the form was encoded: %v", err)
		}
	})

	t.Run("dangling without form", func(t *testing.T) {
		rm, _ := build()
		// the form is never encoded, so the widget reservation is never filled
		if err := rm.Close(); err == nil {
			t.Error("expected Close to report the unfilled widget reservation")
		}
	})
}

// a check box or radio button widget's appearance state must name one of its
// normal appearances and agree with the field value
func TestWidgetAppearanceStateConsistency(t *testing.T) {
	blank := &form.Form{BBox: pdf.Rectangle{URx: 10, URy: 10}, Res: &content.Resources{}}
	tests := []struct {
		name    string
		flags   acroform.FieldFlags
		value   pdf.Name
		state   pdf.Name
		wantErr bool
	}{
		{"on and selected", 0, "Yes", "Yes", false},
		{"off while selected", 0, "Yes", "Off", false},
		{"off while unselected", 0, "Off", "Off", false},
		{"off with no value", 0, "", "Off", false},
		{"unknown state", 0, "Yes", "Maybe", true},
		{"on while unselected", 0, "Off", "Yes", true},
		{"on with no value", 0, "", "Yes", true},
		{"radio on other value", acroform.FieldRadio, "A", "Yes", true},
		{"push button unchecked", acroform.FieldPushbutton, "", "Yes", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)

			f := acroform.NewButtonField("b")
			f.Flags = tc.flags
			f.V = tc.value
			wid := AddWidget(f, pdf.Rectangle{URx: 10, URy: 10})
			wid.Appearance = &appearance.Dict{
				NormalMap: map[pdf.Name]*form.Form{"Yes": blank, "Off": blank},
				SingleUse: true,
			}
			wid.AppearanceState = tc.state

			_, err := wid.encodeOwnEntries(rm)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// the off state needs no appearance stream, while any other state does
func TestWidgetOffStateWithoutAppearance(t *testing.T) {
	blank := &form.Form{BBox: pdf.Rectangle{URx: 10, URy: 10}, Res: &content.Resources{}}
	for _, tc := range []struct {
		state   pdf.Name
		wantErr bool
	}{
		{"Off", false},
		{"Maybe", true},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)

			f := acroform.NewButtonField("b")
			f.V = "Off"
			wid := AddWidget(f, pdf.Rectangle{URx: 10, URy: 10})
			wid.Appearance = &appearance.Dict{
				NormalMap: map[pdf.Name]*form.Form{"Yes": blank},
				SingleUse: true,
			}
			wid.AppearanceState = tc.state

			_, err := wid.encodeOwnEntries(rm)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// a widget listed by one field but pointing at another is rejected when the
// form is encoded
func TestWidgetParentConsistency(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	a := acroform.NewButtonField("a")
	b := acroform.NewButtonField("b")
	wid := AddWidget(b, pdf.Rectangle{URx: 10, URy: 10})
	// move the widget into a's list while it still points at b
	b.Widgets = nil
	a.Widgets = append(a.Widgets, wid)

	form := &acroform.InteractiveForm{Fields: []acroform.Node{a, b}}
	if _, err := form.Encode(rm); err == nil {
		t.Error("expected an error when a widget's field is not the field listing it")
	}
}

// a form widget which was never added to a page has no reference when the
// form is encoded, and the form refuses to write it
func TestWidgetMustBePlaced(t *testing.T) {
	build := func(place bool) (*pdf.ResourceManager, *acroform.InteractiveForm) {
		w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
		rm := pdf.NewResourceManager(w)
		f := acroform.NewTextField("f0")
		f.DefaultAppearance = "/Helv 0 Tf 0 g"
		wid := AddWidget(f, pdf.Rectangle{URx: 10, URy: 10})
		if place {
			// a page write stores the widget, reserving its reference
			if _, err := rm.Store(wid); err != nil {
				t.Fatal(err)
			}
		}
		return rm, &acroform.InteractiveForm{Fields: []acroform.Node{f}}
	}

	t.Run("placed", func(t *testing.T) {
		rm, form := build(true)
		if _, err := form.Encode(rm); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("unplaced", func(t *testing.T) {
		rm, form := build(false)
		if _, err := form.Encode(rm); err == nil {
			t.Error("expected an error for a widget on no page")
		}
	})
}
