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
	O := sec.computeO(padded, padded)
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

func TestCryptV1(t *testing.T) {
	opt := &WriterOptions{
		UserPassword:  "AA",
		OwnerPassword: "BB",
	}
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_1, opt)
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

	inData := buf.Bytes()
	in := bytes.NewReader(inData)
	rOpt := &ReaderOptions{
		Password: "BB",
	}
	r, err := NewReader(in, int64(len(inData)), rOpt)
	if err != nil {
		t.Fatal(err)
	}
	if r.GetMeta().Permissions != PermAll {
		t.Error("expected owner permissions")
	}
}

func TestAuthentication(t *testing.T) {
	msg := TextString("super secret")
	for _, v := range []Version{V1_6, V1_4, V1_3, V1_1} {
		t.Run("PDF-"+v.String(), func(t *testing.T) {
			buf := &bytes.Buffer{}

			opt := &WriterOptions{
				UserPassword:    "user",
				OwnerPassword:   "owner",
				UserPermissions: PermCopy,
			}
			w, err := NewWriter(buf, v, opt)
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
			err = w.Put(ref, msg)
			if err != nil {
				t.Fatal(err)
			}
			err = w.Close()
			if err != nil {
				t.Fatal(err)
			}

			inData := buf.Bytes()

			// open with user password
			t.Run("user", func(t *testing.T) {
				in := bytes.NewReader(inData)
				rOpt := &ReaderOptions{Password: "user"}
				r, err := NewReader(in, int64(len(inData)), rOpt)
				if err != nil {
					t.Fatal(err)
				}
				if r.GetMeta().Permissions != PermCopy {
					t.Errorf("expected PermCopy, got %v", r.GetMeta().Permissions)
				}
				dec, err := GetTextString(r, ref)
				if err != nil {
					t.Error(err)
				}
				if dec != msg {
					t.Error("got wrong message")
				}
			})

			// open with owner password
			t.Run("owner", func(t *testing.T) {
				in := bytes.NewReader(inData)
				rOpt := &ReaderOptions{Password: "owner"}
				r, err := NewReader(in, int64(len(inData)), rOpt)
				if err != nil {
					t.Fatal(err)
				}
				if r.GetMeta().Permissions != PermAll {
					t.Errorf("expected PermAll, got %v", r.GetMeta().Permissions)
				}
				dec, err := GetTextString(r, ref)
				if err != nil {
					t.Error(err)
				}
				if dec != msg {
					t.Error("got wrong message")
				}
			})

			// wrong password
			t.Run("wrong", func(t *testing.T) {
				in := bytes.NewReader(inData)
				rOpt := &ReaderOptions{Password: "wrong"}
				_, err := NewReader(in, int64(len(inData)), rOpt)
				if _, ok := err.(*AuthenticationError); !ok {
					t.Errorf("expected AuthenticationError, got %v", err)
				}
			})
		})
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
		id := []byte("0123456789ABCDEF")
		sec, err := createStdSecHandler(id, test.user, test.owner, PermModify, 128, 4, false)
		if err != nil {
			t.Fatal(err)
		}
		key := sec.key

		// wrong password should fail (unless user password is empty)
		sec.key = nil
		_, err = sec.authenticate("wrong")
		if test.user == "" {
			// empty user password means "wrong" will still fail owner,
			// but empty string would succeed user — authenticate("wrong")
			// should fail
			if err == nil {
				t.Errorf("%d: wrong password accepted", i)
			}
		} else {
			if _, authErr := err.(*AuthenticationError); !authErr {
				t.Errorf("%d: expected AuthenticationError, got %v", i, err)
			}
		}

		// user password should work
		sec.key = nil
		perm, err := sec.authenticate(test.user)
		if err != nil {
			t.Errorf("%d: user password failed: %v", i, err)
			continue
		}
		if !bytes.Equal(key, sec.key) {
			t.Errorf("%d: wrong key from user password", i)
		}
		if test.user == test.owner {
			// same password authenticates as owner (tried first)
			if perm != PermAll {
				t.Errorf("%d: expected PermAll for same password", i)
			}
		}

		// owner password should work and give PermAll
		sec.key = nil
		perm, err = sec.authenticate(test.owner)
		if err != nil {
			t.Errorf("%d: owner password failed: %v", i, err)
			continue
		}
		if !bytes.Equal(key, sec.key) {
			t.Errorf("%d: wrong key from owner password", i)
		}
		if perm != PermAll {
			t.Errorf("%d: expected PermAll from owner password", i)
		}
	}
}

