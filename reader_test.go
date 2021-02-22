// seehuhn.de/go/pdf - support for reading and writing PDF files
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
	"strings"
	"testing"
)

func TestAuthentication(t *testing.T) {
	msg := "super sécret"
	for i, ver := range []Version{V1_6, V1_4, V1_3} {
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

		ref, err := w.Write(TextString(msg), nil)
		if err != nil {
			t.Fatal(err)
		}

		err = w.SetCatalog(Dict{})
		if err != nil {
			t.Fatal(err)
		}

		err = w.Close()
		if err != nil {
			t.Fatal(err)
		}

		pwdList := []string{"don't know", "user", "friend", "owner"}
		pwdFunc := func() string {
			if len(pwdList) == 0 {
				return ""
			}
			res := pwdList[0]
			pwdList = pwdList[1:]
			return res
		}

		fmt.Println(out.String())

		outR := bytes.NewReader(out.Bytes())
		r, err := NewReader(outR, outR.Size(), pwdFunc)
		if err != nil {
			t.Fatal(err, i)
		}
		dec, err := r.getString(ref)
		if err != nil {
			t.Fatal(err, i)
		}
		if dec.AsTextString() != msg {
			t.Error("got wrong message", i)
		}
		if len(pwdList) != 2 {
			t.Error("wrong user password used", i)
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

func TestReaderGoFuzz(t *testing.T) {
	// found by go-fuzz - check that the code doesn't panic
	cases := []string{
		"%PDF-1.0\n0 0obj<startxref8",
		"%PDF-1.0\n0 0obj(startxref8",
		"%PDF-1.0\n0 0obj<</Length -40>>stream\nstartxref8\n",
		"%PDF-1.0\n0 0obj<</ 0 0%startxref8",
	}
	for _, test := range cases {
		buf := strings.NewReader(test)
		_, _ = NewReader(buf, buf.Size(), nil)
	}
}
