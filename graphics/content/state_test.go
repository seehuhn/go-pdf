// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package content

import (
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

func TestNewState_Page(t *testing.T) {
	s := NewState(Page, &Resources{})

	// Page: Set=initializedStateBits, Known=initializedStateBits
	// Font is NOT in initializedStateBits
	if s.Usable&graphics.StateTextFont != 0 {
		t.Error("Page: font should be Unset")
	}
	if s.GState.Set&graphics.StateTextFont != 0 {
		t.Error("Page: font should not be Known")
	}

	// LineWidth should be Set and Known
	if s.Usable&graphics.StateLineWidth == 0 {
		t.Error("Page: line width should be Set")
	}
	if s.GState.Set&graphics.StateLineWidth == 0 {
		t.Error("Page: line width should be Known")
	}
}

func TestNewState_Form(t *testing.T) {
	s := NewState(Form, &Resources{})

	// Form: Set=AllStateBits, Known=0
	if s.Usable != graphics.AllBits {
		t.Errorf("Form: Set = %v, want AllStateBits", s.Usable)
	}
	if s.GState.Set != 0 {
		t.Errorf("Form: Known = %v, want 0", s.GState.Set)
	}
}

func TestState_IsKnown(t *testing.T) {
	s := NewState(Page, &Resources{})

	// LineWidth is Known for Page
	if !s.IsSet(graphics.StateLineWidth) {
		t.Error("line width should be Known")
	}

	// Font is not Known
	if s.IsSet(graphics.StateTextFont) {
		t.Error("font should not be Known")
	}
}

func TestState_IsSet(t *testing.T) {
	s := NewState(Form, &Resources{})

	// Everything is Set for Form
	if !s.IsUsable(graphics.StateLineWidth) {
		t.Error("line width should be Set")
	}
	if !s.IsUsable(graphics.StateTextFont) {
		t.Error("font should be Set")
	}
}

func TestState_PushPop(t *testing.T) {
	s := NewState(Form, &Resources{}) // Form has Set=All, Known=0

	s.Usable |= graphics.StateLineWidth
	s.GState.Set |= graphics.StateLineWidth

	// Push state
	if err := s.Push(); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Modify Known bits
	s.GState.Set = 0

	// Pop state
	if err := s.Pop(); err != nil {
		t.Fatalf("Pop failed: %v", err)
	}

	// Known should be restored
	if !s.IsSet(graphics.StateLineWidth) {
		t.Error("line width Known bit should be restored after Pop")
	}
}

func TestState_PopEmpty(t *testing.T) {
	s := NewState(Page, &Resources{})

	// Pop on empty stack should error
	if err := s.Pop(); err == nil {
		t.Error("Pop on empty stack should error")
	}
}

func TestState_MaxStackDepth(t *testing.T) {
	s := NewState(Page, &Resources{})

	for i := 0; i < 5; i++ {
		s.Push()
	}
	if s.MaxStackDepth != 5 {
		t.Errorf("MaxStackDepth = %d, want 5", s.MaxStackDepth)
	}

	// Pop some
	s.Pop()
	s.Pop()

	// MaxStackDepth should not decrease
	if s.MaxStackDepth != 5 {
		t.Errorf("MaxStackDepth = %d, want 5 (should not decrease)", s.MaxStackDepth)
	}
}

func TestState_Validate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*State)
		wantErr bool
	}{
		{
			name:    "valid initial state",
			setup:   func(s *State) {},
			wantErr: false,
		},
		{
			name: "unclosed q",
			setup: func(s *State) {
				s.Push()
			},
			wantErr: true,
		},
		{
			name: "in text object",
			setup: func(s *State) {
				s.CurrentObject = ObjText
			},
			wantErr: true,
		},
		{
			name: "unbalanced marked content",
			setup: func(s *State) {
				s.MarkedContentBegin()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewState(Page, &Resources{})
			tt.setup(s)
			err := s.CanClose()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// applyOps is a helper that applies a sequence of (opName, args) pairs.
func applyOps(s *State, ops ...any) {
	for i := 0; i < len(ops); i += 2 {
		name := ops[i].(OpName)
		args := ops[i+1].([]pdf.Object)
		s.ApplyStateChanges(name, args)
	}
}

func n(v float64) pdf.Object { return pdf.Number(v) }

func TestClipCapture(t *testing.T) {
	s := NewState(Page, &Resources{})

	// 100 100 200 200 re W n
	applyOps(s,
		OpRectangle, []pdf.Object{n(100), n(100), n(200), n(200)},
		OpClipNonZero, []pdf.Object{},
		OpEndPath, []pdf.Object{},
	)

	if len(s.GState.ClipPaths) != 1 {
		t.Fatalf("got %d clip paths, want 1", len(s.GState.ClipPaths))
	}
	cp := s.GState.ClipPaths[0]
	if cp.EvenOdd {
		t.Error("expected non-zero winding rule")
	}
	// rectangle: MoveTo + 3 LineTo + Close = 5 commands
	if len(cp.Path.Cmds) != 5 {
		t.Errorf("got %d path commands, want 5", len(cp.Path.Cmds))
	}
	if cp.Path.Cmds[0] != path.CmdMoveTo {
		t.Errorf("first command = %v, want MoveTo", cp.Path.Cmds[0])
	}
}

func TestClipEvenOdd(t *testing.T) {
	s := NewState(Page, &Resources{})

	applyOps(s,
		OpRectangle, []pdf.Object{n(0), n(0), n(50), n(50)},
		OpClipEvenOdd, []pdf.Object{},
		OpEndPath, []pdf.Object{},
	)

	if len(s.GState.ClipPaths) != 1 {
		t.Fatalf("got %d clip paths, want 1", len(s.GState.ClipPaths))
	}
	if !s.GState.ClipPaths[0].EvenOdd {
		t.Error("expected even-odd fill rule")
	}
}

func TestClipWithPaint(t *testing.T) {
	s := NewState(Page, &Resources{})

	// W followed by f (fill) should still capture the clip
	applyOps(s,
		OpRectangle, []pdf.Object{n(10), n(10), n(80), n(80)},
		OpClipNonZero, []pdf.Object{},
		OpFill, []pdf.Object{},
	)

	if len(s.GState.ClipPaths) != 1 {
		t.Fatalf("got %d clip paths, want 1", len(s.GState.ClipPaths))
	}
}

func TestClipSaveRestore(t *testing.T) {
	s := NewState(Page, &Resources{})

	// q ... W n ... Q should revert ClipPaths
	applyOps(s,
		OpPushGraphicsState, []pdf.Object{},
		OpRectangle, []pdf.Object{n(0), n(0), n(100), n(100)},
		OpClipNonZero, []pdf.Object{},
		OpEndPath, []pdf.Object{},
	)
	if len(s.GState.ClipPaths) != 1 {
		t.Fatalf("after clip: got %d clip paths, want 1", len(s.GState.ClipPaths))
	}

	applyOps(s,
		OpPopGraphicsState, []pdf.Object{},
	)
	if len(s.GState.ClipPaths) != 0 {
		t.Errorf("after Q: got %d clip paths, want 0", len(s.GState.ClipPaths))
	}
}

func TestClipNested(t *testing.T) {
	s := NewState(Page, &Resources{})

	// two successive clips
	applyOps(s,
		OpRectangle, []pdf.Object{n(0), n(0), n(200), n(200)},
		OpClipNonZero, []pdf.Object{},
		OpEndPath, []pdf.Object{},
		OpRectangle, []pdf.Object{n(50), n(50), n(100), n(100)},
		OpClipEvenOdd, []pdf.Object{},
		OpEndPath, []pdf.Object{},
	)

	if len(s.GState.ClipPaths) != 2 {
		t.Fatalf("got %d clip paths, want 2", len(s.GState.ClipPaths))
	}
	if s.GState.ClipPaths[0].EvenOdd {
		t.Error("first clip should be non-zero")
	}
	if !s.GState.ClipPaths[1].EvenOdd {
		t.Error("second clip should be even-odd")
	}
}

func TestClipCTMCapture(t *testing.T) {
	s := NewState(Page, &Resources{})

	// apply a CTM transformation, then clip
	scale := matrix.Matrix{2, 0, 0, 2, 10, 20}
	applyOps(s,
		OpTransform, []pdf.Object{n(2), n(0), n(0), n(2), n(10), n(20)},
		OpRectangle, []pdf.Object{n(0), n(0), n(50), n(50)},
		OpClipNonZero, []pdf.Object{},
		OpEndPath, []pdf.Object{},
	)

	if len(s.GState.ClipPaths) != 1 {
		t.Fatalf("got %d clip paths, want 1", len(s.GState.ClipPaths))
	}
	got := s.GState.ClipPaths[0].CTM
	if got != scale {
		t.Errorf("CTM = %v, want %v", got, scale)
	}
}

func TestPaintedPathFill(t *testing.T) {
	s := NewState(Page, &Resources{})

	// rectangle produces MoveTo + 3 LineTo + Close = 5 cmds
	applyOps(s,
		OpRectangle, []pdf.Object{n(0), n(0), n(100), n(100)},
		OpFill, []pdf.Object{},
	)

	pp := s.PaintedPath()
	if len(pp.Cmds) != 5 {
		t.Errorf("PaintedPath after fill: got %d cmds, want 5", len(pp.Cmds))
	}
	if pp.Cmds[0] != path.CmdMoveTo {
		t.Errorf("first cmd = %v, want MoveTo", pp.Cmds[0])
	}
	if len(s.currentPath.Cmds) != 0 {
		t.Errorf("currentPath not cleared: %d cmds remain", len(s.currentPath.Cmds))
	}
}

func TestPaintedPathCloseAndStroke(t *testing.T) {
	s := NewState(Page, &Resources{})

	// open triangle: m l l s → library closes → MoveTo + 2 LineTo + Close = 4 cmds
	applyOps(s,
		OpMoveTo, []pdf.Object{n(0), n(0)},
		OpLineTo, []pdf.Object{n(100), n(0)},
		OpLineTo, []pdf.Object{n(50), n(80)},
		OpCloseAndStroke, []pdf.Object{},
	)

	pp := s.PaintedPath()
	if len(pp.Cmds) != 4 {
		t.Errorf("PaintedPath after s: got %d cmds, want 4", len(pp.Cmds))
	}
	if pp.Cmds[len(pp.Cmds)-1] != path.CmdClose {
		t.Errorf("last cmd = %v, want Close", pp.Cmds[len(pp.Cmds)-1])
	}
}

func TestPaintedPathClipIncludesClose(t *testing.T) {
	s := NewState(Page, &Resources{})

	// W followed by b (close-fill-stroke) should produce a clip path
	// that includes the implicit close
	applyOps(s,
		OpMoveTo, []pdf.Object{n(0), n(0)},
		OpLineTo, []pdf.Object{n(100), n(0)},
		OpLineTo, []pdf.Object{n(50), n(80)},
		OpClipNonZero, []pdf.Object{},
		OpCloseFillAndStroke, []pdf.Object{},
	)

	if len(s.GState.ClipPaths) != 1 {
		t.Fatalf("got %d clip paths, want 1", len(s.GState.ClipPaths))
	}
	cp := s.GState.ClipPaths[0]
	if len(cp.Path.Cmds) != 4 {
		t.Errorf("clip path: got %d cmds, want 4", len(cp.Path.Cmds))
	}
	if cp.Path.Cmds[len(cp.Path.Cmds)-1] != path.CmdClose {
		t.Errorf("clip path last cmd = %v, want Close", cp.Path.Cmds[len(cp.Path.Cmds)-1])
	}
}

func TestPaintedPathEndPath(t *testing.T) {
	s := NewState(Page, &Resources{})

	applyOps(s,
		OpRectangle, []pdf.Object{n(10), n(20), n(30), n(40)},
		OpEndPath, []pdf.Object{},
	)

	pp := s.PaintedPath()
	if len(pp.Cmds) != 5 {
		t.Errorf("PaintedPath after n: got %d cmds, want 5", len(pp.Cmds))
	}
}

func TestPaintedPathOverwrite(t *testing.T) {
	s := NewState(Page, &Resources{})

	// first paint: rectangle (5 cmds)
	applyOps(s,
		OpRectangle, []pdf.Object{n(0), n(0), n(100), n(100)},
		OpFill, []pdf.Object{},
	)
	if len(s.PaintedPath().Cmds) != 5 {
		t.Fatalf("first paint: got %d cmds, want 5", len(s.PaintedPath().Cmds))
	}

	// second paint: triangle (3 cmds: m l l)
	applyOps(s,
		OpMoveTo, []pdf.Object{n(0), n(0)},
		OpLineTo, []pdf.Object{n(50), n(0)},
		OpLineTo, []pdf.Object{n(25), n(40)},
		OpStroke, []pdf.Object{},
	)
	if len(s.PaintedPath().Cmds) != 3 {
		t.Errorf("second paint: got %d cmds, want 3", len(s.PaintedPath().Cmds))
	}
	if len(s.currentPath.Cmds) != 0 {
		t.Errorf("currentPath not cleared: %d cmds remain", len(s.currentPath.Cmds))
	}
}

func TestClipPathReset(t *testing.T) {
	s := NewState(Page, &Resources{})

	// after a paint op without W, current path should be cleared
	applyOps(s,
		OpRectangle, []pdf.Object{n(0), n(0), n(100), n(100)},
		OpEndPath, []pdf.Object{},
	)

	if len(s.GState.ClipPaths) != 0 {
		t.Errorf("got %d clip paths without W, want 0", len(s.GState.ClipPaths))
	}
	if len(s.currentPath.Cmds) != 0 {
		t.Errorf("current path not cleared: %d commands remain", len(s.currentPath.Cmds))
	}
}
