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

package decode

import (
	"maps"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/trapnet"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

// TestTrapNetDecodeRepair verifies that decodeTrapNet repairs invalid
// field combinations to produce valid TrapNet objects.
func TestTrapNetDecodeRepair(t *testing.T) {
	x := pdf.NewExtractor(mock.Getter)

	t.Run("all absent", func(t *testing.T) {
		dict := pdf.Dict{
			"Subtype": pdf.Name("TrapNet"),
			"Rect":    &pdf.Rectangle{URx: 612, URy: 792},
		}
		tn, err := decodeTrapNet(pdf.CursorAt(x, nil), dict)
		if err != nil {
			t.Fatal(err)
		}
		if tn.LastModified.IsZero() {
			t.Error("expected LastModified to be set")
		}
		if len(tn.Version) != 0 {
			t.Error("expected Version to be empty")
		}
		if len(tn.AnnotStates) != 0 {
			t.Error("expected AnnotStates to be empty")
		}
	})

	t.Run("all present", func(t *testing.T) {
		dict := pdf.Dict{
			"Subtype":      pdf.Name("TrapNet"),
			"Rect":         &pdf.Rectangle{URx: 612, URy: 792},
			"LastModified": pdf.TextString("D:20231215103000Z"),
			"Version":      pdf.Array{pdf.NewReference(1, 0)},
			"AnnotStates":  pdf.Array{pdf.Name("N")},
		}
		tn, err := decodeTrapNet(pdf.CursorAt(x, nil), dict)
		if err != nil {
			t.Fatal(err)
		}
		if !tn.LastModified.IsZero() {
			t.Errorf("expected LastModified to be cleared, got %v", tn.LastModified)
		}
		if len(tn.Version) == 0 {
			t.Error("expected Version to be kept")
		}
		if len(tn.AnnotStates) == 0 {
			t.Error("expected AnnotStates to be kept")
		}
	})

	t.Run("LastModified and Version", func(t *testing.T) {
		dict := pdf.Dict{
			"Subtype":      pdf.Name("TrapNet"),
			"Rect":         &pdf.Rectangle{URx: 612, URy: 792},
			"LastModified": pdf.TextString("D:20231215103000Z"),
			"Version":      pdf.Array{pdf.NewReference(1, 0)},
		}
		tn, err := decodeTrapNet(pdf.CursorAt(x, nil), dict)
		if err != nil {
			t.Fatal(err)
		}
		if tn.LastModified.IsZero() {
			t.Error("expected LastModified to be kept")
		}
		if len(tn.Version) != 0 {
			t.Error("expected Version to be dropped")
		}
	})

	t.Run("LastModified and AnnotStates", func(t *testing.T) {
		dict := pdf.Dict{
			"Subtype":      pdf.Name("TrapNet"),
			"Rect":         &pdf.Rectangle{URx: 612, URy: 792},
			"LastModified": pdf.TextString("D:20231215103000Z"),
			"AnnotStates":  pdf.Array{pdf.Name("N")},
		}
		tn, err := decodeTrapNet(pdf.CursorAt(x, nil), dict)
		if err != nil {
			t.Fatal(err)
		}
		if tn.LastModified.IsZero() {
			t.Error("expected LastModified to be kept")
		}
		if len(tn.AnnotStates) != 0 {
			t.Error("expected AnnotStates to be dropped")
		}
	})

	t.Run("Version only", func(t *testing.T) {
		dict := pdf.Dict{
			"Subtype": pdf.Name("TrapNet"),
			"Rect":    &pdf.Rectangle{URx: 612, URy: 792},
			"Version": pdf.Array{pdf.NewReference(1, 0)},
		}
		tn, err := decodeTrapNet(pdf.CursorAt(x, nil), dict)
		if err != nil {
			t.Fatal(err)
		}
		if tn.LastModified.IsZero() {
			t.Error("expected LastModified to be set")
		}
		if len(tn.Version) != 0 {
			t.Error("expected Version to be dropped")
		}
	})

	t.Run("AnnotStates only", func(t *testing.T) {
		dict := pdf.Dict{
			"Subtype":     pdf.Name("TrapNet"),
			"Rect":        &pdf.Rectangle{URx: 612, URy: 792},
			"AnnotStates": pdf.Array{pdf.Name("N")},
		}
		tn, err := decodeTrapNet(pdf.CursorAt(x, nil), dict)
		if err != nil {
			t.Fatal(err)
		}
		if tn.LastModified.IsZero() {
			t.Error("expected LastModified to be set")
		}
		if len(tn.AnnotStates) != 0 {
			t.Error("expected AnnotStates to be dropped")
		}
	})
}

// writeFormStream writes a bare form XObject and returns its reference.
func writeFormStream(t *testing.T, w *pdf.Writer, extra pdf.Dict) pdf.Reference {
	t.Helper()
	ref := w.Alloc()
	dict := pdf.Dict{
		"Subtype":   pdf.Name("Form"),
		"BBox":      &pdf.Rectangle{URx: 24, URy: 24},
		"Resources": pdf.Dict{},
	}
	maps.Copy(dict, extra)
	stm, err := w.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	return ref
}

// TestTrapNetFlagRepair checks that the Print and ReadOnly flags are forced on
// read, so that a trap network annotation stays writable.
func TestTrapNetFlagRepair(t *testing.T) {
	x := pdf.NewExtractor(mock.Getter)
	want := annotation.FlagPrint | annotation.FlagReadOnly

	for _, flags := range []pdf.Integer{0, 6, 1, 0xFFFF} {
		dict := pdf.Dict{
			"Subtype":      pdf.Name("TrapNet"),
			"Rect":         &pdf.Rectangle{URx: 612, URy: 792},
			"LastModified": pdf.TextString("D:20231215103000Z"),
			"F":            flags,
		}
		tn, err := decodeTrapNet(pdf.CursorAt(x, nil), dict)
		if err != nil {
			t.Fatal(err)
		}
		if tn.Flags != want {
			t.Errorf("F=%d: expected flags %d, got %d", flags, want, tn.Flags)
		}
	}
}

// TestPrinterMarkFlagRepair checks the same for printer's mark annotations.
func TestPrinterMarkFlagRepair(t *testing.T) {
	x := pdf.NewExtractor(mock.Getter)
	want := annotation.FlagPrint | annotation.FlagReadOnly

	for _, flags := range []pdf.Integer{0, 6, 1, 0xFFFF} {
		dict := pdf.Dict{
			"Subtype": pdf.Name("PrinterMark"),
			"Rect":    &pdf.Rectangle{URx: 612, URy: 792},
			"F":       flags,
		}
		pm, err := decodePrinterMark(pdf.CursorAt(x, nil), dict)
		if err != nil {
			t.Fatal(err)
		}
		if pm.Flags != want {
			t.Errorf("F=%d: expected flags %d, got %d", flags, want, pm.Flags)
		}
	}
}

// TestTrapNetAppearanceRepair checks that a normal appearance without the trap
// network entries is repaired, so that the annotation can be written back.
func TestTrapNetAppearanceRepair(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	formRef := writeFormStream(t, w, nil)

	dict := pdf.Dict{
		"Subtype":      pdf.Name("TrapNet"),
		"Rect":         &pdf.Rectangle{URx: 612, URy: 792},
		"LastModified": pdf.TextString("D:20231215103000Z"),
		"AP":           pdf.Dict{"N": formRef},
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	tn, err := decodeTrapNet(pdf.CursorAt(x, nil), dict)
	if err != nil {
		t.Fatal(err)
	}

	if tn.Appearance == nil || tn.Appearance.Normal == nil {
		t.Fatal("expected a normal appearance")
	}
	if tn.Appearance.Normal.TrapNet == nil {
		t.Fatal("expected the normal appearance to carry trap network entries")
	}
	if got := tn.Appearance.Normal.TrapNet.PCM; got != trapnet.DefaultPCM {
		t.Errorf("expected PCM %q, got %q", trapnet.DefaultPCM, got)
	}

	// the repaired annotation must be writable
	out, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(out)
	if _, err := tn.Encode(rm); err != nil {
		t.Fatalf("cannot write the annotation back: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestTrapNetAppearanceRepairIsLocal checks that repairing the normal
// appearance does not modify forms which are shared with other appearances.
// Forms reached through an appearance sub-dictionary are cached by the
// extractor, so an in-place fix would leak into every other holder.
func TestTrapNetAppearanceRepairIsLocal(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	formRef := writeFormStream(t, w, nil)

	// /N and /D name the very same form object
	dict := pdf.Dict{
		"Subtype":      pdf.Name("TrapNet"),
		"Rect":         &pdf.Rectangle{URx: 612, URy: 792},
		"LastModified": pdf.TextString("D:20231215103000Z"),
		"AP": pdf.Dict{
			"N": pdf.Dict{"On": formRef},
			"D": pdf.Dict{"On": formRef},
		},
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	tn, err := decodeTrapNet(pdf.CursorAt(x, nil), dict)
	if err != nil {
		t.Fatal(err)
	}

	normal := tn.Appearance.NormalMap["On"]
	down := tn.Appearance.DownMap["On"]
	if normal == nil || down == nil {
		t.Fatal("expected both appearance states")
	}
	if normal.TrapNet == nil {
		t.Error("expected the normal appearance to be repaired")
	}
	if down.TrapNet != nil {
		t.Error("the down appearance was modified by the repair")
	}
}

// TestTrapNetAppearanceKeepsPrinterMark checks that supplying the missing trap
// network entries leaves the printer's mark entries alone.  The form may be
// shared with a printer's mark annotation, and dropping them here would
// discard data the file legitimately carries.
func TestTrapNetAppearanceKeepsPrinterMark(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	formRef := writeFormStream(t, w, pdf.Dict{"MarkStyle": pdf.TextString("Colour bar")})

	dict := pdf.Dict{
		"Subtype":      pdf.Name("TrapNet"),
		"Rect":         &pdf.Rectangle{URx: 612, URy: 792},
		"LastModified": pdf.TextString("D:20231215103000Z"),
		"AP":           pdf.Dict{"N": formRef},
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	tn, err := decodeTrapNet(pdf.CursorAt(x, nil), dict)
	if err != nil {
		t.Fatal(err)
	}

	normal := tn.Appearance.Normal
	if normal == nil {
		t.Fatal("expected a normal appearance")
	}
	if normal.PrinterMark == nil {
		t.Error("expected the printer's mark entries to be kept")
	}
	if normal.TrapNet == nil {
		t.Error("expected the normal appearance to carry trap network entries")
	}

	// the repaired annotation must be writable
	out, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(out)
	if _, err := tn.Encode(rm); err != nil {
		t.Fatalf("cannot write the annotation back: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
}
