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

package tounicode

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRange(t *testing.T) {
	entries := []Single{
		{Code: 1, UTF16: []uint16{53}},
		{Code: 2, UTF16: []uint16{54}},
		{Code: 3, UTF16: []uint16{55}},
		{Code: 4, UTF16: []uint16{56}},
	}
	info := FromMappings(entries)
	if len(info.Singles) != 0 {
		t.Errorf("expected 0 singles, got %d", len(info.Singles))
	}
	if len(info.Ranges) != 1 {
		t.Errorf("expected 1 ranges, got %d", len(info.Ranges))
	}
}

func TestSingles(t *testing.T) {
	entries := []Single{
		{Code: 1, UTF16: []uint16{55}},
		{Code: 3, UTF16: []uint16{54}},
		{Code: 5, UTF16: []uint16{53}},
		{Code: 7, UTF16: []uint16{52}},
	}
	info := FromMappings(entries)
	if len(info.Singles) != 4 {
		t.Errorf("expected 4 singles, got %d", len(info.Singles))
	}
	if len(info.Ranges) != 0 {
		t.Errorf("expected 0 ranges, got %d", len(info.Ranges))
	}
}

func TestNoOverflow(t *testing.T) {
	entries := []Single{
		{Code: 251, UTF16: []uint16{53}},
		{Code: 252, UTF16: []uint16{54}},
		{Code: 253, UTF16: []uint16{55}},
		{Code: 254, UTF16: []uint16{56}},
		{Code: 255, UTF16: []uint16{57}},
		{Code: 256, UTF16: []uint16{58}},
	}
	info := FromMappings(entries)
	fmt.Println(info)
	if len(info.Ranges) != 2 {
		t.Errorf("expected 2 ranges, got %d", len(info.Ranges))
	}
}

func TestExtendedPlane(t *testing.T) {
	entries := []Single{
		{Code: 18773, UTF16: []uint16{0xD861, 0xDDC8}},
		{Code: 18774, UTF16: []uint16{0xD861, 0xDDC9}},
	}
	info := FromMappings(entries)
	if len(info.Ranges) != 1 {
		t.Errorf("expected 1 ranges, got %d", len(info.Ranges))
	}
}

// TODO(voss): remove
func TestOne(t *testing.T) {
	fname := "../../../examples/try-all-pdfs/tounicode/0a3ff11a856c425e.txt"

	body, err := os.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}
	info1, err := Read(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	mappings1 := info1.ToMapping()
	fmt.Println(len(mappings1))

	info2 := FromMappings(mappings1)
	mappings2 := info2.ToMapping()

	if d := cmp.Diff(mappings1, mappings2); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

// TODO(voss): remove
func TestMany(t *testing.T) {
	files, err := filepath.Glob("../../../examples/try-all-pdfs/tounicode/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, fname := range files {
		fmt.Println(fname)
		body, err := os.ReadFile(fname)
		if err != nil {
			t.Fatal(err)
		}
		info1, err := Read(bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		mappings1 := info1.ToMapping()

		info2 := FromMappings(mappings1)
		mappings2 := info2.ToMapping()

		if d := cmp.Diff(mappings1, mappings2); d != "" {
			t.Errorf("mismatch (-want +got):\n%s", d)
		}
	}
}
