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

package vfinstance

import (
	"testing"

	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/varfont"
)

// static font with nil variations is returned unchanged.
func TestApplyStaticNil(t *testing.T) {
	info := makefont.TrueType()
	got, err := Apply(info, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != info {
		t.Error("static font with nil variations was not returned unchanged")
	}
}

// an unknown tag on a variable font is an error.
func TestApplyUnknownTagVariable(t *testing.T) {
	info := varfont.Glyf()
	_, err := Apply(info, map[string]float64{"xxxx": 1})
	if err == nil {
		t.Error("expected error for unknown variation axis")
	}
}

// any variations on a static font are an error (no axes to match).
func TestApplyVariationsOnStatic(t *testing.T) {
	info := makefont.TrueType()
	_, err := Apply(info, map[string]float64{"wght": 700})
	if err == nil {
		t.Error("expected error for variations on a static font")
	}
}

// a variable font with explicit variations is pinned to a static instance.
func TestApplyVariableInstanced(t *testing.T) {
	info := varfont.Glyf()
	if !info.IsVariable() {
		t.Fatal("synthetic font is not variable")
	}

	got, err := Apply(info, map[string]float64{"wght": 700})
	if err != nil {
		t.Fatal(err)
	}
	if got.IsVariable() {
		t.Error("instanced font is still variable")
	}
	if got == info {
		t.Error("variable font was not replaced by an instance")
	}
}

// a variable font with nil variations is pinned at its default coordinates.
func TestApplyVariableNilInstanced(t *testing.T) {
	info := varfont.Glyf()
	got, err := Apply(info, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsVariable() {
		t.Error("instanced font is still variable")
	}
}

// a static CFF2 font with nil variations is converted to static CFF, even
// though it has no axes to pin.
func TestApplyStaticCFF2(t *testing.T) {
	info := varfont.StaticCFF2()
	if info.IsVariable() {
		t.Fatal("fixture is unexpectedly variable")
	}
	if !info.IsCFF2() {
		t.Fatal("fixture does not use CFF2 outlines")
	}

	got, err := Apply(info, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsCFF2() {
		t.Error("converted font still reports CFF2")
	}
	if !got.IsCFF() {
		t.Error("converted font is not CFF")
	}
}

// applying to an already-instanced font with nil variations is a no-op.
func TestApplyIdempotent(t *testing.T) {
	info := varfont.Glyf()
	inst, err := Apply(info, map[string]float64{"wght": 700})
	if err != nil {
		t.Fatal(err)
	}
	again, err := Apply(inst, nil)
	if err != nil {
		t.Fatal(err)
	}
	if again != inst {
		t.Error("second Apply with nil variations did not return the same pointer")
	}
}
