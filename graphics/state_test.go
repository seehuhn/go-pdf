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
