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
	w.Catalog.Pages = w.Alloc() // pretend that we have a page tree
	a := w.Alloc()
	b := w.Alloc()
	_, err = w.Write(b, a)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write(Integer(42), b)
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
	x, err := r.Resolve(a)
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
	w.Catalog.Pages = w.Alloc() // pretend that we have a page tree
	a := w.Alloc()
	b := w.Alloc()
	_, err = w.Write(b, a)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write(a, b)
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
	_, err = r.Resolve(a)
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

			ref, err := w.Write(TextString(msg), 0)
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
				dec, err := r.GetString(ref)
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
	w.Catalog.Pages = w.Alloc() // pretend that we have a page tree

	refs := make([]Reference, 9)
	objs := make([]Object, len(refs))
	for i := range refs {
		refs[i] = w.Alloc()
		objs[i] = Name("obj" + strconv.Itoa(i))
	}

	_, err = w.Write(objs[1], refs[1])
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.WriteCompressed([]Reference{refs[0], refs[3], refs[6]},
		objs[0], objs[3], objs[6])
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write(objs[4], refs[4])
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.WriteCompressed([]Reference{refs[2], refs[5], refs[8]},
		objs[2], objs[5], objs[8])
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write(objs[7], refs[7])
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
		obj, err := r.Resolve(ref)
		if err != nil {
			t.Fatal(err)
		}
		if obj != objs[i] {
			t.Errorf("%d: got %s, want %s", i, obj, objs[i])
		}
	}
}
