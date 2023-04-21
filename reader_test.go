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
	"io"
	"strconv"
	"strings"
	"testing"
)

func TestReferenceChain(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = addEmptyPage(w)
	if err != nil {
		t.Fatal(err)
	}

	a := w.Alloc()
	b := w.Alloc()
	err = w.Put(a, b)
	if err != nil {
		t.Fatal(err)
	}
	err = w.Put(b, Integer(42))
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	// err = os.WriteFile("test_ReferenceChain.pdf", buf.Bytes(), 0o666)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	r, err := NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	x, err := Resolve(r, a)
	if err != nil {
		t.Fatal(err)
	}
	if x != Integer(42) {
		t.Errorf("got %v, want 42", x)
	}
}

func TestReferenceLoop(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = addEmptyPage(w)
	if err != nil {
		t.Fatal(err)
	}

	a := w.Alloc()
	b := w.Alloc()
	err = w.Put(a, b)
	if err != nil {
		t.Fatal(err)
	}
	err = w.Put(b, a)
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	// err = os.WriteFile("test_ReferenceLoop.pdf", buf.Bytes(), 0o666)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	r, err := NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Resolve(r, a)
	if err == nil {
		t.Error("reference loop not detected")
	}
}

func TestIndirectStreamLength(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = addEmptyPage(w)
	if err != nil {
		t.Fatal(err)
	}

	sLength := w.Alloc()
	sDict := Dict{
		"Length": sLength,
	}
	sRef := w.Alloc()
	s, err := w.OpenStream(sRef, sDict)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Write([]byte("123456"))
	if err != nil {
		t.Fatal(err)
	}
	err = s.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = w.Put(sLength, Integer(6))
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	// err = os.WriteFile("test_IndirectStreamLength.pdf", buf.Bytes(), 0o666)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	r, err := NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	sObj, err := GetStream(r, sRef)
	if err != nil {
		t.Fatal(err)
	}
	if sObj.Dict["Length"] != sLength {
		t.Errorf("wrong stream length: got %v, want %v", sObj.Dict["Length"], sLength)
	}
	sData, err := io.ReadAll(sObj.R)
	if err != nil {
		t.Fatal(err)
	}
	if string(sData) != "123456" {
		t.Errorf("wrong stream data: got %q, want %q", sData, "123456")
	}
}

func TestStreamLengthInStream(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = addEmptyPage(w)
	if err != nil {
		t.Fatal(err)
	}
	sLength := w.Alloc()
	sDict := Dict{
		"Length": sLength,
	}
	sRef := w.Alloc()
	s, err := w.OpenStream(sRef, sDict)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Write([]byte("123456"))
	if err != nil {
		t.Fatal(err)
	}
	err = s.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = w.WriteCompressed([]Reference{sLength}, Integer(6))
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	// err = os.WriteFile("test_StreamLengthInStream.pdf", buf.Bytes(), 0o666)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	r, err := NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	sObj, err := GetStream(r, sRef)
	if err != nil {
		t.Fatal(err)
	}
	if sObj.Dict["Length"] != sLength {
		t.Errorf("wrong stream length: got %v, want %v", sObj.Dict["Length"], sLength)
	}
	sData, err := io.ReadAll(sObj.R)
	if err != nil {
		t.Fatal(err)
	}
	if string(sData) != "123456" {
		t.Errorf("wrong stream data: got %q, want %q", sData, "123456")
	}
}

func TestStreamLengthCycle(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	sLength := w.Alloc()
	sDict := Dict{
		"Length": sLength,
	}
	sRef := w.Alloc()
	s, err := w.OpenStream(sRef, sDict)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Write([]byte("123456"))
	if err != nil {
		t.Fatal(err)
	}
	err = s.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = w.Put(sLength, sLength) // infinite reference cycle
	if err != nil {
		t.Fatal(err)
	}
	err = addEmptyPage(w, Name("Contents"), sRef)
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	// err = os.WriteFile("test_StreamLengthCycle.pdf", buf.Bytes(), 0o666)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	r, err := NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = GetStream(r, sRef)
	if err == nil {
		t.Error("reference loop not detected")
	}
}

