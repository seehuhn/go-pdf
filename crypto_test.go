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
		ID: []byte{0xac, 0xac, 0x29, 0xb4, 0x19, 0x2f, 0xd9, 0x23,
			0xc2, 0x4f, 0xe6, 0x04, 0x24, 0x79, 0xb2, 0xa9},
		R:        4,
		keyBytes: 16,
	}

	padded, err := padPasswd(passwd)
	if err != nil {
		t.Fatal(err)
	}
	O, err := sec.computeO(padded, padded)
	if err != nil {
		t.Fatal(err)
	}
	goodO := "badad1e86442699427116d3e5d5271bc80a27814fc5e80f815efeef839354c5f"
	if fmt.Sprintf("%x", O) != goodO {
		t.Fatal("wrong O value")
	}
	sec.O = O

	pw, err := padPasswd(passwd)
	if err != nil {
		t.Fatal(err)
	}
	enc := sec.computeFileEncyptionKey(pw)
	U := sec.computeU(enc)
	goodU := "a5b5fc1fcc399c6845fedcdfac82027c00000000000000000000000000000000"
	if fmt.Sprintf("%x", U) != goodU {
		t.Fatalf("wrong U value:\n  %x\n  %s", U, goodU)
	}
}

func (sec *stdSecHandler) deauthenticate() {
	sec.key = nil
	sec.ownerAuthenticated = false
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
				UserPermissions: PermCopy,
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
			pwdList = append(pwdList, "friend", "owner")
			pwdFunc := func([]byte, int) string {
				if len(pwdList) == 0 {
					return ""
				}
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
				if len(pwdList) != 2 {
					t.Error("wrong user password used", i)
				}
			}
			if r.enc.sec.ownerAuthenticated {
				t.Fatal("owner wrongly authenticated")
			}
			err = r.AuthenticateOwner()
			if err != nil {
				t.Error(err, "PDF-"+ver.String(), i, userFirst)
				continue
			}
			if !r.enc.sec.ownerAuthenticated {
				t.Fatal("owner not authenticated")
			}
			if len(pwdList) != 0 {
				t.Error("wrong owner password used", i, userFirst, pwdList)
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
			sec, err := createStdSecHandler(id, test.user, test.owner, PermModify, 128, 4)
			if err != nil {
				t.Fatal(err)
			}
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

			if (lastPwd == test.owner) != sec.ownerAuthenticated {
				t.Errorf("%d.%d: wrong value for .OwnerAuthenticated"+
					" (%q %q %t)",
					i, j, lastPwd, test.owner, sec.ownerAuthenticated)
			}
		}
	}
}

