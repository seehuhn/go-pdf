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
