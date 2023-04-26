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
	"testing"
)

func TestComputeOU(t *testing.T) {
	passwd := "test"
	P := -4
	sec := &stdSecHandler{
		P: uint32(P),
		id: []byte{0xac, 0xac, 0x29, 0xb4, 0x19, 0x2f, 0xd9, 0x23,
			0xc2, 0x4f, 0xe6, 0x04, 0x24, 0x79, 0xb2, 0xa9},
		R:        4,
		KeyBytes: 16,
	}

	O := sec.computeO(passwd, "")
	goodO := "badad1e86442699427116d3e5d5271bc80a27814fc5e80f815efeef839354c5f"
	if fmt.Sprintf("%x", O) != goodO {
		t.Fatal("wrong O value")
	}
	sec.o = O

	U := make([]byte, 32)
	pw := padPasswd(passwd)
	key := sec.computeKey(nil, pw)
	U = sec.computeU(U, key)
	goodU := "a5b5fc1fcc399c6845fedcdfac82027c00000000000000000000000000000000"
	if fmt.Sprintf("%x", U) != goodU {
		t.Fatalf("wrong U value:\n  %x\n  %s", U, goodU)
	}
}

// TODO(voss): remove?
func (sec *stdSecHandler) deauthenticate() {
	sec.key = nil
	sec.OwnerAuthenticated = false
}

func TestCryptV1(t *testing.T) {
	opt := &WriterOptions{
		Version:       V1_1,
		UserPassword:  "AA",
		OwnerPassword: "BB",
		// UserPermissions: PermAll,
	}
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, opt)
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	contentsRef := w.Alloc()
	s, err := w.OpenStream(contentsRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Write([]byte("0 0 m 100 100 l s"))
	err = s.Close()
	if err != nil {
		t.Fatal(err)
	}
	addPage(w, Name("Contents"), contentsRef)
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// os.WriteFile("test_TestV1.pdf", buf.Bytes(), 0o666)

	in := bytes.NewReader(buf.Bytes())
	pwdFunc := func(_ []byte, try int) string {
		switch try {
		case 0:
			return "BB"
		default:
			return ""
		}
	}
	rOpt := &ReaderOptions{
		ReadPassword: pwdFunc,
	}
	r, err := NewReader(in, rOpt)
	if err != nil {
		t.Fatal(err)
	}
	err = r.AuthenticateOwner()
	if err != nil {
		t.Error(err)
	}
}

func TestAuthentication(t *testing.T) {
	msg := "super secret"
	for i, ver := range []Version{V1_6, V1_4, V1_3, V1_1} {
		for _, userFirst := range []bool{true, false} {
			buf := &bytes.Buffer{}

			opt := &WriterOptions{
				Version:         ver,
				UserPassword:    "user",
				OwnerPassword:   "owner",
				UserPermissions: PermAll,
			}
			w, err := NewWriter(buf, opt)
			if err != nil {
				t.Fatal(err)
			}

			contentsRef := w.Alloc()
			s, err := w.OpenStream(contentsRef, nil)
			if err != nil {
				t.Fatal(err)
			}
			s.Write([]byte("0 0 m 100 100 l s"))
			err = s.Close()
			if err != nil {
				t.Fatal(err)
			}
			addPage(w, Name("Contents"), contentsRef)

			ref := w.Alloc()
			err = w.Put(ref, TextString(msg))
			if err != nil {
				t.Fatal(err)
			}

			err = w.Close()
			if err != nil {
				t.Fatal(err)
			}

			// os.WriteFile(fmt.Sprintf("xxx%d.pdf", i), buf.Bytes(), 0o666)

			var pwdList []string
			if userFirst {
				pwdList = append(pwdList, "don't know", "user")
			}
			pwdList = append(pwdList, "friend", "owner", "")
			pwdFunc := func([]byte, int) string {
				res := pwdList[0]
				pwdList = pwdList[1:]
				return res
			}

			in := bytes.NewReader(buf.Bytes())
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
					t.Error(err, i, userFirst)
					continue
				}
				if dec.AsTextString() != msg {
					t.Error("got wrong message", i)
				}
				if len(pwdList) != 3 {
					t.Error("wrong user password used", i)
				}
			}
			if r.enc.sec.OwnerAuthenticated {
				t.Fatal("owner wrongly authenticated")
			}
			err = r.AuthenticateOwner()
			if err != nil {
				t.Error(err, "PDF-"+ver.String(), i, userFirst)
				continue
			}
			if !r.enc.sec.OwnerAuthenticated {
				t.Fatal("owner not authenticated")
			}
			if len(pwdList) != 1 {
				t.Error("wrong owner password used", i, userFirst)
			}
		}
	}
}

func TestAuth(t *testing.T) {
	cases := []struct {
		user, owner string
	}{
		{"", ""},
		{"", "owner"},
		{"user", "owner"},
		{"secret", "secret"},
	}
	for i, test := range cases {
		trials := [][]string{
			{"wrong"},
			{"wrong", test.user},
			{"wrong", test.owner},
		}
		for j, pwds := range trials {
			id := []byte("0123456789ABCDEF")
			sec := createStdSecHandler(id, test.user, test.owner, PermModify, 4)
			key := sec.key

			sec.deauthenticate()

			pwdPos := -1
			lastPwd := ""
			sec.readPwd = func([]byte, int) string {
				candidate := ""
				pwdPos++
				if pwdPos < len(pwds) {
					candidate = pwds[pwdPos]
				}
				lastPwd = candidate
				return candidate
			}

			computedKey, err := sec.GetKey(false)
			if _, authErr := err.(*AuthenticationError); err != nil && !authErr {
				t.Errorf("wrong error: %s", err)
				continue
			}
			if test.user != "" && len(pwds) < 2 {
				// need password, and only the wrong one supplied
				if _, authErr := err.(*AuthenticationError); !authErr {
					t.Error("wrong password not detected")
				} else if pwdPos < len(pwds) {
					t.Error("not all passwords tried")
				}
				continue
			} else if err != nil {
				t.Errorf("%d.%d: unexpected error: %s", i, j, err)
				continue
			}

			if !bytes.Equal(key, computedKey) {
				t.Errorf("wrong key")
			}

			if (lastPwd == test.owner) != sec.OwnerAuthenticated {
				t.Errorf("%d.%d: wrong value for .OwnerAuthenticated"+
					" (%q %q %t)",
					i, j, lastPwd, test.owner, sec.OwnerAuthenticated)
			}
		}
	}
}

