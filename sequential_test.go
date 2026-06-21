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
	"cmp"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"
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
	info, err := SequentialScan(strings.NewReader(file), int64(len(file)))
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
	keys := slices.SortedFunc(maps.Keys(objs), func(a, b uint32) int {
		return cmp.Compare(objs[a].pos, objs[b].pos)
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

// TestSequentialIndirectLength scans a damaged file (no xref) containing a
// content stream whose /Length is an indirect reference, the normal way PDFs
// encode streams.  The scan must classify it as a stream, keep it (not mark it
// broken), and make it resolvable through the recovered reader; otherwise
// recovery silently drops every stream in the file.
func TestSequentialIndirectLength(t *testing.T) {
	file := "%PDF-1.5\n" +
		"1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj\n" +
		"2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj\n" +
		"3 0 obj << /Type /Page /Parent 2 0 R /Contents 4 0 R /MediaBox [0 0 100 100] >> endobj\n" +
		"4 0 obj << /Length 5 0 R >> stream\nBT ET\nendstream endobj\n" +
		"5 0 obj 5 endobj\n" +
		"trailer << /Root 1 0 R >>\n"
	info, err := SequentialScan(strings.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}

	// the content stream must be classified and kept, not marked broken
	stmObj := info.findObject(NewReference(4, 0))
	if stmObj == nil {
		t.Fatal("content stream not located")
	}
	if stmObj.Broken {
		t.Error("content stream marked broken")
	}
	if stmObj.Type != "Stream" {
		t.Errorf("content stream Type = %q, want \"Stream\"", stmObj.Type)
	}

	// and it must resolve through the recovered reader
	r, err := info.MakeReader(nil)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := NewCursor(r).Resolve(NewReference(4, 0))
	if err != nil {
		t.Fatal(err)
	}
	stm, ok := obj.(*Stream)
	if !ok {
		t.Fatalf("recovered object = %T, want *Stream", obj)
	}
	if stm.length != 5 {
		t.Errorf("stream length = %d, want 5", stm.length)
	}
}

// TestSequentialBrokenLength scans a damaged file (no xref) whose streams have
// unusable /Length values: one resolves through a reference cycle, one points
// at another stream object.  A single bad /Length must not abort the whole
// scan; each stream whose endstream is recoverable is kept (extent found by
// scanning), and a stream with no recoverable endstream is marked broken.
func TestSequentialBrokenLength(t *testing.T) {
	file := "%PDF-1.5\n" +
		"1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj\n" +
		"2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj\n" +
		"3 0 obj << /Type /Page /Parent 2 0 R /Contents 4 0 R /MediaBox [0 0 100 100] >> endobj\n" +
		// /Length resolves through a 7<->8 reference cycle
		"4 0 obj << /Length 7 0 R >> stream\nBT ET\nendstream endobj\n" +
		"7 0 obj 8 0 R endobj\n" +
		"8 0 obj 7 0 R endobj\n" +
		// /Length points at a stream object instead of an integer
		"5 0 obj << /Length 4 0 R >> stream\nHELLO\nendstream endobj\n" +
		// truncated stream with no recoverable endstream (must come last)
		"9 0 obj << /Length 999 >> stream\nNOPE\n" +
		"trailer << /Root 1 0 R >>\n"
	info, err := SequentialScan(strings.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}

	r, err := info.MakeReader(nil)
	if err != nil {
		t.Fatal(err)
	}

	// both recoverable streams are kept and decode to their real content
	for _, tc := range []struct {
		ref     Reference
		content string
	}{
		{NewReference(4, 0), "BT ET"},
		{NewReference(5, 0), "HELLO"},
	} {
		stmObj := info.findObject(tc.ref)
		if stmObj == nil {
			t.Fatalf("%s: not located", tc.ref)
		}
		if stmObj.Broken {
			t.Errorf("%s: marked broken", tc.ref)
		}
		if stmObj.Type != "Stream" {
			t.Errorf("%s: Type = %q, want \"Stream\"", tc.ref, stmObj.Type)
		}

		obj, err := NewCursor(r).Resolve(tc.ref)
		if err != nil {
			t.Fatalf("%s: resolve: %v", tc.ref, err)
		}
		stm, ok := obj.(*Stream)
		if !ok {
			t.Fatalf("%s: recovered object = %T, want *Stream", tc.ref, obj)
		}
		body, err := DecodeStream(r, nil, stm)
		if err != nil {
			t.Fatalf("%s: decode: %v", tc.ref, err)
		}
		data, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("%s: read: %v", tc.ref, err)
		}
		if string(data) != tc.content {
			t.Errorf("%s: content = %q, want %q", tc.ref, data, tc.content)
		}
	}

	// the truncated stream has no recoverable extent and must be broken,
	// without having aborted the scan above
	if stmObj := info.findObject(NewReference(9, 0)); stmObj == nil {
		t.Error("truncated stream not located")
	} else if !stmObj.Broken {
		t.Error("truncated stream not marked broken")
	}
}
