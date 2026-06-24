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
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// a widget whose Field back-reference disagrees with the field's Widgets slice
// is rejected by Encode.
func TestWidgetFieldConsistency(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	f := acroform.NewTextField("f0")
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
		w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		rm := pdf.NewResourceManager(w)
		f := acroform.NewTextField("f0")
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