func TestStreamLengthCycle2(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	xRef := w.Alloc()
	err = addEmptyPage(w, Name("Rotate"), xRef)
	if err != nil {
		t.Fatal(err)
	}
	// Manually construct two object streams, so that the length of each
	// stream is contained in the other stream.
	L1 := w.Alloc()
	L2 := w.Alloc()
	sDict1 := Dict{
		"Length": L2,
		"Type":   Name("ObjStm"),
		"N":      Integer(2),
		"First":  Integer(8),
	}
	sRef1 := w.Alloc()
	s1, err := w.OpenStream(sRef1, sDict1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s1.Write([]byte(fmt.Sprintf("%d 0\n%d 2\n6\n90",
		L1.Number(), xRef.Number())))
	if err != nil {
		t.Fatal(err)
	}
	err = s1.Close()
	if err != nil {
		t.Fatal(err)
	}
	sDict2 := Dict{
		"Length": L1,
		"Type":   Name("ObjStm"),
		"N":      Integer(1),
		"First":  Integer(4),
	}
	sRef2 := w.Alloc()
	s2, err := w.OpenStream(sRef2, sDict2)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s2.Write([]byte(fmt.Sprintf("%d 0\n12", L2.Number())))
	if err != nil {
		t.Fatal(err)
	}
	err = s2.Close()
	if err != nil {
		t.Fatal(err)
	}
	w.xref[L2.Number()] = &xRefEntry{
		InStream: sRef2,
		Pos:      0,
	}
	w.xref[L1.Number()] = &xRefEntry{
		InStream: sRef1,
		Pos:      0,
	}
	w.xref[xRef.Number()] = &xRefEntry{
		InStream: sRef1,
		Pos:      1,
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	// err = os.WriteFile("test_StreamLengthCycle2.pdf", buf.Bytes(), 0o666)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	r, err := NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = GetStream(r, xRef)
	if err == nil {
		t.Error("reference loop not detected")
	}
}

func TestAuthentication(t *testing.T) {
	msg := "super sécret"
	for i, ver := range []Version{V1_6, V1_4, V1_3} {
		for _, userFirst := range []bool{true, false} {
			out := &bytes.Buffer{}

			opt := &WriterOptions{
				Version:        ver,
				UserPassword:   "user",
				OwnerPassword:  "owner",
				UserPermission: PermAll,
			}
			w, err := NewWriter(out, opt)
			if err != nil {
				t.Fatal(err)
			}

			ref := w.Alloc()
			err = w.Put(ref, TextString(msg))
			if err != nil {
				t.Fatal(err)
			}

			w.Catalog.Pages = w.Alloc()

			err = w.Close()
			if err != nil {
				t.Fatal(err)
			}

			// ---------------------------------------------------------
			// os.WriteFile(fmt.Sprintf("xxx%d.pdf", i), out.Bytes(), 0o666)
			// ---------------------------------------------------------

			var pwdList []string
			if userFirst {
				pwdList = append(pwdList, "don't know", "user")
			}
			pwdList = append(pwdList, "friend", "owner")
			pwdFunc := func([]byte, int) string {
				res := pwdList[0]
				pwdList = pwdList[1:]
				return res
			}

			in := bytes.NewReader(out.Bytes())
			rOpt := &ReaderOptions{
				ReadPassword: pwdFunc,
			}
			r, err := NewReader(in, rOpt)
			if err != nil {
				t.Fatal(err, i)
			}
			if userFirst {
				dec, err := GetString(r, ref)
				if err != nil {
					t.Fatal(err, i)
				}
				if dec.AsTextString() != msg {
					t.Error("got wrong message", i)
				}
				if len(pwdList) != 2 {
					t.Error("wrong user password used", i)
				}
			}
			if r.enc.sec.OwnerAuthenticated {
				t.Fatal("owner wrongly authenticated")
			}
			err = r.AuthenticateOwner()
			if err != nil {
				t.Fatal(err, i)
			}
			if !r.enc.sec.OwnerAuthenticated {
				t.Fatal("owner not authenticated")
			}
			if len(pwdList) != 0 {
				t.Error("wrong owner password used", i)
			}
		}
	}
}

func TestReaderGoFuzz(t *testing.T) {
	// found by go-fuzz - check that the reader doesn't panic
	cases := []string{
		"%PDF-1.0\n0 0obj<startxref8",
		"%PDF-1.0\n0 0obj(startxref8",
		"%PDF-1.0\n0 0obj<</Length -40>>stream\nstartxref8\n",
		"%PDF-1.0\n0 0obj<</ 0 0%startxref8",
	}
	for _, test := range cases {
		buf := strings.NewReader(test)
		_, _ = NewReader(buf, nil)
	}
}

func TestObjectStream(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = addEmptyPage(w)
	if err != nil {
		t.Fatal(err)
	}

	refs := make([]Reference, 9)
	objs := make([]Object, len(refs))
	for i := range refs {
		refs[i] = w.Alloc()
		objs[i] = Name("obj" + strconv.Itoa(i))
	}

	err = w.Put(refs[1], objs[1])
	if err != nil {
		t.Fatal(err)
	}
	err = w.WriteCompressed([]Reference{refs[0], refs[3], refs[6]},
		objs[0], objs[3], objs[6])
	if err != nil {
		t.Fatal(err)
	}
	err = w.Put(refs[4], objs[4])
	if err != nil {
		t.Fatal(err)
	}
	err = w.WriteCompressed([]Reference{refs[2], refs[5], refs[8]},
		objs[2], objs[5], objs[8])
	if err != nil {
		t.Fatal(err)
	}
	err = w.Put(refs[7], objs[7])
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}

	for i, ref := range refs {
		obj, err := Resolve(r, ref)
		if err != nil {
			t.Fatal(err)
		}
		if obj != objs[i] {
			t.Errorf("%d: got %s, want %s", i, obj, objs[i])
		}
	}
}

func addEmptyPage(w *Writer, args ...Object) error {
	pRef := w.Alloc()
	ppRef := w.Alloc()
	pageDict := Dict{
		"Type":      Name("Page"),
		"Parent":    ppRef,
		"Resources": Dict{},
		"MediaBox":  &Rectangle{URx: 100, URy: 100},
	}
	for i := 0; i < len(args); i += 2 {
		pageDict[args[i].(Name)] = args[i+1]
	}
	err := w.Put(pRef, pageDict)
	if err != nil {
		return err
	}
	err = w.Put(ppRef, Dict{
		"Type":  Name("Pages"),
		"Kids":  Array{pRef},
		"Count": Integer(1),
	})
	if err != nil {
		return err
	}
	w.Catalog.Pages = ppRef
	return nil
}