func TestAuth2(t *testing.T) {
	id := []byte{0xfb, 0xa6, 0x25, 0xd9, 0xcd, 0xfb, 0x88, 0x11,
		0x9a, 0xd5, 0xa0, 0x94, 0x33, 0x68, 0x42, 0x95}
	sec, err := createStdSecHandler(id, "", "test", PermCopy, 128, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	key := sec.key

	// empty user password should authenticate via authenticate("")
	sec.key = nil
	_, err = sec.authenticate("")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, sec.key) {
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
		sec, err := createStdSecHandler(id, userPasswd, ownerPasswd, test.perm, L, test.V, false)
		if err != nil {
			t.Fatal(err)
		}
		if sec.R != test.R {
			t.Errorf("wrong R: %d != %d", sec.R, test.R)
		}

		if sec.R < 6 {
			// test 1: the user password works
			sec.key = nil
			padded, err := padPasswd(userPasswd)
			if err != nil {
				t.Fatal(err)
			}
			err = sec.authenticateUser(padded)
			if err != nil {
				t.Error(err)
			} else if sec.key == nil {
				t.Error("key not set")
			}

			// test 2: the owner password works
			sec.key = nil
			padded, err = padPasswd(ownerPasswd)
			if err != nil {
				t.Fatal(err)
			}
			err = sec.authenticateOwner(padded)
			if err != nil {
				t.Error(err)
			} else if sec.key == nil {
				t.Error("key not set")
			}

			// test 3: the user password does not authenticate the owner
			sec.key = nil
			padded, err = padPasswd(userPasswd)
			if err != nil {
				t.Fatal(err)
			}
			err = sec.authenticateOwner(padded)
			if err == nil || sec.key != nil {
				t.Error("wrong password accepted")
			}
			if _, ok := err.(*AuthenticationError); !ok {
				t.Error("wrong error", err)
			}
		} else {
			// test 1: the user password works
			sec.key = nil
			padded, err := utf8Passwd(userPasswd)
			if err != nil {
				t.Fatal(err)
			}
			err = sec.authenticateUser6(padded)
			if err != nil {
				t.Error(err)
			} else if sec.key == nil {
				t.Error("key not set")
			}

			// test 2: the owner password works
			sec.key = nil
			padded, err = utf8Passwd(ownerPasswd)
			if err != nil {
				t.Fatal(err)
			}
			err = sec.authenticateOwner6(padded)
			if err != nil {
				t.Error(err)
			} else if sec.key == nil {
				t.Error("key not set")
			}

			// test 3: the user password does not authenticate the owner
			sec.key = nil
			padded, err = utf8Passwd(userPasswd)
			if err != nil {
				t.Fatal(err)
			}
			err = sec.authenticateOwner6(padded)
			if err == nil || sec.key != nil {
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
				sec, err := createStdSecHandler(id, "secret", "supersecret", PermPrint, 128, 4, false)
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
				sec, err := createStdSecHandler(id, "secret", "supersecret", PermAll, 128, 4, false)
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
	for b := range uint32(127) {
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

// TestAuthEmbed tests that encryption information is correctly embedded in the
// PDF file.
func TestAuthEmbed(t *testing.T) {
	ref := NewReference(1, 2)

	opt := &WriterOptions{
		UserPassword:  "A",
		OwnerPassword: "B",
	}
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_7, opt)
	if err != nil {
		t.Fatal(err)
	}
	err = w.Put(ref, TextString("Hello, World!"))
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// opening without a password should fail (user password is non-empty)
	_, err = NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil)
	if _, ok := err.(*AuthenticationError); !ok {
		t.Errorf("expected AuthenticationError, got %v", err)
	}

	// opening with the user password should work
	rOpt := &ReaderOptions{
		Password:      "A",
		ErrorHandling: ErrorHandlingReport,
	}
	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), rOpt)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Get(ref, true)
	if err != nil {
		t.Fatal(err)
	}
}
