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
	for _, cipher := range []cipherType{cipherRC4, cipherAES} {
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
	for _, cipher := range []cipherType{cipherRC4, cipherAES} {
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