func TestAuth2(t *testing.T) {
	id := []byte{0xfb, 0xa6, 0x25, 0xd9, 0xcd, 0xfb, 0x88, 0x11,
		0x9a, 0xd5, 0xa0, 0x94, 0x33, 0x68, 0x42, 0x95}
	sec, err := createStdSecHandler(id, "", "test", PermCopy, 128, 4)
	if err != nil {
		t.Fatal(err)
	}

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

func TestAuth3(t *testing.T) {
	id := []byte{0x3d, 0xe8, 0x0b, 0x6f, 0x8a, 0x2c, 0xd4, 0x79,
		0x54, 0xae, 0x62, 0x91, 0x17, 0xf0, 0x7e, 0xc8}
	cases := []struct {
		perm Perm
		V    int
		R    int
	}{
		{PermAll, 1, 2},
		{PermPrintDegraded, 1, 3},
		{PermCopy, 4, 4},
		{PermCopy, 5, 6},
	}
	const userPasswd = "secret"
	const ownerPasswd = "geheim"
	for _, test := range cases {
		var L int
		switch test.V {
		case 1:
			L = 40
		case 4:
			L = 128
		case 5:
			L = 256
		default:
			t.Fatalf("unsupported V: %d", test.V)
		}
		sec, err := createStdSecHandler(id, userPasswd, ownerPasswd, test.perm, L, test.V)
		if err != nil {
			t.Fatal(err)
		}
		if sec.R != test.R {
			t.Errorf("wrong R: %d != %d", sec.R, test.R)
		}

		if sec.R < 6 {
			// test 1: the user password works
			sec.deauthenticate()
			padded, err := padPasswd(userPasswd)
			if err != nil {
				t.Fatal(err)
			}
			err = sec.authenticateUser(padded)
			if err != nil {
				t.Error(err)
			} else if sec.key == nil {
				t.Error("key not set")
			} else if sec.ownerAuthenticated {
				t.Error("ownerAuthenticated true")
			}

			// test 2: the owner password works
			sec.deauthenticate()
			padded, err = padPasswd(ownerPasswd)
			if err != nil {
				t.Fatal(err)
			}
			err = sec.authenticateOwner(padded)
			if err != nil {
				t.Error(err)
			} else if sec.key == nil {
				t.Error("key not set")
			} else if !sec.ownerAuthenticated {
				t.Error("ownerAuthenticated false")
			}

			// test 3: the user password does not authenticate the owner
			sec.deauthenticate()
			padded, err = padPasswd(userPasswd)
			if err != nil {
				t.Fatal(err)
			}
			err = sec.authenticateOwner(padded)
			if err == nil || sec.key != nil || sec.ownerAuthenticated {
				t.Error("wrong password accepted")
			}
			if _, ok := err.(*AuthenticationError); !ok {
				t.Error("wrong error", err)
			}
		} else {
			// test 1: the user password works
			sec.deauthenticate()
			padded, err := utf8Passwd(userPasswd)
			if err != nil {
				t.Fatal(err)
			}
			err = sec.authenticateUser6(padded)
			if err != nil {
				t.Error(err)
			} else if sec.key == nil {
				t.Error("key not set")
			} else if sec.ownerAuthenticated {
				t.Error("ownerAuthenticated true")
			}

			// test 2: the owner password works
			sec.deauthenticate()
			padded, err = utf8Passwd(ownerPasswd)
			if err != nil {
				t.Fatal(err)
			}
			err = sec.authenticateOwner6(padded)
			if err != nil {
				t.Error(err)
			} else if sec.key == nil {
				t.Error("key not set")
			} else if !sec.ownerAuthenticated {
				t.Error("ownerAuthenticated false")
			}

			// test 3: the user password does not authenticate the owner
			sec.deauthenticate()
			padded, err = utf8Passwd(userPasswd)
			if err != nil {
				t.Fatal(err)
			}
			err = sec.authenticateOwner6(padded)
			if err == nil || sec.key != nil || sec.ownerAuthenticated {
				t.Error("wrong password accepted")
			}
			if _, ok := err.(*AuthenticationError); !ok {
				t.Error("wrong error", err)
			}
		}
	}
}

func TestEncryptBytes(t *testing.T) {
	id := []byte("0123456789ABCDEF")
	for _, cipher := range []cipherType{cipherRC4, cipherAES} {
		for length := 40; length <= 128; length += 8 {
			ref := NewReference(1, 2)
			for _, msg := range []string{"", "pssst!!!", "0123456789ABCDE",
				"0123456789ABCDEF", "0123456789ABCDEF0"} {
				sec, err := createStdSecHandler(id, "secret", "supersecret", PermPrint, 128, 4)
				if err != nil {
					t.Fatal(err)
				}
				enc := encryptInfo{
					strF: &cryptFilter{Cipher: cipher, Length: length},
					sec:  sec,
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
	for _, cipher := range []cipherType{cipherRC4, cipherAES} {
		for length := 40; length <= 128; length += 8 {
			ref := NewReference(1, 2)
			for _, msg := range []string{"", "pssst!!!", "0123456789ABCDE",
				"0123456789ABCDEF", "0123456789ABCDEF0"} {
				sec, err := createStdSecHandler(id, "secret", "supersecret", PermAll, 128, 4)
				if err != nil {
					t.Fatal(err)
				}
				enc := encryptInfo{
					stmF: &cryptFilter{Cipher: cipher, Length: 128},
					sec:  sec,
				}

				buf := &bytes.Buffer{}
				w, err := enc.EncryptStream(ref, withDummyClose{buf})
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
