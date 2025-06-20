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

package pdf

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWriter(t *testing.T) {
	out := &bytes.Buffer{}

	opt := &WriterOptions{
		ID:              [][]byte{},
		OwnerPassword:   "test",
		UserPermissions: PermCopy,
	}
	w, err := NewWriter(out, V1_7, opt)
	if err != nil {
		t.Fatal(err)
	}
	encryptDict, err := w.w.enc.AsDict(w.meta.Version)
	if err != nil {
		t.Fatal(err)
	}
	encInfo1 := AsString(encryptDict)

	author := TextString("Jochen Voß")
	w.GetMeta().Info = &Info{
		Title:        "PDF Test Document",
		Author:       author,
		Subject:      "Testing",
		Keywords:     "PDF, testing, Go",
		CreationDate: Date(time.Now()),
	}

	refs := []Reference{w.Alloc()}
	err = w.WriteCompressed(refs,
		Dict{
			"Type":     Name("Font"),
			"Subtype":  Name("Type1"),
			"BaseFont": Name("Helvetica"),
			"Encoding": Name("MacRomanEncoding"),
		})
	if err != nil {
		t.Fatal(err)
	}
	font := refs[0]

	contentRef := w.Alloc()
	stream, err := w.OpenStream(contentRef, Dict{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = stream.Write([]byte(`BT
/F1 24 Tf
30 30 Td
(Hello World) Tj
ET
`))
	if err != nil {
		t.Fatal(err)
	}
	err = stream.Close()
	if err != nil {
		t.Fatal(err)
	}

	resources := Dict{
		"Font": Dict{"F1": font},
	}

	pagesRef := w.Alloc()
	pages := Dict{
		"Type":  Name("Pages"),
		"Kids":  Array{},
		"Count": Integer(0),
	}

	page1 := w.Alloc()
	err = w.Put(page1, Dict{
		"Type":      Name("Page"),
		"MediaBox":  Array{Integer(0), Integer(0), Integer(200), Integer(100)},
		"Resources": resources,
		"Contents":  contentRef,
		"Parent":    pagesRef,
	})
	if err != nil {
		t.Fatal(err)
	}

	pages["Kids"] = append(pages["Kids"].(Array), page1)
	pages["Count"] = pages["Count"].(Integer) + 1
	err = w.Put(pagesRef, pages)
	if err != nil {
		t.Fatal(err)
	}

	w.GetMeta().Catalog.Pages = pagesRef

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// os.WriteFile("debug.pdf", out.Bytes(), 0o644)

	outR := bytes.NewReader(out.Bytes())
	r, err := NewReader(outR, nil)
	if err != nil {
		t.Fatal(err)
	}
	encryptDict, err = r.enc.AsDict(w.meta.Version)
	if err != nil {
		t.Fatal(err)
	}
	encInfo2 := AsString(encryptDict)

	if encInfo1 != encInfo2 {
		t.Error("encryption dictionaries differ")
	}

	_, err = r.enc.sec.GetKey(false)
	if err != nil {
		t.Fatal(err)
	}

	if r.meta.Info == nil {
		t.Fatal("no document information dictionary")
	}
	if x := r.meta.Info.Author; x != author {
		t.Error("wrong author " + x)
	}
}

type testCloseWriter struct {
	isClosed bool
}

func (w *testCloseWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w *testCloseWriter) Close() error {
	w.isClosed = true
	return nil
}

// TestClose tests that the writer does not close the underlying
// io.Writer, unless .closeDownstream is set.
func TestClose(t *testing.T) {
	for _, doClose := range []bool{true, false} {
		w := &testCloseWriter{}
		out, err := NewWriter(w, V1_7, nil)
		if err != nil {
			t.Fatal(err)
		}
		out.closeOrigW = doClose

		out.GetMeta().Catalog.Pages = out.Alloc() // pretend we have pages

		err = out.Close()
		if err != nil {
			t.Fatal(err)
		}

		if doClose != w.isClosed {
			t.Errorf("expected %v, got %v", doClose, w.isClosed)
		}
	}
}

func TestWriterGet(t *testing.T) {
	for _, name := range []string{"direct", "objStm"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, name+".pdf")
			w, err := Create(file, V2_0, nil)
			if err != nil {
				t.Fatal(err)
			}

			testObjects := []Object{
				nil,
				Array{Integer(1), Integer(2), Integer(3), NewReference(999, 1)},
				Boolean(true),
				Dict{
					"foo": Integer(1),
					"bar": Integer(2),
					"baz": Name("3"),
				},
				Integer(42),
				Name("test"),
				Real(3.14),
				String("test"),
			}
			refs := make([]Reference, len(testObjects))
			for i := range len(testObjects) {
				refs[i] = w.Alloc()
			}

			// write test objects
			if name == "direct" {
				for i, obj := range testObjects {
					err := w.Put(refs[i], obj)
					if err != nil {
						t.Fatal(err)
					}
				}
			} else {
				err = w.WriteCompressed(refs, testObjects...)
				if err != nil {
					t.Fatal(err)
				}
			}

			// Check whether the write position is still correct after reading the
			// first object. If the write position is not correct, the extra object
			// will overwrite some or all of the objects.
			_, err = w.Get(refs[0], true)
			if err != nil {
				t.Fatal(err)
			}
			extraRef := w.Alloc()
			extraObj := Name(strings.Repeat("x", 1000))
			err = w.Put(extraRef, extraObj)
			if err != nil {
				t.Fatal(err)
			}

			// read back the objects
			for i, ref := range refs {
				obj, err := w.Get(ref, true)
				if err != nil {
					t.Fatalf("error reading object %d: %v", i, err)
				}
				if !reflect.DeepEqual(obj, testObjects[i]) {
					t.Errorf("expected %v, got %v", testObjects[i], obj)
				}
			}
			obj, err := GetName(w, extraRef)
			if err != nil {
				t.Errorf("error reading extra object: %v", err)
			} else if obj != extraObj {
				t.Errorf("expected %v, got %v", extraObj, obj)
			}

			err = w.Close()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWriter_ID(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = w.Alloc() // pretend we have pages
	w.GetMeta().ID = [][]byte{[]byte("0123456789abcdef"), []byte("0123456789ABCDEF")}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.meta.ID, w.meta.ID) {
		t.Errorf("expected %v, got %v", w.meta.ID, r.meta.ID)
	}
}
