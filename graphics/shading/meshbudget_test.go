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

package shading

import (
	"errors"
	"testing"

	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/limits"
)

// meshStreamData embeds a mesh shading and returns its decoded stream bytes,
// the raw vertex/patch data that the parsers consume.
func meshStreamData(t *testing.T, sh graphics.Shading) []byte {
	t.Helper()
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)
	embedded, err := rm.Embed(sh)
	if err != nil {
		t.Fatal(err)
	}
	if err := buf.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := pdf.CursorAt(pdf.NewExtractor(buf), nil).ReadAll(embedded, limits.MaxShadingBytes)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestShadingBudgetRejection checks that each mesh parser fails when the
// budget cannot cover the vertex/patch storage, and succeeds with a budget
// sized as in production.
func TestShadingBudgetRejection(t *testing.T) {
	const tiny = 64

	t.Run("type4", func(t *testing.T) {
		s := testCases[4][0].shading.(*Type4)
		data := meshStreamData(t, s)
		if _, err := parseType4Vertices(data, s, membudget.New(tiny)); !errors.Is(err, membudget.ErrExceeded) {
			t.Errorf("expected ErrExceeded, got %v", err)
		}
		if _, err := parseType4Vertices(data, s, membudget.New(limits.ShadingBudget(int64(len(data))))); err != nil {
			t.Errorf("unexpected error with full budget: %v", err)
		}
	})

	t.Run("type5", func(t *testing.T) {
		s := testCases[5][0].shading.(*Type5)
		data := meshStreamData(t, s)
		if _, err := parseType5Vertices(data, s, membudget.New(tiny)); !errors.Is(err, membudget.ErrExceeded) {
			t.Errorf("expected ErrExceeded, got %v", err)
		}
		if _, err := parseType5Vertices(data, s, membudget.New(limits.ShadingBudget(int64(len(data))))); err != nil {
			t.Errorf("unexpected error with full budget: %v", err)
		}
	})

	t.Run("type6", func(t *testing.T) {
		s := testCases[6][0].shading.(*Type6)
		data := meshStreamData(t, s)
		if _, err := parseType6Patches(data, s, membudget.New(tiny)); !errors.Is(err, membudget.ErrExceeded) {
			t.Errorf("expected ErrExceeded, got %v", err)
		}
		if _, err := parseType6Patches(data, s, membudget.New(limits.ShadingBudget(int64(len(data))))); err != nil {
			t.Errorf("unexpected error with full budget: %v", err)
		}
	})

	t.Run("type7", func(t *testing.T) {
		s := testCases[7][0].shading.(*Type7)
		data := meshStreamData(t, s)
		if _, err := parseType7Patches(data, s, membudget.New(tiny)); !errors.Is(err, membudget.ErrExceeded) {
			t.Errorf("expected ErrExceeded, got %v", err)
		}
		if _, err := parseType7Patches(data, s, membudget.New(limits.ShadingBudget(int64(len(data))))); err != nil {
			t.Errorf("unexpected error with full budget: %v", err)
		}
	})
}

// TestShadingBudgetSmallMeshAccepted guards against a regression where a
// pure in-memory/decoded ratio cap rejected small, valid meshes.  A DeviceGray
// Type 4 mesh with low bit depths packs each vertex into 3 bytes but expands to
// ~56 bytes in memory; the 8 MiB floor in ShadingBudget admits small meshes
// regardless of their packing ratio.
func TestShadingBudgetSmallMeshAccepted(t *testing.T) {
	s := &Type4{
		ColorSpace:        color.SpaceDeviceGray,
		BitsPerCoordinate: 8,
		BitsPerComponent:  4,
		BitsPerFlag:       2,
		Decode:            []float64{0, 100, 0, 100, 0, 1},
	}
	for i := range 10 {
		s.Vertices = append(s.Vertices, Type4Vertex{X: float64(i), Y: float64(i), Color: []float64{0.5}})
	}

	data := meshStreamData(t, s)
	got, err := parseType4Vertices(data, s, membudget.New(limits.ShadingBudget(int64(len(data)))))
	if err != nil {
		t.Fatalf("small valid mesh rejected: %v", err)
	}
	if len(got) != len(s.Vertices) {
		t.Errorf("got %d vertices, want %d", len(got), len(s.Vertices))
	}
}
