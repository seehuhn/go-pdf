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

package cmap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func TestPredefined(t *testing.T) {
	dir, err := predefined.ReadDir("predefined")
	if err != nil {
		t.Fatal(err)
	}

	allNames := make([]string, len(dir))
	for i, d := range dir {
		name := d.Name()
		name = strings.TrimPrefix(name, "predefined/")
		name = strings.TrimSuffix(name, ".gz")
		allNames[i] = name
	}
	slices.Sort(allNames)

	if d := cmp.Diff(allNames, AllPredefined); d != "" {
		t.Errorf("wrong names:\n%s", d)
	}

	for _, name := range allNames {
		r, err := OpenPredefined(name)
		if err != nil {
			t.Fatal(err)
		}

		cmap, err := Read(r, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr1 := builtinCS[name]
		rr2 := cmap.CS.Ranges()
		if d := cmp.Diff(rr1, rr2); d != "" {
			fmt.Printf("\t%q: {\n", name)
			for _, r := range rr2 {
				fmt.Printf("\t\t{Low: %#v, High: %#v},\n", r.Low, r.High)
			}
			fmt.Printf("\t},\n")
			t.Errorf("wrong ranges for %q:\n%s", name, d)
		}

		err = r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestConsistency(t *testing.T) {
	names := maps.Keys(builtinCS)
	slices.Sort(names)
	if d := cmp.Diff(names, AllPredefined); d != "" {
		t.Errorf("wrong names:\n%s", d)
	}
}
