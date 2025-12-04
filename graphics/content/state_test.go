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

	"seehuhn.de/go/pdf/graphics"
)

func TestNewState_Page(t *testing.T) {
	s := NewState(Page)

	// Page: Set=initializedStateBits, Known=initializedStateBits
	// Font is NOT in initializedStateBits
	if s.Set&graphics.StateTextFont != 0 {
		t.Error("Page: font should be Unset")
	}
	if s.Known&graphics.StateTextFont != 0 {
		t.Error("Page: font should not be Known")
	}

	// LineWidth should be Set and Known
	if s.Set&graphics.StateLineWidth == 0 {
		t.Error("Page: line width should be Set")
	}
	if s.Known&graphics.StateLineWidth == 0 {
		t.Error("Page: line width should be Known")
	}
}

func TestNewState_Form(t *testing.T) {
	s := NewState(Form)

	// Form: Set=AllStateBits, Known=0
	if s.Set != graphics.AllStateBits {
		t.Errorf("Form: Set = %v, want AllStateBits", s.Set)
	}
	if s.Known != 0 {
		t.Errorf("Form: Known = %v, want 0", s.Known)
	}
}

func TestState_IsKnown(t *testing.T) {
	s := NewState(Page)

	// LineWidth is Known for Page
	if !s.IsKnown(graphics.StateLineWidth) {
		t.Error("line width should be Known")
	}

	// Font is not Known
	if s.IsKnown(graphics.StateTextFont) {
		t.Error("font should not be Known")
	}
}

func TestState_IsSet(t *testing.T) {
	s := NewState(Form)

	// Everything is Set for Form
	if !s.IsSet(graphics.StateLineWidth) {
		t.Error("line width should be Set")
	}
	if !s.IsSet(graphics.StateTextFont) {
		t.Error("font should be Set")
	}
}

func TestState_MarkAsSet(t *testing.T) {
	s := NewState(Form)

	// Initially not Known
	if s.IsKnown(graphics.StateLineWidth) {
		t.Error("line width should not be Known initially")
	}

	// After MarkAsSet
	s.MarkAsSet(graphics.StateLineWidth)
	if !s.IsKnown(graphics.StateLineWidth) {
		t.Error("line width should be Known after MarkAsSet")
	}
}

func TestState_MarkUsedUnknown(t *testing.T) {
	s := NewState(Form)

	// Use a Set-Unknown parameter
	s.MarkAsUsed(graphics.StateLineWidth)
	if s.FromContext&graphics.StateLineWidth == 0 {
		t.Error("line width should be in UsedUnknown")
	}

	// Known params should not be marked
	s.MarkAsSet(graphics.StateLineCap)
	s.MarkAsUsed(graphics.StateLineCap)
	if s.FromContext&graphics.StateLineCap != 0 {
		t.Error("line cap should not be in UsedUnknown (it's Known)")
	}
}

func TestState_PushPop(t *testing.T) {
	s := NewState(Form) // Form has Set=All, Known=0

	// Mark line width as Known
	s.MarkAsSet(graphics.StateLineWidth)
	if !s.IsKnown(graphics.StateLineWidth) {
		t.Fatal("line width should be Known after MarkAsSet")
	}

	// Push state
	if err := s.Push(); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Modify Known bits
	s.Known = 0

	// Pop state
	if err := s.Pop(); err != nil {
		t.Fatalf("Pop failed: %v", err)
	}

	// Known should be restored
	if !s.IsKnown(graphics.StateLineWidth) {
		t.Error("line width Known bit should be restored after Pop")
	}
}

func TestState_PopEmpty(t *testing.T) {
	s := NewState(Page)

	// Pop on empty stack should error
	if err := s.Pop(); err == nil {
		t.Error("Pop on empty stack should error")
	}
}

func TestState_MaxStackDepth(t *testing.T) {
	s := NewState(Page)

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
			s := NewState(Page)
			tt.setup(s)
			err := s.CanClose()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
