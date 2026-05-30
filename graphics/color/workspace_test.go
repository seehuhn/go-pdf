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

package color

import (
	"testing"

	"seehuhn.de/go/icc"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
)

// TestToXYZWorkspaceEquivalence checks that Space.ToXYZ produces identical
// results whether a Workspace is supplied or not, across every space type
// (including the nested Indexed/Separation/DeviceN paths).
func TestToXYZWorkspaceEquivalence(t *testing.T) {
	ws := &icc.Workspace{}
	for _, cs := range testColorSpaces {
		n := cs.Channels()
		if n == 0 {
			continue // pattern spaces take no component values
		}
		lo, hi := cs.ComponentRanges()
		values := make([]float64, n)
		for _, frac := range []float64{0, 0.25, 0.5, 0.75, 1} {
			for i := range n {
				values[i] = lo[i] + frac*(hi[i]-lo[i])
			}
			wantX, wantY, wantZ := cs.ToXYZ(values, nil)
			gotX, gotY, gotZ := cs.ToXYZ(values, ws)
			if gotX != wantX || gotY != wantY || gotZ != wantZ {
				t.Errorf("%s ToXYZ(%v): with ws (%v,%v,%v) != without (%v,%v,%v)",
					cs.Family(), values, gotX, gotY, gotZ, wantX, wantY, wantZ)
			}
		}
	}
}

// TestToXYZWorkspaceZeroAlloc verifies that, with a supplied Workspace, the
// ICC-based and the nested Indexed-of-ICCBased conversion paths perform no heap
// allocation per call.
func TestToXYZWorkspaceZeroAlloc(t *testing.T) {
	iccCMYK := must(ICCBased(icc.CMYKProfile, nil)).(*SpaceICCBased)
	idx := must(Indexed([]Color{iccCMYK.Default(), iccCMYK.Default()})).(*SpaceIndexed)

	// Indexed over a DeviceN base exercises the slotIdx and slotAlt scratch
	// buffers together; a single-colorant Type 2 transform keeps the conversion
	// allocation-free.
	dn := must(DeviceN([]pdf.Name{"ink"}, SpaceDeviceRGB, &function.Type2{
		XMin: 0,
		XMax: 1,
		C0:   []float64{0, 0, 0},
		C1:   []float64{1, 1, 1},
		N:    1,
	}, nil)).(*SpaceDeviceN)
	idxDN := must(Indexed([]Color{dn.New([]float64{0.3}), dn.New([]float64{0.7})})).(*SpaceIndexed)

	ws := &icc.Workspace{}
	cmykIn := []float64{0.1, 0.2, 0.3, 0.4}
	idxIn := []float64{1}

	// prime workspace buffers and the cached forward transform / ranges
	iccCMYK.ToXYZ(cmykIn, ws)
	idx.ToXYZ(idxIn, ws)
	idxDN.ToXYZ(idxIn, ws)

	if n := testing.AllocsPerRun(100, func() {
		iccCMYK.ToXYZ(cmykIn, ws)
	}); n != 0 {
		t.Errorf("SpaceICCBased.ToXYZ with workspace: got %v allocs/op, want 0", n)
	}

	if n := testing.AllocsPerRun(100, func() {
		idx.ToXYZ(idxIn, ws)
	}); n != 0 {
		t.Errorf("Indexed-of-ICCBased ToXYZ with workspace: got %v allocs/op, want 0", n)
	}

	if n := testing.AllocsPerRun(100, func() {
		idxDN.ToXYZ(idxIn, ws)
	}); n != 0 {
		t.Errorf("Indexed-of-DeviceN ToXYZ with workspace: got %v allocs/op, want 0", n)
	}
}

func BenchmarkSpaceICCBasedToXYZ(b *testing.B) {
	cs := must(ICCBased(icc.CMYKProfile, nil)).(*SpaceICCBased)
	in := []float64{0.1, 0.2, 0.3, 0.4}

	b.Run("nil", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			cs.ToXYZ(in, nil)
		}
	})
	b.Run("workspace", func(b *testing.B) {
		ws := &icc.Workspace{}
		cs.ToXYZ(in, ws)
		b.ReportAllocs()
		for range b.N {
			cs.ToXYZ(in, ws)
		}
	})
}
