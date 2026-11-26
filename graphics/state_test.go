// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package graphics

import (
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
)

type stubSoftClip struct{}

func (stubSoftClip) Embed(*pdf.EmbedHelper) (pdf.Native, error) { return nil, nil }

func (stubSoftClip) Equal(other SoftClip) bool {
	_, ok := other.(stubSoftClip)
	return ok
}

func TestSoftMaskCTMApplyTo(t *testing.T) {
	m := matrix.Translate(10, 20)

	src := NewState()
	src.SoftMask = stubSoftClip{}
	src.SoftMaskCTM = m

	dst := NewState()
	src.Set = StateSoftMask
	src.ApplyTo(&dst)
	if dst.SoftMaskCTM != m {
		t.Errorf("SoftMaskCTM not copied: got %v, want %v", dst.SoftMaskCTM, m)
	}

	dst = NewState()
	src.Set = StateBlendMode
	src.ApplyTo(&dst)
	if dst.SoftMaskCTM != (matrix.Matrix{}) {
		t.Errorf("SoftMaskCTM copied without StateSoftMask: %v", dst.SoftMaskCTM)
	}
}

func TestCloneClipPaths(t *testing.T) {
	cp := func(x float64) ClipPath { return ClipPath{CTM: matrix.Matrix{x, 0, 0, 1, 0, 0}} }

	// give the slice spare capacity, so that a careless append would write
	// into the shared backing array
	orig := &State{ClipPaths: make([]ClipPath, 1, 8)}
	orig.ClipPaths[0] = cp(1)

	clone := orig.Clone()
	clone.AppendClip(cp(2))
	if len(orig.ClipPaths) != 1 || orig.ClipPaths[0] != cp(1) {
		t.Errorf("original changed: %v", orig.ClipPaths)
	}

	clone2 := orig.Clone()
	orig.AppendClip(cp(3))
	clone2.AppendClip(cp(4))
	if len(clone.ClipPaths) != 2 || clone.ClipPaths[1] != cp(2) {
		t.Errorf("first clone changed: %v", clone.ClipPaths)
	}
	if len(orig.ClipPaths) != 2 || orig.ClipPaths[1] != cp(3) {
		t.Errorf("original changed by clone: %v", orig.ClipPaths)
	}
	if len(clone2.ClipPaths) != 2 || clone2.ClipPaths[1] != cp(4) {
		t.Errorf("second clone changed: %v", clone2.ClipPaths)
	}
}
