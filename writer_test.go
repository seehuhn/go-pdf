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
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestWriter(t *testing.T) {
	out := &bytes.Buffer{}

	opt := &WriterOptions{
		ID:             [][]byte{},
		OwnerPassword:  "test",
		UserPermission: PermCopy,
	}
	w, err := NewWriter(out, opt)
	if err != nil {
		t.Fatal(err)
	}
	encInfo1 := format(w.w.enc.ToDict())

	author := "Jochen Voß"
	w.SetInfo(&Info{
		Title:        "PDF Test Document",
		Author:       author,
		Subject:      "Testing",
		Keywords:     "PDF, testing, Go",
		CreationDate: time.Now(),
	})

	refs, err := w.WriteCompressed(nil,
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

	stream, contentNode, err := w.OpenStream(Dict{}, 0)
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

	page1, err := w.Write(Dict{
		"Type":      Name("Page"),
		"MediaBox":  Array{Integer(0), Integer(0), Integer(200), Integer(100)},
		"Resources": resources,
		"Contents":  contentNode,
		"Parent":    pagesRef,
	}, 0)
	if err != nil {
		t.Fatal(err)
	}

	pages["Kids"] = append(pages["Kids"].(Array), page1)
	pages["Count"] = pages["Count"].(Integer) + 1
	_, err = w.Write(pages, pagesRef)
	if err != nil {
		t.Fatal(err)
	}

	w.Catalog.Pages = pagesRef

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// os.WriteFile("debug.pdf", out.Bytes(), 0o644)

	outR := bytes.NewReader(out.Bytes())
	r, err := NewReader(outR, outR.Size(), nil)
	if err != nil {
		t.Fatal(err)
	}
	encInfo2 := format(r.enc.ToDict())

	if encInfo1 != encInfo2 {
		fmt.Println()
		fmt.Println(encInfo1)
		fmt.Println()
		fmt.Println(encInfo2)
		t.Error("encryption dictionaries differ")
	}

	_, err = r.enc.sec.GetKey(false)
	if err != nil {
		t.Fatal(err)
	}

	info, err := r.GetInfo()
	if err != nil {
		t.Fatal(err)
	}

	if x := info.Author; x != author {
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
		out, err := NewWriter(w, nil)
		if err != nil {
			t.Fatal(err)
		}
		out.closeDownstream = doClose

		out.Catalog.Pages = out.Alloc() // pretend we have pages

		err = out.Close()
		if err != nil {
			t.Fatal(err)
		}

		if doClose != w.isClosed {
			t.Errorf("expected %v, got %v", doClose, w.isClosed)
		}
	}
}

func TestPlaceholder(t *testing.T) {
	const testVal = 12345

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.pdf")

	w, err := Create(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	w.Catalog.Pages = w.Alloc() // pretend we have pages

	length := w.NewPlaceholder(5)
	testRef, err := w.Write(Dict{
		"Test":   Bool(true),
		"Length": length,
	}, 0)
	if err != nil {
		t.Fatal(err)
	}

	if length.ref != 0 {
		t.Error("failed to detect that file is seekable")
	}

	err = length.Set(Integer(testVal))
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// try to read back the file

	r, err := Open(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := r.GetDict(testRef)
	if err != nil {
		t.Fatal(err)
	}

	lengthOut, err := r.GetInt(obj["Length"])
	if err != nil {
		t.Fatal(err)
	}

	if lengthOut != testVal {
		t.Errorf("wrong /Length: %d vs %d", lengthOut, testVal)
	}
}
