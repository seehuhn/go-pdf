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
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"io"
	"strings"
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
				dec, err := NewCursor(r).TextString(ref)
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
				dec, err := NewCursor(r).TextString(ref)
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

// TestDecryptBytesRejectsBadAESPadding is a regression test for the padding
// oracle in DecryptBytes: the AES branch must reject any ciphertext whose last
// plaintext block does not end in valid PKCS#7 padding, not just any byte in
// 1..16.  Without per-byte verification, flipping the second-to-last byte of
// ciphertext leaves the trailing pad-length byte intact and so historically
// decrypted "successfully" with corrupted plaintext.
func TestDecryptBytesRejectsBadAESPadding(t *testing.T) {
	id := []byte("0123456789ABCDEF")
	ref := NewReference(1, 2)
	sec, err := createStdSecHandler(id, "secret", "supersecret", PermPrint, 128, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	enc := encryptInfo{
		strF: &cryptFilter{Cipher: cipherAES, Length: 128},
		sec:  sec,
	}

	// pick a message whose last plaintext byte is far from 1..16 so that
	// flipping bits in the second-to-last ciphertext byte reliably moves
	// the recovered pad-length byte through invalid values
	plaintext := []byte("0123456789ABCDEF") // 16 bytes -> full extra pad block of 0x10
	cipherText, err := enc.EncryptBytes(ref, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	// in CBC, flipping bit b of ciphertext byte buf[i] flips bit b of
	// plaintext byte pt[i+16].  We flip buf[len-18], which lives in the
	// second-to-last ciphertext block and so corrupts the second-to-last
	// plaintext byte (a pad byte that should equal 0x10) while leaving
	// the final byte (also 0x10, the pad-length indicator) untouched.
	// The old buggy code accepts this because it only inspects the
	// pad-length byte; the new code must reject it.
	for bit := range 8 {
		tampered := append([]byte(nil), cipherText...)
		tampered[len(tampered)-18] ^= byte(1 << bit)

		got, err := enc.DecryptBytes(ref, tampered)
		if err == nil {
			t.Errorf("bit %d: tampered ciphertext decrypted to %x, want error", bit, got)
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
	w.GetMeta().Catalog.Pages = w.Alloc() // pretend we have pages
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

// encDictForR builds an encryption dictionary with correctly-sized entries
// for the given revision, so that the V/R consistency check is exercised
// independently of the other field validations.
func encDictForR(V, R int) Dict {
	ouLen := 32
	if R >= 5 {
		ouLen = 48
	}
	enc := Dict{
		"V": Integer(V),
		"R": Integer(R),
		"O": String(make([]byte, ouLen)),
		"U": String(make([]byte, ouLen)),
		"P": Integer(-4),
	}
	if R >= 5 {
		enc["OE"] = String(make([]byte, 32))
		enc["UE"] = String(make([]byte, 32))
		enc["Perms"] = String(make([]byte, 16))
	}
	return enc
}

func TestMalformedVR(t *testing.T) {
	id := []byte("0123456789ABCDEF")

	// inconsistent V/R combinations must be rejected, never panic
	bad := []struct{ V, R, keyBytes int }{
		{5, 4, 32}, // 256-bit key on the MD5 path
		{5, 3, 32},
		{5, 2, 32},
		{4, 6, 16}, // R=6 without V=5
	}
	for _, test := range bad {
		enc := encDictForR(test.V, test.R)
		_, err := openStdSecHandler(NewCursor(mockGetter), enc, Integer(test.V), test.keyBytes, id)
		if _, ok := err.(*MalformedFileError); !ok {
			t.Errorf("V=%d R=%d: expected MalformedFileError, got %v",
				test.V, test.R, err)
		}
	}

	// valid combinations must still construct
	good := []struct{ V, R, keyBytes int }{
		{4, 4, 16},
		{5, 5, 32}, // deprecated Adobe AES-256 extension
		{5, 6, 32},
	}
	for _, test := range good {
		enc := encDictForR(test.V, test.R)
		if _, err := openStdSecHandler(NewCursor(mockGetter), enc, Integer(test.V), test.keyBytes, id); err != nil {
			t.Errorf("V=%d R=%d: unexpected error: %v", test.V, test.R, err)
		}
	}
}

// buildRawPDF assembles a minimal PDF whose object i+1 has body objs[i], with a
// trailer that references the encryption dictionary as /Encrypt <encRef> 0 R.
func buildRawPDF(objs []string, encRef int) []byte {
	id := "<" + strings.Repeat("00", 16) + ">"
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offs := make([]int, len(objs)+1)
	for i, body := range objs {
		offs[i+1] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	x := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for i := 1; i <= len(objs); i++ {
		fmt.Fprintf(&b, "%010d 00000 n \n", offs[i])
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R /Encrypt %d 0 R "+
		"/ID [%s %s] >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, encRef, id, id, x)
	return b.Bytes()
}

// TestOpenMalformedVR checks that opening a crafted /V 5 /R 4 file returns an
// error instead of panicking in the standard security handler.
func TestOpenMalformedVR(t *testing.T) {
	h32 := "<" + strings.Repeat("00", 32) + ">"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
		"<< /Filter /Standard /V 5 /R 4 /Length 256 /O " + h32 +
			" /U " + h32 + " /P -4 >>",
	}
	data := buildRawPDF(objs, 3)
	if _, err := NewReader(bytes.NewReader(data), int64(len(data)), nil); err == nil {
		t.Error("expected an error opening a /V 5 /R 4 file, got nil")
	}
}

// TestEncryptVTypeCoercion checks that a /V (or /R) written as a Real or as an
// indirect reference is coerced/resolved like any other integer instead of
// panicking the parser.  The crafted dicts have zero O/U, so parsing reaches
// authentication and fails there gracefully.
func TestEncryptVTypeCoercion(t *testing.T) {
	h32 := "<" + strings.Repeat("00", 32) + ">"
	cat := "<< /Type /Catalog /Pages 2 0 R >>"
	pages := "<< /Type /Pages /Kids [] /Count 0 >>"
	for _, tc := range []struct {
		name string
		objs []string
	}{
		{"real V", []string{cat, pages,
			"<< /Filter /Standard /V 4.0 /R 4 /O " + h32 + " /U " + h32 + " /P -4 >>"}},
		{"indirect V", []string{cat, pages,
			"<< /Filter /Standard /V 4 0 R /R 4 /O " + h32 + " /U " + h32 + " /P -4 >>", "4"}},
		{"indirect R", []string{cat, pages,
			"<< /Filter /Standard /V 4 /R 4 0 R /O " + h32 + " /U " + h32 + " /P -4 >>", "4"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := buildRawPDF(tc.objs, 3)
			var err error
			func() {
				defer func() {
					if p := recover(); p != nil {
						t.Fatalf("NewReader panicked: %v", p)
					}
				}()
				_, err = NewReader(bytes.NewReader(data), int64(len(data)), nil)
			}()
			var ae *AuthenticationError
			if !errors.As(err, &ae) {
				t.Errorf("expected AuthenticationError (parse reached auth), got %v", err)
			}
		})
	}
}

// aesCBCEncryptZeroIV encrypts whole 16-byte blocks with AES-CBC, a zero IV and
// no padding — the transform Algorithm 2.A uses to wrap the file encryption key
// into UE/OE.
func aesCBCEncryptZeroIV(t *testing.T, key, plaintext []byte) []byte {
	t.Helper()
	c, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(plaintext))
	cipher.NewCBCEncrypter(c, make([]byte, 16)).CryptBlocks(out, plaintext)
	return out
}

// TestAuthRevision5 exercises the best-effort read support for the deprecated
// Adobe AES-256 extension (revision 5).  R=5 shares revision 6's structure but
// hashes passwords with a plain SHA-256 instead of Algorithm 2.B.  This builds
// an R=5 handler with our own encoder and checks the read path recovers the file
// key and permissions; validation against real Adobe files is a TODO.
func TestAuthRevision5(t *testing.T) {
	id := []byte("0123456789ABCDEF")
	fileKey := bytes.Repeat([]byte{0x42}, 32)
	userPwd, ownerPwd := "user-secret", "owner-secret"

	uPrep, err := utf8Passwd(userPwd)
	if err != nil {
		t.Fatal(err)
	}
	oPrep, err := utf8Passwd(ownerPwd)
	if err != nil {
		t.Fatal(err)
	}

	// fixed salts (random in real files; arbitrary but valid here)
	uValSalt, uKeySalt := []byte("UVALSALT"), []byte("UKEYSALT")
	oValSalt, oKeySalt := []byte("OVALSALT"), []byte("OKEYSALT")

	// Algorithm 8 (revision-5 hash): U and UE
	U := make([]byte, 0, 48)
	U = append(U, hashRev(5, uPrep, uValSalt, nil)...)
	U = append(U, uValSalt...)
	U = append(U, uKeySalt...)
	UE := aesCBCEncryptZeroIV(t, hashRev(5, uPrep, uKeySalt, nil), fileKey)

	// Algorithm 9 (revision-5 hash): O and OE (depend on U)
	O := make([]byte, 0, 48)
	O = append(O, hashRev(5, oPrep, oValSalt, U)...)
	O = append(O, oValSalt...)
	O = append(O, oKeySalt...)
	OE := aesCBCEncryptZeroIV(t, hashRev(5, oPrep, oKeySalt, U), fileKey)

	sec := &stdSecHandler{
		ID: id, keyBytes: 32, R: 5,
		O: O, U: U, OE: OE, UE: UE,
		P: stdSecPermToP(PermPrint),
	}
	sec.Perms, err = sec.computePerms(fileKey)
	if err != nil {
		t.Fatal(err)
	}

	// user password: recovers the file key with restricted permissions
	sec.key = nil
	perm, err := sec.authenticate(userPwd)
	if err != nil {
		t.Fatalf("user authentication failed: %v", err)
	}
	if !bytes.Equal(sec.key, fileKey) {
		t.Error("user authentication recovered the wrong file key")
	}
	if perm == PermAll {
		t.Error("user authentication should not grant full permissions")
	}

	// owner password: recovers the file key with full permissions
	sec.key = nil
	perm, err = sec.authenticate(ownerPwd)
	if err != nil {
		t.Fatalf("owner authentication failed: %v", err)
	}
	if !bytes.Equal(sec.key, fileKey) {
		t.Error("owner authentication recovered the wrong file key")
	}
	if perm != PermAll {
		t.Errorf("owner perm = %v, want PermAll", perm)
	}

	// wrong password: authentication error
	sec.key = nil
	if _, err := sec.authenticate("nope"); err == nil {
		t.Error("expected authentication failure for the wrong password")
	}
}
