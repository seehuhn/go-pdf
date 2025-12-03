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

import "testing"

func TestContentType_String(t *testing.T) {
	tests := []struct {
		ct   Type
		want string
	}{
		{PageContent, "page"},
		{FormContent, "form"},
		{PatternContent, "pattern"},
		{Type3Content, "type3"},
	}
	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.want {
			t.Errorf("ContentType(%d).String() = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestType3Mode_String(t *testing.T) {
	tests := []struct {
		mode Type3Mode
		want string
	}{
		{Type3ModeUnset, "unset"},
		{Type3ModeD0, "d0"},
		{Type3ModeD1, "d1"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("Type3Mode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}
