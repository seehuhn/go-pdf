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

import (
	"bytes"
	"sort"
	"strings"
	"testing"

	"golang.org/x/exp/maps"
)

func TestSequential(t *testing.T) {
	file := `%PDF-1.7
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj
2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj
3 0 obj
<<
/Type /Page
/Parent 2 0 R
/Contents 4 0 R
>>
endobj
4 0 obj
<<
/Length 36
>>
stream
0 0 m
100 0 l
100 100 l
0 100 l
h
f
endstream
endobj
trailer
<<
/Root 1 0 R
>>
`
	info, err := SequentialScan(strings.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	r, err := info.MakeReader(nil)
	if err != nil {
		t.Fatal(err)
	}

	type objInfo struct {
		pos int64
		gen uint16
	}
	objs := make(map[uint32]objInfo)
	for _, sect := range info.Sections {
		for _, obj := range sect.Objects {
			n := obj.Reference.Number()
			if obj.Broken || obj.Reference.Generation() < objs[n].gen {
				continue
			}
			objs[n] = objInfo{obj.ObjStart, obj.Reference.Generation()}
		}
	}
	keys := maps.Keys(objs)
	sort.Slice(keys, func(i, j int) bool {
		return objs[keys[i]].pos < objs[keys[j]].pos
	})

	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range keys {
		ref := NewReference(n, objs[n].gen)
		obj, err := Resolve(r, ref)
		if err != nil {
			t.Fatal(err)
		}
		err = w.Put(ref, obj)
		if err != nil {
			t.Fatal(err)
		}
	}
	*w.GetMeta().Catalog = *r.GetMeta().Catalog
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
