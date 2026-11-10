// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package annotation

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestFreeTextRejectsInvalidAlign(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	a := &FreeText{
		Rect:              pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
		DefaultAppearance: "/Helv 12 Tf 0 g",
		Align:             pdf.TextAlign(99),
	}

	if _, err := a.Encode(rm); err == nil {
		t.Error("expected error for out-of-range Align, got nil")
	}
}

func TestRedactRejectsInvalidAlign(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	a := &Redact{
		Rect:  pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
		Align: pdf.TextAlign(99),
	}

	if _, err := a.Encode(rm); err == nil {
		t.Error("expected error for out-of-range Align, got nil")
	}
}
