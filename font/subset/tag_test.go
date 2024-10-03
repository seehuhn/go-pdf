// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package subset

import (
	"testing"
)

func TestGetSubsetTag(t *testing.T) {
	tag := Tag(nil, 0)
	if tag != "AAAAAA" {
		t.Error("wrong tag " + tag)
	}
}

func TestIsValidTag(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"Valid tag", "ABCDEF", true},
		{"Valid tag with different letters", "XYZPQR", true},
		{"Empty string", "", false},
		{"Too short", "ABCDE", false},
		{"Too long", "ABCDEFG", false},
		{"Lowercase letters", "abcdef", false},
		{"Mixed case", "ABCDEf", false},
		{"Numbers", "A1CDEF", false},
		{"Special characters", "A@CDEF", false},
		{"Unicode characters", "АBCDEF", false}, // first letter cyrillic
		{"Spaces", "ABC EF", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidTag(tt.input); got != tt.want {
				t.Errorf("IsValidTag(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
