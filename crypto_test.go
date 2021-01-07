package pdf

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestComputeOU(t *testing.T) {
	passwd := "test"
	P := -4
	sec := &StandardSecurityHandler{
		P: uint32(P),
		id: []byte{0xac, 0xac, 0x29, 0xb4, 0x19, 0x2f, 0xd9, 0x23,
			0xc2, 0x4f, 0xe6, 0x04, 0x24, 0x79, 0xb2, 0xa9},
		R: 4,
		n: 16,
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
		t.Fatal("wrong U value")
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
			pwdPos := -1
			lastPwd := ""

			sec := NewSecurityHandler([]byte("0123456789ABCDEF"),
				test.user, test.owner, ^uint32(4))
			key := sec.key

			// deauthenticate the security handler
			sec.key = nil
			sec.OwnerAuthenticated = false

			sec.getPasswd = func(_ bool) string {
				candidate := ""
				pwdPos++
				if pwdPos < len(pwds) {
					candidate = pwds[pwdPos]
				}
				lastPwd = candidate
				return candidate
			}

			computedKey, err := sec.GetKey(false)
			if err != nil && err != ErrWrongPassword {
				t.Errorf("wrong error: %s", err)
				continue
			}
			if test.user != "" && len(pwds) < 2 {
				// need password, and only the wrong one supplied
				if err != ErrWrongPassword {
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

func TestCrypto(t *testing.T) {
	// fname := "PDF32000_2008.pdf"
	fname := "encrypted.pdf"

	fd, err := os.Open(fname)
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()

	fi, err := fd.Stat()
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewReader(fd, fi.Size(), nil)
	if err != nil {
		t.Fatal(err)
	}

	catalog, err := r.GetDict(r.Trailer["Root"])
	if err != nil {
		t.Fatal(err)
	}

	_ = catalog
}
