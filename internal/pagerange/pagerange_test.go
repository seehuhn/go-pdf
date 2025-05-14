// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package pagerange

import (
	"reflect"
	"testing"
)

func TestPageRangeString(t *testing.T) {
	tests := []struct {
		name string
		pr   PageRange
		want string
	}{
		{"Single page", PageRange{5, 5}, "5"},
		{"Page range", PageRange{1, 10}, "1-10"},
		{"Large numbers", PageRange{1000, 9999}, "1000-9999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pr.String(); got != tt.want {
				t.Errorf("PageRange.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPageRangeSet(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    PageRange
		wantErr bool
	}{
		{"Single page", "5", PageRange{5, 5}, false},
		{"Page range", "1-10", PageRange{1, 10}, false},
		{"Same page range", "7-7", PageRange{7, 7}, false},
		{"Invalid format", "1-2-3", PageRange{}, true},
		{"Non-numeric input", "a-b", PageRange{}, true},
		{"Negative page", "-1", PageRange{}, true},
		{"Reversed range", "10-1", PageRange{}, true},
		{"Zero page", "0", PageRange{}, true},
		{"Zero in range", "0-5", PageRange{}, true},
		{"Large numbers", "1000-9999", PageRange{1000, 9999}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &PageRange{}
			err := pr.Set(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("PageRange.Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(*pr, tt.want) {
				t.Errorf("PageRange.Set() = %v, want %v", *pr, tt.want)
			}
		})
	}
}