func TestAuth2(t *testing.T) {
	id := []byte{0xfb, 0xa6, 0x25, 0xd9, 0xcd, 0xfb, 0x88, 0x11,
		0x9a, 0xd5, 0xa0, 0x94, 0x33, 0x68, 0x42, 0x95}
	sec := createStdSecHandler(id, "", "test", PermCopy, 4)

	key, err := sec.GetKey(false)
	if err != nil {
		t.Fatal(err)
	}
	sec.deauthenticate()

	key2, err := sec.GetKey(false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, key2) {
		t.Error("wrong key")
	}
}

func TestEncryptBytes(t *testing.T) {
	id := []byte("0123456789ABCDEF")
	for _, cipher := range []cipherType{cipherRC4, cipherAES128} {
		for length := 40; length <= 128; length += 8 {
			ref := NewReference(1, 2)
			for _, msg := range []string{"", "pssst!!!", "0123456789ABCDE",
				"0123456789ABCDEF", "0123456789ABCDEF0"} {
				enc := encryptInfo{
					strF: &cryptFilter{Cipher: cipher, Length: length},
					sec:  createStdSecHandler(id, "secret", "supersecret", PermPrint, 4),
				}

				plainText := []byte(msg)
				// fmt.Printf("%x\n", plainText)
				cipherText, err := enc.EncryptBytes(ref, plainText)
				if err != nil {
					t.Fatal(err)
				}
				// fmt.Printf("%x\n", cipherText)
				restored, err := enc.DecryptBytes(ref, cipherText)
				if err != nil {
					t.Fatal(err)
				}
				// fmt.Printf("%x\n", restored)
				if string(restored) != msg {
					t.Error("round-trip failed")
				}
			}
		}
	}
}

func TestEncryptStream(t *testing.T) {
	id := []byte("0123456789ABCDEF")
	for _, cipher := range []cipherType{cipherRC4, cipherAES128} {
		for length := 40; length <= 128; length += 8 {
			ref := NewReference(1, 2)
			for _, msg := range []string{"", "pssst!!!", "0123456789ABCDE",
				"0123456789ABCDEF", "0123456789ABCDEF0"} {
				enc := encryptInfo{
					stmF: &cryptFilter{Cipher: cipher, Length: 128},
					sec:  createStdSecHandler(id, "secret", "supersecret", PermAll, 4),
				}

				buf := &bytes.Buffer{}
				w, err := enc.cryptFilter(ref, withDummyClose{buf})
				if err != nil {
					t.Fatal(err)
				}
				_, err = w.Write([]byte(msg))
				if err != nil {
					t.Fatal(err)
				}
				err = w.Close()
				if err != nil {
					t.Fatal(err)
				}

				restored, err := enc.DecryptStream(ref, buf)
				if err != nil {
					t.Fatal(err)
				}
				res, err := io.ReadAll(restored)
				if err != nil {
					t.Fatal(err)
				}
				// fmt.Printf("%x\n", res)
				if string(res) != msg {
					t.Error("round-trip failed")
				}
			}
		}
	}
}

func TestPerm(t *testing.T) {
	// We iterate over all combinations of bits
	// 3, 4, 5, 6, 9, 11, and 12 (1-based).
	for b := uint32(0); b < 127; b++ {
		// bit in b -> bit in P
		//       0  ->  3-1 = 2
		//       1  ->  4-1 = 3
		//       2  ->  5-1 = 4
		//       3  ->  6-1 = 5
		//       4  ->  9-1 = 8
		//       5  -> 11-1 = 10
		//       6  -> 12-1 = 11
		var P uint32 = 0b11111111_11111111_11110010_11000000
		P |= (b&15)<<2 | (b&16)<<4 | (b&96)<<5

		perm := stdSecPToPerm(3, P)

		if perm&PermPrint != 0 && perm&PermPrintDegraded == 0 {
			t.Error("print permission without degraded print permission")
		}
		if perm&PermAnnotate != 0 && perm&PermForms == 0 {
			t.Error("annotate permission without forms permission")
		}
		if perm&PermModify != 0 && perm&PermAssemble == 0 {
			t.Error("modify permission without assemble permission")
		}

		// Remove some combinations which make no sense, e.g. full print
		// permission without degraded print permission.
		if P&(1<<(4-1)) != 0 && P&(1<<(11-1)) == 0 {
			continue
		}
		if P&(1<<(6-1)) != 0 && P&(1<<(9-1)) == 0 {
			continue
		}
		if P&(1<<(12-1)) != 0 && P&(1<<(3-1)) == 0 {
			continue
		}

		P2 := stdSecPermToP(perm)
		if P != P2 {
			mask := uint32(0b00001111_11111111)
			t.Errorf("perm=%07b P1=%012b P2=%012b diff=%012b",
				perm, P&mask, P2&mask, P^P2)
		}
	}
}
