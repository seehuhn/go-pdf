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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
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
	msg := Text("super secret")
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

		// a wrong password falls back to the empty string, which only
		// succeeds if the user password is empty
		sec.key = nil
		perm, err := sec.unlock("wrong")
		switch {
		case test.user != "":
			if _, authErr := err.(*AuthenticationError); !authErr {
				t.Errorf("%d: expected AuthenticationError, got %v", i, err)
			}
		case err != nil:
			t.Errorf("%d: empty user password failed: %v", i, err)
		case test.owner == "" && perm != PermAll:
			t.Errorf("%d: expected PermAll for empty owner password", i)
		case test.owner != "" && perm == PermAll:
			t.Errorf("%d: wrong password granted PermAll", i)
		}

		// user password should work
		sec.key = nil
		perm, err = sec.unlock(test.user)
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
		perm, err = sec.unlock(test.owner)
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

	// empty user password should authenticate via unlock("")
	sec.key = nil
	_, err = sec.unlock("")
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
				w, err := enc.encryptStream(enc.stmF, ref, withDummyClose{buf})
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

				restored, err := enc.decryptStream(enc.stmF, ref, buf)
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
	err = w.Put(ref, Text("Hello, World!"))
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
// trailer that references the encryption dictionary as /Encrypt <encRef> 0 R,
// or has no /Encrypt if encRef is 0.
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
	encrypt := ""
	if encRef != 0 {
		encrypt = fmt.Sprintf("/Encrypt %d 0 R ", encRef)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R %s"+
		"/ID [%s %s] >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, encrypt, id, id, x)
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
			"<< /Filter /Standard /V 4.0 /R 4 /CF << /StdCF << /CFM /AESV2 /Length 16 >> >> /StmF /StdCF /StrF /StdCF /O " + h32 + " /U " + h32 + " /P -4 >>"}},
		{"indirect V", []string{cat, pages,
			"<< /Filter /Standard /V 4 0 R /R 4 /CF << /StdCF << /CFM /AESV2 /Length 16 >> >> /StmF /StdCF /StrF /StdCF /O " + h32 + " /U " + h32 + " /P -4 >>", "4"}},
		{"indirect R", []string{cat, pages,
			"<< /Filter /Standard /V 4 /R 4 0 R /CF << /StdCF << /CFM /AESV2 /Length 16 >> >> /StmF /StdCF /StrF /StdCF /O " + h32 + " /U " + h32 + " /P -4 >>", "4"}},
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
			if _, ok := errors.AsType[*AuthenticationError](err); !ok {
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
	perm, err := sec.unlock(userPwd)
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
	perm, err = sec.unlock(ownerPwd)
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
	if _, err := sec.unlock("nope"); err == nil {
		t.Error("expected authentication failure for the wrong password")
	}
}

// TestReaderPassword checks that the reader opens a file with the most
// permissive access granted by either the empty string or the given password.
func TestReaderPassword(t *testing.T) {
	type check struct {
		password string
		perm     Perm // 0 means authentication fails
	}
	cases := []struct {
		user   string
		checks []check
	}{
		{"", []check{
			{"", PermCopy},
			{"wrong", PermCopy},
			{"owner", PermAll},
		}},
		{"user", []check{
			{"", 0},
			{"wrong", 0},
			{"user", PermCopy},
			{"owner", PermAll},
		}},
	}
	for _, v := range []Version{V1_7, V2_0} {
		for _, c := range cases {
			opt := &WriterOptions{
				UserPassword:    c.user,
				OwnerPassword:   "owner",
				UserPermissions: PermCopy,
			}
			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, v, opt)
			if err != nil {
				t.Fatal(err)
			}
			w.GetMeta().Catalog.Pages = w.Alloc() // pretend we have pages
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			for _, ch := range c.checks {
				rOpt := &ReaderOptions{Password: ch.password}
				r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), rOpt)
				if ch.perm == 0 {
					if _, ok := errors.AsType[*AuthenticationError](err); !ok {
						t.Errorf("%s: user %q, password %q: got %v, want AuthenticationError",
							v, c.user, ch.password, err)
					}
					continue
				}
				if err != nil {
					t.Fatalf("%s: user %q, password %q: %v", v, c.user, ch.password, err)
				}
				if got := r.GetMeta().Permissions; got != ch.perm {
					t.Errorf("%s: user %q, password %q: got permissions %v, want %v",
						v, c.user, ch.password, got, ch.perm)
				}
			}
		}
	}
}

// TestCheckPerms checks that only the fields the specification requires are
// validated in the decrypted Perms block.
func TestCheckPerms(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	const P uint32 = 0xFFFFF0C0

	mkPerms := func(modify func(buf []byte)) []byte {
		buf := make([]byte, 16)
		binary.LittleEndian.PutUint32(buf, P)
		copy(buf[4:8], []byte{0xFF, 0xFF, 0xFF, 0xFF})
		buf[8] = 'T'
		copy(buf[9:12], "adb")
		modify(buf)
		c, _ := aes.NewCipher(key)
		c.Encrypt(buf, buf)
		return buf
	}

	cases := []struct {
		name   string
		modify func(buf []byte)
		ok     bool
	}{
		{"valid", func([]byte) {}, true},
		{"reserved bytes not 0xFF", func(buf []byte) { buf[5] = 0 }, true},
		{"EncryptMetadata mismatch", func(buf []byte) { buf[8] = 'F' }, true},
		{"random fill", func(buf []byte) { buf[13] = 7 }, true},
		{"P mismatch", func(buf []byte) { buf[1] ^= 0x04 }, false},
		{"missing marker", func(buf []byte) { buf[10] = 'x' }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sec := &stdSecHandler{P: P, Perms: mkPerms(c.modify)}
			err := sec.checkPerms(key)
			if (err == nil) != c.ok {
				t.Errorf("got err=%v, want ok=%v", err, c.ok)
			}
		})
	}
}

