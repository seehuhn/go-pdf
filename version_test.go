// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package pdf

import "testing"

func TestVersion(t *testing.T) {
	cases := []struct {
		in  string
		out Version
		ok  bool
	}{
		{"1.0", V1_0, true},
		{"1.1", V1_1, true},
		{"1.2", V1_2, true},
		{"1.3", V1_3, true},
		{"1.4", V1_4, true},
		{"1.5", V1_5, true},
		{"1.6", V1_6, true},
		{"1.7", V1_7, true},
		{"2.0", V2_0, true},
		{"", 0, false},
		{"0.9", 0, false},
		{"1.8", 0, false},
		{"2.1", 0, false},
	}
	for _, test := range cases {
		v, err := ParseVersion(test.in)
		if (err == nil) != test.ok {
			t.Errorf("unexpected err = %s", err)
			continue
		}
		if v != test.out {
			t.Errorf("wrong version %d != %d", int(v), int(test.out))
			continue
		}
		if !test.ok {
			continue
		}
		s, err := v.ToString()
		if err != nil {
			t.Error(err)
			continue
		}
		if s != test.in {
			t.Errorf("wrong version %q != %q", s, test.in)
		}
	}
}
