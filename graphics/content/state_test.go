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

func TestNewState_PageContent(t *testing.T) {
	s := NewStateForContent(PageContent)

	// PageContent: Set=initializedStateBits, Known=initializedStateBits
	// Font is NOT in initializedStateBits
	if s.Set&graphics.StateTextFont != 0 {
		t.Error("PageContent: font should be Unset")
	}
	if s.Known&graphics.StateTextFont != 0 {
		t.Error("PageContent: font should not be Known")
	}

	// LineWidth should be Set and Known
	if s.Set&graphics.StateLineWidth == 0 {
		t.Error("PageContent: line width should be Set")
	}
	if s.Known&graphics.StateLineWidth == 0 {
		t.Error("PageContent: line width should be Known")
	}
}

func TestNewState_FormContent(t *testing.T) {
	s := NewStateForContent(FormContent)

	// FormContent: Set=AllStateBits, Known=0
	if s.Set != graphics.AllStateBits {
		t.Errorf("FormContent: Set = %v, want AllStateBits", s.Set)
	}
	if s.Known != 0 {
		t.Errorf("FormContent: Known = %v, want 0", s.Known)
	}
}

func TestState_IsKnown(t *testing.T) {
	s := NewStateForContent(PageContent)

	// LineWidth is Known for PageContent
	if !s.IsKnown(graphics.StateLineWidth) {
		t.Error("line width should be Known")
	}

	// Font is not Known
	if s.IsKnown(graphics.StateTextFont) {
		t.Error("font should not be Known")
	}
}

func TestState_IsSet(t *testing.T) {
	s := NewStateForContent(FormContent)

	// Everything is Set for FormContent
	if !s.IsSet(graphics.StateLineWidth) {
		t.Error("line width should be Set")
	}
	if !s.IsSet(graphics.StateTextFont) {
		t.Error("font should be Set")
	}
}

func TestState_MarkKnown(t *testing.T) {
	s := NewStateForContent(FormContent)

	// Initially not Known
	if s.IsKnown(graphics.StateLineWidth) {
		t.Error("line width should not be Known initially")
	}

	// After marking Known
	s.MarkKnown(graphics.StateLineWidth)
	if !s.IsKnown(graphics.StateLineWidth) {
		t.Error("line width should be Known after marking")
	}
}

func TestState_MarkUsedUnknown(t *testing.T) {
	s := NewStateForContent(FormContent)

	// Use a Set-Unknown parameter
	s.MarkUsedIfUnknown(graphics.StateLineWidth)
	if s.UsedUnknown&graphics.StateLineWidth == 0 {
		t.Error("line width should be in UsedUnknown")
	}

	// Known params should not be marked
	s.MarkKnown(graphics.StateLineCap)
	s.MarkUsedIfUnknown(graphics.StateLineCap)
	if s.UsedUnknown&graphics.StateLineCap != 0 {
		t.Error("line cap should not be in UsedUnknown (it's Known)")
	}
}

func TestState_PushPop(t *testing.T) {
	s := NewStateForContent(PageContent)

	// Set line width to custom value
	s.Param.LineWidth = 5.0
	s.MarkKnown(graphics.StateLineWidth)

	// Push state
	if err := s.Push(); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Modify line width
	s.Param.LineWidth = 10.0

	// Pop state
	if err := s.Pop(); err != nil {
		t.Fatalf("Pop failed: %v", err)
	}

	// Should be restored
	if s.Param.LineWidth != 5.0 {
		t.Errorf("LineWidth = %v, want 5.0", s.Param.LineWidth)
	}
}

func TestState_PopEmpty(t *testing.T) {
	s := NewStateForContent(PageContent)

	// Pop on empty stack should error
	if err := s.Pop(); err == nil {
		t.Error("Pop on empty stack should error")
	}
}

func TestState_MaxStackDepth(t *testing.T) {
	s := NewStateForContent(PageContent)

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
			s := NewStateForContent(PageContent)
			tt.setup(s)
			err := s.CanClose()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