// buildDanglingEncryptFile returns a file with the given objects whose
// trailer has /Encrypt set to enc.
func buildDanglingEncryptFile(tb testing.TB, objs []string, enc string) []byte {
	tb.Helper()
	data := buildRawPDF(objs, 0)
	data = bytes.Replace(data, []byte("/Root 1 0 R "), []byte("/Root 1 0 R /Encrypt "+enc+" "), 1)
	if !bytes.Contains(data, []byte("/Encrypt "+enc)) {
		tb.Fatal("fixture lacks /Encrypt")
	}
	return data
}

// TestEncryptDanglingRef checks that an /Encrypt entry which does not
// resolve to a dictionary is treated like an absent entry.
func TestEncryptDanglingRef(t *testing.T) {
	cat := "<< /Type /Catalog /Pages 2 0 R >>"
	pages := "<< /Type /Pages /Kids [] /Count 0 >>"
	for _, tc := range []struct {
		name string
		objs []string
		enc  string
	}{
		{"missing object", []string{cat, pages}, "9 0 R"},
		{"wrong generation", []string{cat, pages, "<< >>"}, "3 5 R"},
		{"null", []string{cat, pages}, "null"},
		{"not a dictionary", []string{cat, pages, "42"}, "3 0 R"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := buildDanglingEncryptFile(t, tc.objs, tc.enc)
			r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
			if err != nil {
				t.Fatal(err)
			}
			if r.GetMeta().Permissions != PermAll {
				t.Errorf("permissions %v, want all", r.GetMeta().Permissions)
			}
			if r.GetMeta().Encryption != nil {
				t.Error("unexpected encryption info")
			}
			if _, ok := r.GetMeta().Trailer["Encrypt"]; ok {
				t.Error("trailer keeps the /Encrypt entry")
			}
		})
	}
}

func TestCryptFilterDictLength(t *testing.T) {
	id := []byte("0123456789ABCDEF")
	cases := []struct {
		version Version
		V       int
		length  int
	}{
		{V1_7, 4, 128},
		{V2_0, 5, 256},
	}
	for _, c := range cases {
		sec, err := createStdSecHandler(id, "secret", "supersecret", PermPrint, c.length, c.V, false)
		if err != nil {
			t.Fatal(err)
		}
		cf := &cryptFilter{Name: "StdCF", Cipher: cipherAES, Length: c.length}
		enc := &encryptInfo{sec: sec, strF: cf, stmF: cf, efF: cf,
			cf: map[Name]*cryptFilter{"StdCF": cf}}
		dict, err := enc.AsDict(c.version)
		if err != nil {
			t.Fatal(err)
		}
		if dict["Length"] != Integer(c.length) {
			t.Errorf("V %d: wrong Length: %v", c.V, dict["Length"])
		}
		cfDict, _ := dict["CF"].(Dict)
		stdCF, _ := cfDict["StdCF"].(Dict)
		if stdCF["Length"] != Integer(c.length) {
			t.Errorf("V %d: wrong CF/StdCF/Length: %v", c.V, stdCF["Length"])
		}
	}
}

func TestDecryptBytesKeepsInputOnBadPadding(t *testing.T) {
	id := []byte("0123456789ABCDEF")
	ref := NewReference(1, 0)
	sec, err := createStdSecHandler(id, "secret", "supersecret", PermPrint, 128, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	enc := encryptInfo{
		strF: &cryptFilter{Cipher: cipherAES, Length: 128},
		sec:  sec,
	}
	cipherText, err := enc.EncryptBytes(ref, []byte("0123456789ABCDEF"))
	if err != nil {
		t.Fatal(err)
	}
	cipherText[len(cipherText)-18] ^= 1
	orig := bytes.Clone(cipherText)
	if _, err := enc.DecryptBytes(ref, cipherText); err == nil {
		t.Fatal("expected a padding error")
	}
	if !bytes.Equal(cipherText, orig) {
		t.Error("input was modified")
	}
}

func TestEncryptBytesKeepsInput(t *testing.T) {
	id := []byte("0123456789ABCDEF")
	ref := NewReference(1, 0)
	sec, err := createStdSecHandler(id, "secret", "supersecret", PermPrint, 128, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, cipher := range []cipherType{cipherRC4, cipherAES} {
		enc := encryptInfo{
			strF: &cryptFilter{Cipher: cipher, Length: 128},
			sec:  sec,
		}
		plain := []byte("some plain text")
		orig := bytes.Clone(plain)
		if _, err := enc.EncryptBytes(ref, plain); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(plain, orig) {
			t.Errorf("cipher %d: EncryptBytes modified its input", cipher)
		}
	}
}

// object numbers of the streams in the file built by buildNamedCFFile
const (
	namedCFStdRef = 4 // /Crypt /StdCF
	namedCFMyRef  = 5 // /Crypt /MyCF
	namedCFDefRef = 6 // no /Crypt
)

// buildNamedCFFile returns a V 4 file whose /CF dictionary has the entries
// StdCF (AESV2) and MyCF (RC4), together with the payloads of its streams.
func buildNamedCFFile(tb testing.TB) ([]byte, map[uint32]string) {
	tb.Helper()
	id := make([]byte, 16) // matches the /ID written by buildRawPDF
	sec, err := createStdSecHandler(id, "", "owner", PermAll, 128, 4, false)
	if err != nil {
		tb.Fatal(err)
	}
	stdCF := &cryptFilter{Name: "StdCF", Cipher: cipherAES, Length: 128}
	myCF := &cryptFilter{Name: "MyCF", Cipher: cipherRC4, Length: 128}
	enc := &encryptInfo{
		sec:  sec,
		stmF: stdCF,
		strF: stdCF,
		efF:  stdCF,
		cf:   map[Name]*cryptFilter{"StdCF": stdCF, "MyCF": myCF},
	}

	const (
		stdRef = namedCFStdRef
		myRef  = namedCFMyRef
		defRef = namedCFDefRef
	)
	payload := map[uint32]string{
		stdRef: "stream encrypted with StdCF",
		myRef:  "stream encrypted with MyCF",
		defRef: "stream encrypted with the default filter",
	}
	streamObj := func(num uint32, cf *cryptFilter, filter string) string {
		buf := &bytes.Buffer{}
		w, err := enc.encryptStream(cf, NewReference(num, 0), withDummyClose{buf})
		if err != nil {
			tb.Fatal(err)
		}
		if _, err := w.Write([]byte(payload[num])); err != nil {
			tb.Fatal(err)
		}
		if err := w.Close(); err != nil {
			tb.Fatal(err)
		}
		return fmt.Sprintf("<< /Length %d %s >>\nstream\n%s\nendstream",
			buf.Len(), filter, buf.String())
	}
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
		fmt.Sprintf("<< /Filter /Standard /V 4 /R 4 /Length 128"+
			" /CF << /StdCF << /CFM /AESV2 /Length 16 >> /MyCF << /CFM /V2 /Length 16 >> >>"+
			" /StmF /StdCF /StrF /StdCF /O <%x> /U <%x> /P %d >>",
			sec.O, sec.U, int32(sec.P)),
		streamObj(stdRef, stdCF, "/Filter /Crypt /DecodeParms << /Name /StdCF >>"),
		streamObj(myRef, myCF, "/Filter [/Crypt] /DecodeParms [<< /Name /MyCF >>]"),
		streamObj(defRef, stdCF, ""),
	}
	return buildRawPDF(objs, 3), payload
}

// TestNamedCryptFilter checks streams which select a crypt filter of the
// /CF dictionary through a leading /Crypt entry.
func TestNamedCryptFilter(t *testing.T) {
	const (
		stdRef = namedCFStdRef
		myRef  = namedCFMyRef
		defRef = namedCFDefRef
	)
	data, payload := buildNamedCFFile(t)

	// checkStreams verifies that the streams in r decode to the payloads
	checkStreams := func(t *testing.T, r Getter, refs map[uint32]Reference) {
		t.Helper()
		for num, ref := range refs {
			stm, err := NewCursor(r).Stream(ref)
			if err != nil {
				t.Fatal(err)
			}
			rc, err := DecodeStream(r, nil, stm)
			if err != nil {
				t.Fatalf("object %d: %v", num, err)
			}
			got, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("object %d: %v", num, err)
			}
			if string(got) != payload[num] {
				t.Errorf("object %d: got %q, want %q", num, got, payload[num])
			}
		}
	}
	srcRefs := map[uint32]Reference{}
	for num := range payload {
		srcRefs[num] = NewReference(num, 0)
	}

	src, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("DecodeStream", func(t *testing.T) {
		checkStreams(t, src, srcRefs)
	})

	t.Run("RawStreamReader", func(t *testing.T) {
		for num, ref := range srcRefs {
			stm, err := NewCursor(src).Stream(ref)
			if err != nil {
				t.Fatal(err)
			}
			rc, err := RawStreamReader(src, stm)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != payload[num] {
				t.Errorf("object %d: got %q, want %q", num, got, payload[num])
			}
		}
	})

	for _, encrypted := range []bool{true, false} {
		t.Run(fmt.Sprintf("Copier/encrypted=%t", encrypted), func(t *testing.T) {
			opt := &WriterOptions{}
			if encrypted {
				opt.UserPassword = "dst"
			}
			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, V1_6, opt)
			if err != nil {
				t.Fatal(err)
			}
			copier := NewCopier(w, src)
			dstRefs := map[uint32]Reference{}
			for num, ref := range srcRefs {
				dstRefs[num], err = copier.CopyReference(ref)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := addPage(w); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			dst, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()),
				&ReaderOptions{Password: "dst"})
			if err != nil {
				t.Fatal(err)
			}
			checkStreams(t, dst, dstRefs)

			// the destination defines StdCF only if it is encrypted
			wantFilter := map[uint32]Object{stdRef: nil, myRef: nil, defRef: nil}
			if encrypted {
				wantFilter[stdRef] = Name("Crypt")
			}
			for num, ref := range dstRefs {
				stm, err := NewCursor(dst).Stream(ref)
				if err != nil {
					t.Fatal(err)
				}
				if got := stm.Dict["Filter"]; got != wantFilter[num] {
					t.Errorf("object %d: /Filter %v, want %v", num, got, wantFilter[num])
				}
				if num != stdRef || !encrypted {
					if _, ok := stm.Dict["DecodeParms"]; ok {
						t.Errorf("object %d: unexpected /DecodeParms", num)
					}
				}
			}
		})
	}

	t.Run("Update", func(t *testing.T) {
		dst := &bytes.Buffer{}
		w, err := NewUpdaterTo(bytes.NewReader(data), int64(len(data)), dst, nil)
		if err != nil {
			t.Fatal(err)
		}
		newRefs := map[uint32]Reference{}
		for num, f := range map[uint32]Filter{
			stdRef: FilterCryptStandard{},
			myRef:  FilterCryptNamed{Name: "MyCF"},
		} {
			ref := w.Alloc()
			s, err := w.OpenStream(ref, nil, f)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Write([]byte(payload[num])); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			newRefs[num] = ref
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}

		r, err := NewReader(bytes.NewReader(dst.Bytes()), int64(dst.Len()), nil)
		if err != nil {
			t.Fatal(err)
		}
		checkStreams(t, r, newRefs)
		checkStreams(t, r, srcRefs)
	})

	t.Run("Writer", func(t *testing.T) {
		buf := &bytes.Buffer{}
		w, err := NewWriter(buf, V1_6, &WriterOptions{UserPassword: "u"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.OpenStream(w.Alloc(), nil, FilterCryptNamed{Name: "MyCF"}); err == nil {
			t.Error("expected error for undefined crypt filter")
		}

		ref := w.Alloc()
		s, err := w.OpenStream(ref, nil, FilterCryptStandard{}, FilterFlate{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Write([]byte(payload[stdRef])); err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if err := addPage(w); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()),
			&ReaderOptions{Password: "u"})
		if err != nil {
			t.Fatal(err)
		}
		checkStreams(t, r, map[uint32]Reference{stdRef: ref})
	})

	t.Run("UnencryptedWriter", func(t *testing.T) {
		w, err := NewWriter(&bytes.Buffer{}, V1_6, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.OpenStream(w.Alloc(), nil, FilterCryptStandard{}); err == nil {
			t.Error("expected error for crypt filter in unencrypted file")
		}
	})
}

// TestCFLengthPermissive checks the interpretation of /Length in crypt
// filter dictionaries.
func TestCFLengthPermissive(t *testing.T) {
	c := NewCursor(mockGetter)
	for _, tc := range []struct {
		cfm    Name
		length Object
		want   int
	}{
		{"V2", nil, 128},
		{"V2", Integer(5), 40},
		{"V2", Integer(16), 128},
		{"V2", Integer(40), 40},
		{"V2", Integer(96), 96},
		{"V2", Integer(128), 128},
		{"V2", Integer(4), 128},
		{"V2", Integer(17), 128},
		{"V2", Integer(44), 128},
		{"V2", Integer(256), 128},
		{"V2", Name("x"), 128},
		{"AESV2", Integer(5), 128},
		{"AESV2", nil, 128},
		{"AESV3", Integer(16), 256},
		{"AESV3", nil, 256},
	} {
		cfDict := Dict{"CFM": tc.cfm, "Length": tc.length}
		cf, err := getCryptFilter(c, "MyCF", cfDict)
		if err != nil {
			t.Errorf("%s/%v: %v", tc.cfm, tc.length, err)
			continue
		}
		if cf.Length != tc.want {
			t.Errorf("%s/%v: length %d, want %d", tc.cfm, tc.length, cf.Length, tc.want)
		}
		if cf.AuthEvent != "DocOpen" {
			t.Errorf("%s/%v: AuthEvent %q", tc.cfm, tc.length, cf.AuthEvent)
		}
	}

	for _, cfm := range []Object{nil, Name("None"), Name("AESV4"), Integer(2)} {
		_, err := getCryptFilter(c, "MyCF", Dict{"CFM": cfm})
		if !IsMalformed(err) {
			t.Errorf("CFM %v: expected malformed-file error, got %v", cfm, err)
		}
	}
}

// effTestFile is a hand-built V 4 file whose /EFF differs from /StmF.
type effTestFile struct {
	data []byte
	enc  *encryptInfo

	payload map[Reference]string // stream contents, by object
	specRef Reference            // file specification with /EF and /RF
	efRef   Reference            // untyped stream referenced from /EF
	rfRef   Reference            // untyped stream referenced from /RF
	stmRef  Reference            // ordinary stream
	stdRef  Reference            // /EF stream with its own /Crypt /StdCF
	idRef   Reference            // /EF stream with its own /Crypt /Identity
}

// buildEFFTestFile returns a file with /StmF /StdCF (AESV2) and the given
// /EFF, which is either Identity or MyCF (RC4).  The embedded file streams
// lack /Type /EmbeddedFile, so that only their use in /EF and /RF identifies
// them.
func buildEFFTestFile(t *testing.T, eff Name) *effTestFile {
	t.Helper()
	id := make([]byte, 16) // matches the /ID written by buildRawPDF
	sec, err := createStdSecHandler(id, "", "owner", PermAll, 128, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	stdCF := &cryptFilter{Name: "StdCF", Cipher: cipherAES, Length: 128}
	myCF := &cryptFilter{Name: "MyCF", Cipher: cipherRC4, Length: 128}
	enc := &encryptInfo{
		sec:  sec,
		stmF: stdCF,
		strF: stdCF,
		cf:   map[Name]*cryptFilter{"StdCF": stdCF, "MyCF": myCF},
	}
	switch eff {
	case "Identity":
	case "MyCF":
		enc.efF = myCF
	default:
		t.Fatalf("unsupported EFF %s", eff)
	}

	f := &effTestFile{
		enc:     enc,
		specRef: NewReference(4, 0),
		efRef:   NewReference(5, 0),
		rfRef:   NewReference(6, 0),
		stmRef:  NewReference(7, 0),
		stdRef:  NewReference(8, 0),
		idRef:   NewReference(9, 0),
	}
	f.payload = map[Reference]string{
		f.stdRef: "own StdCF contents",
		f.idRef:  "own Identity contents",
		f.efRef:  "embedded file contents",
		f.rfRef:  "related file contents",
		f.stmRef: "ordinary stream contents",
	}
	streamObj := func(ref Reference, cf *cryptFilter, extra string) string {
		buf := &bytes.Buffer{}
		w, err := enc.encryptStream(cf, ref, withDummyClose{buf})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(f.payload[ref])); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("<< %s/Length %d >>\nstream\n%s\nendstream", extra, buf.Len(), buf.String())
	}
	fileName, err := enc.EncryptBytes(f.specRef, []byte("a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
		fmt.Sprintf("<< /Filter /Standard /V 4 /R 4 /Length 128"+
			" /CF << /StdCF << /CFM /AESV2 /Length 16 >> /MyCF << /CFM /V2 /Length 16 >> >>"+
			" /StmF /StdCF /StrF /StdCF /EFF /%s /O <%x> /U <%x> /P %d >>",
			eff, sec.O, sec.U, int32(sec.P)),
		fmt.Sprintf("<< /Type /Filespec /EF << /F 5 0 R /UF 8 0 R /DOS 9 0 R >> /RF << /F [<%x> 6 0 R] >> >>",
			fileName),
		streamObj(f.efRef, enc.efF, ""),
		streamObj(f.rfRef, enc.efF, ""),
		streamObj(f.stmRef, enc.stmF, ""),
		streamObj(f.stdRef, stdCF, "/Filter /Crypt /DecodeParms << /Name /StdCF >> "),
		streamObj(f.idRef, nil, "/Filter /Crypt "),
	}
	f.data = buildRawPDF(objs, 3)
	return f
}

// decodeAll returns the decoded contents of stm.
func decodeAll(t *testing.T, r Getter, stm *Stream) string {
	t.Helper()
	rc, err := DecodeStream(r, nil, stm)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

// TestEmbeddedFileEFF checks that embedded file streams read from a file
// whose /EFF differs from /StmF are decrypted with /EFF.
func TestEmbeddedFileEFF(t *testing.T) {
	for _, eff := range []Name{"Identity", "MyCF"} {
		t.Run(string(eff), func(t *testing.T) {
			f := buildEFFTestFile(t, eff)
			r, err := NewReader(bytes.NewReader(f.data), int64(len(f.data)), nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, ref := range []Reference{f.efRef, f.rfRef} {
				stm, err := NewCursor(r).Stream(ref)
				if err != nil {
					t.Fatal(err)
				}
				if got := decodeAll(t, r, stm.AsEmbeddedFile()); got != f.payload[ref] {
					t.Errorf("DecodeStream: got %q, want %q", got, f.payload[ref])
				}

				rc, err := RawStreamReader(r, stm.AsEmbeddedFile())
				if err != nil {
					t.Fatal(err)
				}
				got, err := io.ReadAll(rc)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != f.payload[ref] {
					t.Errorf("RawStreamReader: got %q, want %q", got, f.payload[ref])
				}
			}
			stm, err := NewCursor(r).Stream(f.stmRef)
			if err != nil {
				t.Fatal(err)
			}
			if got := decodeAll(t, r, stm); got != f.payload[f.stmRef] {
				t.Errorf("got %q, want %q", got, f.payload[f.stmRef])
			}
		})
	}
}

// TestEmbeddedFileOwnCrypt checks that an embedded file stream with its own
// leading /Crypt filter is decrypted with that filter rather than /EFF.
func TestEmbeddedFileOwnCrypt(t *testing.T) {
	for _, eff := range []Name{"Identity", "MyCF"} {
		t.Run(string(eff), func(t *testing.T) {
			f := buildEFFTestFile(t, eff)
			r, err := NewReader(bytes.NewReader(f.data), int64(len(f.data)), nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, ref := range []Reference{f.stdRef, f.idRef} {
				stm, err := NewCursor(r).Stream(ref)
				if err != nil {
					t.Fatal(err)
				}
				if got := decodeAll(t, r, stm.AsEmbeddedFile()); got != f.payload[ref] {
					t.Errorf("object %d: got %q, want %q", ref.Number(), got, f.payload[ref])
				}
			}
		})
	}
}

// attachmentsOnlyFile is a hand-built V 4 file in which only embedded files
// are encrypted: /StmF and /StrF are absent and /EFF /StdCF has
// /AuthEvent /EFOpen.  The user password is "secret".
type attachmentsOnlyFile struct {
	data    []byte
	payload string
	efRef   Reference
	pageRef Reference
	perm    Perm // user permissions given by /R and /P
}

func buildAttachmentsOnlyFile(tb testing.TB) *attachmentsOnlyFile {
	tb.Helper()
	id := make([]byte, 16) // matches the /ID written by buildRawPDF
	sec, err := createStdSecHandler(id, "secret", "owner", PermAll&^PermPrint, 128, 4, false)
	if err != nil {
		tb.Fatal(err)
	}
	stdCF := &cryptFilter{Name: "StdCF", Cipher: cipherAES, Length: 128}
	enc := &encryptInfo{sec: sec, efF: stdCF, cf: map[Name]*cryptFilter{"StdCF": stdCF}}
	f := &attachmentsOnlyFile{
		payload: "attachment contents",
		efRef:   NewReference(5, 0),
		pageRef: NewReference(6, 0),
		perm:    stdSecPToPerm(sec.R, sec.P),
	}
	buf := &bytes.Buffer{}
	w, err := enc.encryptStream(stdCF, f.efRef, withDummyClose{buf})
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := w.Write([]byte(f.payload)); err != nil {
		tb.Fatal(err)
	}
	if err := w.Close(); err != nil {
		tb.Fatal(err)
	}
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
		fmt.Sprintf("<< /Filter /Standard /V 4 /R 4 /Length 128"+
			" /CF << /StdCF << /CFM /AESV2 /Length 16 /AuthEvent /EFOpen >> >>"+
			" /EFF /StdCF /O <%x> /U <%x> /P %d >>",
			sec.O, sec.U, int32(sec.P)),
		"<< /Type /Filespec /EF << /F 5 0 R >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", buf.Len(), buf.String()),
		"<< /Length 12 >>\nstream\nplain stream\nendstream",
	}
	f.data = buildRawPDF(objs, 3)
	return f
}

// TestAttachmentsOnlyDeferredAuth checks that a file with only its embedded
// files encrypted opens without a password, and that the password is needed
// only to access the embedded files.
func TestAttachmentsOnlyDeferredAuth(t *testing.T) {
	f := buildAttachmentsOnlyFile(t)

	r, err := NewReader(bytes.NewReader(f.data), int64(len(f.data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.GetMeta().Permissions; got != f.perm {
		t.Errorf("permissions: got %v, want %v", got, f.perm)
	}
	stm, err := NewCursor(r).Stream(f.pageRef)
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeAll(t, r, stm); got != "plain stream" {
		t.Errorf("plain stream: got %q", got)
	}

	stm, err = NewCursor(r).Stream(f.efRef)
	if err != nil {
		t.Fatal(err)
	}
	rc, err := DecodeStream(r, nil, stm.AsEmbeddedFile())
	if err == nil {
		_, err = io.ReadAll(rc)
	}
	if _, ok := errors.AsType[*AuthenticationError](err); !ok {
		t.Errorf("embedded file without password: got error %v", err)
	}

	// with the password, the embedded file can be read
	r, err = NewReader(bytes.NewReader(f.data), int64(len(f.data)),
		&ReaderOptions{Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	stm, err = NewCursor(r).Stream(f.efRef)
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeAll(t, r, stm.AsEmbeddedFile()); got != f.payload {
		t.Errorf("embedded file: got %q, want %q", got, f.payload)
	}
}

// TestAttachmentsOnlyDeferredAuthUpdate checks that in update mode, adding
// an embedded file to an attachments-only file needs the password, while
// other changes do not.
func TestAttachmentsOnlyDeferredAuthUpdate(t *testing.T) {
	f := buildAttachmentsOnlyFile(t)

	w, err := NewUpdaterTo(bytes.NewReader(f.data), int64(len(f.data)), io.Discard, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.OpenEmbeddedFileStream(w.Alloc(), nil)
	if _, ok := errors.AsType[*AuthenticationError](err); !ok {
		t.Errorf("embedded file without password: got error %v", err)
	}
	s, err := w.OpenStream(w.Alloc(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	w, err = NewUpdaterTo(bytes.NewReader(f.data), int64(len(f.data)), io.Discard,
		&UpdateOptions{Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	s, err = w.OpenEmbeddedFileStream(w.Alloc(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestAllIdentityMalformedAuth checks that a file without /StmF, /StrF and
// /EFF, in which all data is stored as plaintext, opens despite a wrong
// password and garbage /O and /U values.
func TestAllIdentityMalformedAuth(t *testing.T) {
	garbage := func(n int) string {
		return fmt.Sprintf("<%x>", bytes.Repeat([]byte{0xA5}, n))
	}
	cases := []struct {
		name string
		enc  string
	}{
		{"V4", "<< /Filter /Standard /V 4 /R 4 /Length 128" +
			" /CF << /StdCF << /CFM /AESV2 >> >>" +
			" /O " + garbage(32) + " /U " + garbage(32) + " /P -4 >>"},
		{"V5", "<< /Filter /Standard /V 5 /R 6" +
			" /CF << /StdCF << /CFM /AESV3 >> >>" +
			" /O " + garbage(48) + " /U " + garbage(48) +
			" /OE " + garbage(32) + " /UE " + garbage(32) +
			" /Perms " + garbage(16) + " /P -4 >>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			objs := []string{
				"<< /Type /Catalog /Pages 2 0 R >>",
				"<< /Type /Pages /Kids [] /Count 0 >>",
				c.enc,
				"<< /Length 12 >>\nstream\nplain stream\nendstream",
			}
			data := buildRawPDF(objs, 3)
			r, err := NewReader(bytes.NewReader(data), int64(len(data)),
				&ReaderOptions{Password: "wrong"})
			if err != nil {
				t.Fatal(err)
			}
			want := &Encryption{
				Streams:       CryptIdentity,
				Strings:       CryptIdentity,
				EmbeddedFiles: CryptIdentity,
			}
			if diff := cmp.Diff(want, r.GetMeta().Encryption); diff != "" {
				t.Errorf("wrong Encryption (-want +got):\n%s", diff)
			}
			stm, err := NewCursor(r).Stream(NewReference(4, 0))
			if err != nil {
				t.Fatal(err)
			}
			if got := decodeAll(t, r, stm); got != "plain stream" {
				t.Errorf("got %q", got)
			}
		})
	}
}

// TestAuthEventRoundTrip checks that a crypt filter used only for embedded
// files is written with /AuthEvent /EFOpen and reads back unchanged.
func TestAuthEventRoundTrip(t *testing.T) {
	for _, v := range []Version{V1_7, V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			opt := &WriterOptions{
				UserPassword:   "secret",
				EncryptStreams: CryptIdentity,
				EncryptStrings: CryptIdentity,
			}
			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, v, opt)
			if err != nil {
				t.Fatal(err)
			}
			want, err := w.w.enc.AsDict(v)
			if err != nil {
				t.Fatal(err)
			}
			if err := addPage(w); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil)
			if err != nil {
				t.Fatal(err)
			}
			if r.enc.efF == nil || r.enc.efF.AuthEvent != "EFOpen" {
				t.Errorf("embedded file filter: got %v", r.enc.efF)
			}
			got, err := r.enc.AsDict(v)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("encryption dictionary round trip (-want +got):\n%s", diff)
			}
		})
	}
}

// TestV4RC4KeyLength checks that a V 4 file whose stream filter is RC4 with a
// key shorter than 128 bits derives the file key length from /CF.
func TestV4RC4KeyLength(t *testing.T) {
	const (
		stmPayload = "short-key stream"
		strPayload = "short-key string"
	)
	for _, cfLength := range []int{5, 40} {
		t.Run(fmt.Sprintf("Length=%d", cfLength), func(t *testing.T) {
			id := make([]byte, 16) // matches the /ID written by buildRawPDF
			sec, err := createStdSecHandler(id, "", "owner", PermAll, 40, 4, false)
			if err != nil {
				t.Fatal(err)
			}
			cf := &cryptFilter{Name: "StdCF", Cipher: cipherRC4, Length: 40}
			enc := &encryptInfo{sec: sec, stmF: cf, strF: cf, efF: cf,
				cf: map[Name]*cryptFilter{"StdCF": cf}}

			stmRef := NewReference(4, 0)
			buf := &bytes.Buffer{}
			sw, err := enc.encryptStream(cf, stmRef, withDummyClose{buf})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := sw.Write([]byte(stmPayload)); err != nil {
				t.Fatal(err)
			}
			if err := sw.Close(); err != nil {
				t.Fatal(err)
			}
			strRef := NewReference(5, 0)
			encStr, err := enc.EncryptBytes(strRef, []byte(strPayload))
			if err != nil {
				t.Fatal(err)
			}

			objs := []string{
				"<< /Type /Catalog /Pages 2 0 R >>",
				"<< /Type /Pages /Kids [] /Count 0 >>",
				fmt.Sprintf("<< /Filter /Standard /V 4 /R 4"+
					" /CF << /StdCF << /CFM /V2 /Length %d >> >>"+
					" /StmF /StdCF /StrF /StdCF /O <%x> /U <%x> /P %d >>",
					cfLength, sec.O, sec.U, int32(sec.P)),
				fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", buf.Len(), buf.String()),
				fmt.Sprintf("[<%x>]", encStr),
			}
			data := buildRawPDF(objs, 3)

			r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
			if err != nil {
				t.Fatal(err)
			}
			want := &Encryption{Streams: CryptRC4, Strings: CryptRC4,
				EmbeddedFiles: CryptRC4, KeyLength: 40}
			if diff := cmp.Diff(want, r.GetMeta().Encryption); diff != "" {
				t.Errorf("wrong encryption (-want +got):\n%s", diff)
			}
			stm, err := NewCursor(r).Stream(stmRef)
			if err != nil {
				t.Fatal(err)
			}
			if got := decodeAll(t, r, stm); got != stmPayload {
				t.Errorf("stream: got %q, want %q", got, stmPayload)
			}
			arr, err := NewCursor(r).Array(strRef)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(Array{String(strPayload)}, arr); diff != "" {
				t.Errorf("string (-want +got):\n%s", diff)
			}
		})
	}
}

// TestDecodeStreamMalformedAES checks AES stream bodies which are empty or
// not valid ciphertext.
func TestDecodeStreamMalformedAES(t *testing.T) {
	id := make([]byte, 16) // matches the /ID written by buildRawPDF
	sec, err := createStdSecHandler(id, "", "owner", PermAll, 128, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		body      []byte
		malformed bool
	}{
		{"empty", nil, false},
		{"short IV", bytes.Repeat([]byte{1}, 8), true},
		{"partial block", bytes.Repeat([]byte{1}, 20), true},
		{"bad padding", bytes.Repeat([]byte{1}, 32), true},
	}
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
		fmt.Sprintf("<< /Filter /Standard /V 4 /R 4"+
			" /CF << /StdCF << /CFM /AESV2 /Length 16 >> >>"+
			" /StmF /StdCF /StrF /StdCF /O <%x> /U <%x> /P %d >>",
			sec.O, sec.U, int32(sec.P)),
	}
	for _, c := range cases {
		objs = append(objs, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream",
			len(c.body), c.body))
	}
	data := buildRawPDF(objs, 3)
	r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stm, err := NewCursor(r).Stream(NewReference(uint32(4+i), 0))
			if err != nil {
				t.Fatal(err)
			}
			var got []byte
			rc, err := DecodeStream(r, nil, stm)
			if err == nil {
				got, err = io.ReadAll(rc)
			}
			switch {
			case c.malformed && !IsMalformed(err):
				t.Errorf("expected malformed-file error, got %v", err)
			case !c.malformed && err != nil:
				t.Error(err)
			case !c.malformed && len(got) != 0:
				t.Errorf("got %q, want empty data", got)
			}
		})
	}
}

// TestV4ShortKeyAES checks a V 4 file with a 40-bit RC4 stream filter and an
// AES filter: RC4 streams decrypt, while AES data, which needs a longer file
// key, is reported as malformed.
func TestV4ShortKeyAES(t *testing.T) {
	const payload = "RC4-40 stream"
	id := make([]byte, 16) // matches the /ID written by buildRawPDF
	sec, err := createStdSecHandler(id, "", "owner", PermAll, 40, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	cf := &cryptFilter{Name: "StdCF", Cipher: cipherRC4, Length: 40}
	enc := &encryptInfo{sec: sec, stmF: cf, strF: cf, efF: cf,
		cf: map[Name]*cryptFilter{"StdCF": cf}}
	rc4Ref := NewReference(4, 0)
	buf := &bytes.Buffer{}
	sw, err := enc.encryptStream(cf, rc4Ref, withDummyClose{buf})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sw.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	if err := sw.Close(); err != nil {
		t.Fatal(err)
	}
	aesBody := bytes.Repeat([]byte{1}, 32)

	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
		fmt.Sprintf("<< /Filter /Standard /V 4 /R 4"+
			" /CF << /StdCF << /CFM /V2 /Length 5 >> /MyCF << /CFM /AESV2 >> >>"+
			" /StmF /StdCF /StrF /StdCF /O <%x> /U <%x> /P %d >>",
			sec.O, sec.U, int32(sec.P)),
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", buf.Len(), buf.String()),
		fmt.Sprintf("<< /Length %d /Filter /Crypt /DecodeParms << /Name /MyCF >> >>\nstream\n%s\nendstream",
			len(aesBody), aesBody),
	}
	data := buildRawPDF(objs, 3)
	r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}

	stm, err := NewCursor(r).Stream(rc4Ref)
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeAll(t, r, stm); got != payload {
		t.Errorf("RC4 stream: got %q, want %q", got, payload)
	}

	stm, err = NewCursor(r).Stream(NewReference(5, 0))
	if err != nil {
		t.Fatal(err)
	}
	rc, err := DecodeStream(r, nil, stm)
	if err == nil {
		_, err = io.ReadAll(rc)
	}
	if !IsMalformed(err) {
		t.Errorf("AES stream: expected malformed-file error, got %v", err)
	}
}

// TestV4RC4KeyLengthEFF checks that the file key length follows /EFF when
// that is the only RC4 filter.
func TestV4RC4KeyLengthEFF(t *testing.T) {
	id := make([]byte, 16) // matches the /ID written by buildRawPDF
	sec, err := createStdSecHandler(id, "", "owner", PermAll, 40, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
		fmt.Sprintf("<< /Filter /Standard /V 4 /R 4"+
			" /CF << /StdCF << /CFM /V2 /Length 40 >> >>"+
			" /StmF /Identity /StrF /Identity /EFF /StdCF /O <%x> /U <%x> /P %d >>",
			sec.O, sec.U, int32(sec.P)),
	}
	data := buildRawPDF(objs, 3)
	r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := &Encryption{Streams: CryptIdentity, Strings: CryptIdentity,
		EmbeddedFiles: CryptRC4, KeyLength: 40}
	if diff := cmp.Diff(want, r.GetMeta().Encryption); diff != "" {
		t.Errorf("wrong encryption (-want +got):\n%s", diff)
	}
	if r.enc.sec.key == nil {
		t.Error("authentication failed")
	}
}
