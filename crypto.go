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
	"crypto/md5"
	"crypto/rand"
	"crypto/rc4"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"

	"github.com/xdg-go/stringprep"
)

type encryptInfo struct {
	sec *stdSecHandler

	strF *cryptFilter // strings
	stmF *cryptFilter // streams
	efF  *cryptFilter // embedded files

	UserPermissions Perm
}

func (r *Reader) parseEncryptDict(encObj Object, readPwd func([]byte, int) string) (*encryptInfo, error) {
	enc, err := GetDict(r, encObj)
	if err != nil {
		return nil, err
	}
	if len(r.ID) != 2 {
		return nil, &MalformedFileError{Err: errors.New("found Encrypt but no ID")}
	}

	res := &encryptInfo{}

	filter, err := GetName(r, enc["Filter"])
	if err != nil {
		return nil, err
	}
	// subFilter, err := GetName(r, enc["SubFilter"])
	// if err != nil {
	// 	return nil, err
	// }

	// version of the encryption/decryption algorithm
	V, err := GetInt(r, enc["V"])
	if err != nil {
		return nil, err
	}

	var keyBytes int
	switch V {
	case 1:
		cf := &cryptFilter{
			Cipher: cipherRC4,
			Length: 40,
		}
		res.stmF = cf
		res.strF = cf
		res.efF = cf

		keyBytes = 5
	case 2:
		cf := &cryptFilter{
			Cipher: cipherRC4,
			Length: 40, // default
		}
		if obj, ok := enc["Length"].(Integer); ok && (V == 2 || V == 3) {
			cf.Length = int(obj)
			if cf.Length < 40 || cf.Length > 128 || cf.Length%8 != 0 {
				return nil, &MalformedFileError{
					Err: fmt.Errorf("invalid Length=%d", cf.Length),
				}
			}
		}
		res.stmF = cf
		res.strF = cf
		res.efF = cf
		keyBytes = cf.Length / 8
	case 4, 5:
		var CF Dict
		if obj, ok := enc["CF"].(Dict); ok {
			CF = obj
		}
		if obj, ok := enc["StmF"].(Name); ok {
			cf, err := getCryptFilter(obj, CF)
			if err != nil {
				return nil, wrap(err, "StmF")
			}
			res.stmF = cf
		}
		if obj, ok := enc["StrF"].(Name); ok {
			cf, err := getCryptFilter(obj, CF)
			if err != nil {
				return nil, wrap(err, "StrF")
			}
			res.strF = cf
		}
		res.efF = res.stmF // default
		if obj, ok := enc["EFF"].(Name); ok {
			cf, err := getCryptFilter(obj, CF)
			if err != nil {
				return nil, wrap(err, "EFF")
			}
			res.efF = cf
		}
		if V == 4 {
			keyBytes = 16
		} else {
			keyBytes = 32
		}

	default:
		return nil, &MalformedFileError{
			Err: fmt.Errorf("invalid V=%d", V),
		}
	}

	switch {
	case filter == "Standard":
		sec, err := openStdSecHandler(enc, keyBytes, r.ID[0], readPwd)
		if err != nil {
			return nil, wrap(err, "standard security handler")
		}
		res.sec = sec
		res.UserPermissions = stdSecPToPerm(sec.R, sec.P)
	default:
		return nil, &MalformedFileError{
			Err: fmt.Errorf("unsupported Filter=%s", filter),
		}
	}

	return res, nil
}

func (enc *encryptInfo) AsDict(version Version) (Dict, error) {
	dict := Dict{
		"Filter": Name("Standard"),
	}

	length := -1
	var cipher cipherType
	for _, cf := range []*cryptFilter{enc.stmF, enc.strF, enc.efF} {
		if length < 0 {
			length = cf.Length
			cipher = cf.Cipher
		} else {
			if length > 0 && length != cf.Length {
				length = 0
			}
			if cipher != cf.Cipher {
				cipher = cipherUnknown
			}
		}

		if cf.Length%8 != 0 {
			return nil, errors.New("invalid key length")
		}
	}
	if length == 0 {
		return nil, errors.New("not implemented: mixed key length")
	}
	if cipher == cipherUnknown {
		return nil, errors.New("not implemented: mixed key ciphers")
	}

	if cipher == cipherAES && length == 256 && version >= V2_0 {
		dict["V"] = Integer(5)
		dict["StmF"] = Name("StdCF")
		dict["StrF"] = Name("StdCF")
		dict["Length"] = Integer(256)
		dict["CF"] = Dict{
			"StdCF": Dict{"Length": Integer(256), "CFM": Name("AESV3")},
		}
	} else if cipher == cipherAES && length == 128 && version >= V1_6 {
		dict["V"] = Integer(4)
		dict["StmF"] = Name("StdCF")
		dict["StrF"] = Name("StdCF")
		dict["CF"] = Dict{
			"StdCF": Dict{"Length": Integer(128), "CFM": Name("AESV2")},
		}
	} else if cipher == cipherRC4 && length == 40 && version >= V1_1 {
		dict["V"] = Integer(1)
	} else if cipher == cipherRC4 && version >= V1_4 {
		dict["V"] = Integer(2)
		dict["Length"] = Integer(length)
	} else {
		return nil, errors.New("no supported encryption scheme found")
	}

	sec := enc.sec
	dict["R"] = Integer(sec.R)
	dict["O"] = String(sec.O)
	dict["U"] = String(sec.U)
	dict["P"] = Integer(int32(sec.P))
	if sec.unencryptedMetaData {
		dict["EncryptMetadata"] = Bool(false)
	}
	if sec.R == 6 {
		dict["OE"] = String(sec.OE)
		dict["UE"] = String(sec.UE)
		dict["Perms"] = String(sec.Perms)
	}

	return dict, nil
}

// EncryptBytes encrypts the bytes in buf using Algorithm 1 in the PDF spec.
// This function modfies the contents of buf and may return buf.
func (enc *encryptInfo) EncryptBytes(ref Reference, buf []byte) ([]byte, error) {
	cf := enc.strF
	if cf == nil {
		return buf, nil
	}

	key, err := enc.sec.KeyForRef(cf, ref)
	if err != nil {
		return nil, err
	}
	switch cf.Cipher {
	case cipherAES:
		n := len(buf)
		nPad := 16 - n%16
		out := make([]byte, 16+n+nPad) // iv | c(data|padding)

		iv := out[:16]
		_, err = io.ReadFull(rand.Reader, iv)
		if err != nil {
			return nil, err
		}

		c, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		cbc := cipher.NewCBCEncrypter(c, iv)
		cbc.CryptBlocks(out[16:], buf[:n+nPad-16])
		// encrypt the last block separately, after appending the padding
		copy(out[n+nPad:], buf[n+nPad-16:])
		for i := 16 + n; i < len(out); i++ {
			out[i] = byte(nPad)
		}
		cbc.CryptBlocks(out[n+nPad:], out[n+nPad:])
		return out, nil
	case cipherRC4:
		c, err := rc4.NewCipher(key)
		if err != nil {
			return nil, err
		}
		c.XORKeyStream(buf, buf)
		return buf, nil
	default:
		panic("unknown cipher")
	}
}

// DecryptBytes decrypts the bytes in buf using Algorithm 1 in the PDF spec.
// This function modfies the contents of buf and may return buf.
func (enc *encryptInfo) DecryptBytes(ref Reference, buf []byte) ([]byte, error) {
	cf := enc.strF
	if cf == nil {
		return buf, nil
	}

	key, err := enc.sec.KeyForRef(cf, ref)
	if err != nil {
		return nil, err
	}
	switch cf.Cipher {
	case cipherAES:
		if len(buf) < 32 {
			return nil, errCorrupted
		}
		iv := buf[:16]

		c, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}

		cbc := cipher.NewCBCDecrypter(c, iv)
		cbc.CryptBlocks(buf[16:], buf[16:])

		nPad := int(buf[len(buf)-1])
		if nPad < 1 || nPad > 16 {
			return nil, errCorrupted
		}
		return buf[16 : len(buf)-nPad], nil
	case cipherRC4:
		c, _ := rc4.NewCipher(key)
		c.XORKeyStream(buf, buf)
		return buf, nil
	default:
		panic("unknown cipher")
	}
}

func (enc *encryptInfo) EncryptStream(ref Reference, w io.WriteCloser) (io.WriteCloser, error) {
	cf := enc.stmF
	if cf == nil {
		return w, nil
	}

	key, err := enc.sec.KeyForRef(cf, ref)
	if err != nil {
		return nil, err
	}

	switch cf.Cipher {
	case cipherAES:
		c, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}

		// generate and write the IV
		iv := make([]byte, 16)
		_, err = io.ReadFull(rand.Reader, iv)
		if err != nil {
			return nil, err
		}
		_, err = w.Write(iv)
		if err != nil {
			return nil, err
		}

		return &encryptWriter{
			w:   w,
			cbc: cipher.NewCBCEncrypter(c, iv),
			buf: iv,
		}, nil
	case cipherRC4:
		c, _ := rc4.NewCipher(key)
		return &cipher.StreamWriter{S: c, W: w}, nil
	default:
		panic("unknown cipher")
	}
}

func (enc *encryptInfo) DecryptStream(ref Reference, r io.Reader) (io.Reader, error) {
	cf := enc.stmF
	if cf == nil {
		return r, nil
	}

	key, err := enc.sec.KeyForRef(cf, ref)
	if err != nil {
		return nil, err
	}

	switch cf.Cipher {
	case cipherRC4:
		c, _ := rc4.NewCipher(key)
		return &cipher.StreamReader{S: c, R: r}, nil
	case cipherAES:
		buf := make([]byte, 32)
		iv := buf[:16]
		_, err := io.ReadFull(r, iv)
		if err != nil {
			return nil, err
		}

		c, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}

		return &decryptReader{
			cbc: cipher.NewCBCDecrypter(c, iv),
			r:   r,
			buf: buf,
		}, nil
	default:
		panic("unknown cipher")
	}
}

// The stdSecHandler authenticates the user via a pair of passwords.
// The "user password" is used to access the contents of the document, the
// "owner password" can be used to control additional permissions, e.g.
// permission to print the document.
//
// This represents the PDF standard security handler, which is specified in
// section 7.6.3 of PDF 32000-1:2008.
type stdSecHandler struct {
	// R specified the revision of the standard security handler used.
	R int

	// ID is the original PDF document ID, i.e. the first element of the ID
	// array in the trailer dictionary.
	ID []byte

	// O is a byte string, based on the owner password, that is used in
	// computing the file encryption key and in determining whether a valid
	// owner password was entered.
	O []byte

	// U is a byte string, based on the owner and user password, that is used
	// in determining whether to prompt the user for a password and, if so,
	// whether a valid user or owner password was entered.
	U []byte

	OE []byte

	UE []byte

	Perms []byte

	// P is a set of flags specifying which operations shall be permitted when
	// the document is opened with user access.
	P uint32

	keyBytes int

	readPwd func([]byte, int) string
	key     []byte

	// unencryptedMetaData specifies whether document-level XMP metadata
	// streams are encrypted.
	//
	// We use the negation of /EncryptMetadata from the PDF spec, so that
	// the Go default value (unencryptedMetaData==false) corresponds to the
	// PDF default value (/EncryptMetadata true).
	unencryptedMetaData bool

	ownerAuthenticated bool
}

// openStdSecHandler creates a new stdSecHandler from the encryption dictionary
// and the document ID.  This is used when reading existing PDF documents.
func openStdSecHandler(enc Dict, keyBytes int, ID []byte, readPwd func([]byte, int) string) (*stdSecHandler, error) {
	R, ok := enc["R"].(Integer)
	if !ok || R < 2 || R == 5 || R > 6 {
		return nil, errors.New("invalid Encrypt.R")
	}
	ouLength := 32
	if R == 6 {
		ouLength = 48
	}

	// V has already been checked when this function is called, so we know
	// it is present and valid.
	V := enc["V"].(Integer)

	O, ok := enc["O"].(String)
	if !ok || len(O) != ouLength {
		return nil, errors.New("invalid Encrypt.O")
	}

	U, ok := enc["U"].(String)
	if !ok || len(U) != ouLength {
		return nil, errors.New("invalid Encrypt.U")
	}

	P, ok := enc["P"].(Integer)
	if !ok {
		return nil, errors.New("invalid Encrypt.P")
	}

	emd := true
	if obj, ok := enc["EncryptMetaData"].(Bool); ok && V >= 4 {
		emd = bool(obj)
	}

	sec := &stdSecHandler{
		ID:       ID,
		keyBytes: keyBytes,
		readPwd:  readPwd,

		R: int(R),
		O: []byte(O),
		U: []byte(U),
		P: uint32(P),

		unencryptedMetaData: !emd,
	}

	if R == 6 {
		OE, ok := enc["OE"].(String)
		if !ok || len(OE) != 32 {
			return nil, errors.New("invalid Encrypt.OE")
		}
		sec.OE = []byte(OE)

		UE, ok := enc["UE"].(String)
		if !ok || len(UE) != 32 {
			return nil, errors.New("invalid Encrypt.UE")
		}
		sec.UE = []byte(UE)

		Perms, ok := enc["Perms"].(String)
		if !ok || len(Perms) != 16 {
			return nil, errors.New("invalid Encrypt.Perms")
		}
		sec.Perms = []byte(Perms)
	}

	return sec, nil
}

// createStdSecHandler allocates a new, pre-authenticated PDF Standard Security
// Handler.  This is used when creating new PDF documents.
func createStdSecHandler(id []byte, userPwd, ownerPwd string, perm Perm, length, V int) (*stdSecHandler, error) {
	if ownerPwd == "" {
		ownerPwd = userPwd
	}

	var R int
	switch {
	case V < 2 && perm.canR2():
		R = 2
	case V <= 3:
		R = 3
	case V == 4:
		R = 4
	case V == 5:
		R = 6
	default:
		return nil, &MalformedFileError{
			Err: errors.New("invalid Encrypt.V"),
		}
	}
	keyBytes := length / 8

	sec := &stdSecHandler{
		ID:       id,
		keyBytes: keyBytes,
		R:        R,
		P:        stdSecPermToP(perm),

		ownerAuthenticated: true,
	}

	switch R {
	case 2, 3, 4:
		paddedUserPwd, err := padPasswd(userPwd)
		if err != nil {
			return nil, err
		}
		paddedOwnerPwd, err := padPasswd(ownerPwd)
		if err != nil {
			return nil, err
		}
		sec.O, err = sec.computeO(paddedUserPwd, paddedOwnerPwd)
		if err != nil {
			return nil, err
		}
		fileEncryptionKey := sec.computeFileEncyptionKey(paddedUserPwd)
		sec.U = sec.computeU(fileEncryptionKey)
		sec.key = fileEncryptionKey
	case 6:
		utf8UserPwd, err := utf8Passwd(userPwd)
		if err != nil {
			return nil, err
		}
		utf8OwnerPwd, err := utf8Passwd(ownerPwd)
		if err != nil {
			return nil, err
		}
		sec.key = make([]byte, 32)
		_, err = rand.Read(sec.key)
		if err != nil {
			return nil, err
		}
		sec.U, sec.UE, err = sec.computeUAndUE(utf8UserPwd)
		if err != nil {
			return nil, err
		}
		sec.O, sec.OE, err = sec.computeOAndOE(utf8OwnerPwd)
		if err != nil {
			return nil, err
		}
		sec.Perms = sec.computePerms(sec.key)
	default:
		panic("unreachable")
	}

	return sec, nil
}

func (sec *stdSecHandler) KeyForRef(cf *cryptFilter, ref Reference) ([]byte, error) {
	key, err := sec.GetKey(false)
	if err != nil {
		return nil, err
	}
	switch sec.R {
	case 2, 3, 4:
		h := md5.New()
		h.Write(key)
		num := ref.Number()
		gen := ref.Generation()
		h.Write([]byte{
			byte(num), byte(num >> 8), byte(num >> 16),
			byte(gen), byte(gen >> 8)})
		if cf.Cipher == cipherAES {
			h.Write([]byte("sAlT"))
		}
		l := sec.keyBytes + 5
		if l > 16 {
			l = 16
		}
		return h.Sum(nil)[:l], nil
	case 6:
		return key, nil
	default:
		panic("invalid R")
	}
}

// GetKey returns the file encryption key to decrypt string and stream data.
// Passwords will be requested via the getPasswd callback.  If the correct
// owner password was supplied, the OwnerAuthenticated field will be set to
// true, in addition to returning the key.
func (sec *stdSecHandler) GetKey(needOwner bool) ([]byte, error) {
	if sec.key != nil && (sec.ownerAuthenticated || !needOwner) {
		return sec.key, nil
	}

	passwd := ""
	passWdTry := 0
	for { // try different passwords until one succeeds
		if sec.R < 6 {
			padded, err := padPasswd(passwd)
			if err != nil {
				continue
			}
			err = sec.authenticateOwner(padded)
			if err == nil {
				return sec.key, nil
			}
			if !needOwner {
				err = sec.authenticateUser(padded)
				if err == nil {
					return sec.key, nil
				}
			}
		} else {
			prepared, err := utf8Passwd(passwd)
			if err != nil {
				continue
			}
			err = sec.authenticateOwner6(prepared)
			if err == nil {
				return sec.key, nil
			}
			if !needOwner {
				err = sec.authenticateUser6(prepared)
				if err == nil {
					return sec.key, nil
				}
			}
		}

		// wrong password, try another one
		if sec.readPwd != nil {
			passwd = sec.readPwd(sec.ID, passWdTry)
			passWdTry++
		} else {
			passwd = ""
		}
		if passwd == "" {
			return nil, &AuthenticationError{sec.ID}
		}
	}
}

// Algorithm 2: compute the file encryption key for R <= 4.
// pw must be the padded password.
func (sec *stdSecHandler) computeFileEncyptionKey(paddedUserPwd []byte) []byte {
	h := md5.New()
	h.Write(paddedUserPwd)
	h.Write(sec.O)
	h.Write([]byte{
		byte(sec.P), byte(sec.P >> 8), byte(sec.P >> 16), byte(sec.P >> 24)})
	h.Write(sec.ID)
	if sec.unencryptedMetaData && sec.R >= 4 {
		h.Write([]byte{255, 255, 255, 255})
	}
	key := h.Sum(nil)

	if sec.R >= 3 {
		for i := 0; i < 50; i++ {
			h.Reset()
			h.Write(key[:sec.keyBytes])
			key = h.Sum(key[:0])
		}
	}

	return key[:sec.keyBytes]
}

// Algorithm 2.B: Computing a hash (revision 6 and later)
func slowHash(passwd, salt, U []byte) []byte {
	// Take the SHA-256 hash of the original input to the algorithm and name
	// the resulting 32 bytes K.
	h := sha256.New()
	h.Write(passwd)
	h.Write(salt)
	h.Write(U)
	K := h.Sum(nil)

	K1 := make([]byte, 64*(len(passwd)+64+len(U)))

	// Perform the following steps (a)-(d) 64 times:
	for i := 0; i < 64 || K1[len(K1)-1] > byte(i-32); i++ {
		// a) Make a new string, K1, consisting of 64 repetitions of the sequence:
		// input password, K, the 48-byte user key. The 48 byte user key is only
		// used when checking the owner password or creating the owner key. If
		// checking the user password or creating the user key, K1 is the
		// concatenation of the input password and K.
		K1 = K1[:0]
		for j := 0; j < 64; j++ {
			K1 = append(K1, passwd...)
			K1 = append(K1, K...)
			K1 = append(K1, U...)
		}

		// b) Encrypt K1 with the AES-128 (CBC, no padding) algorithm, using the
		// first 16 bytes of K as the key and the second 16 bytes of K as the
		// initialization vector. The result of this encryption is E.
		c, _ := aes.NewCipher(K[:16])
		cbc := cipher.NewCBCEncrypter(c, K[16:32])
		// The length of K1 is a multiple of 64, so this is safe.
		cbc.CryptBlocks(K1, K1)

		// c) Taking the first 16 bytes of E as an unsigned big-endian integer,
		// compute the remainder, modulo 3.
		// Since (a*256)%3 = (a*255)%3+a%3 = a%3, we can just add all bytes.
		var rem int
		for _, b := range K1[:16] {
			rem += int(b)
		}
		rem %= 3

		// If the result is 0, the next hash used is SHA-256,
		// if the result is 1, the next hash used is SHA-384,
		// if the result is 2, the next hash used is SHA-512.
		var h hash.Hash
		switch rem {
		case 0:
			h = sha256.New()
		case 1:
			h = sha512.New384()
		case 2:
			h = sha512.New()
		}

		// d) Using the hash algorithm determined in step c, take the hash of E.
		// The result is a new value of K, which will be 32, 48, or 64 bytes in
		// length.
		h.Write(K1)
		K = h.Sum(K[:0])

		// Repeat the process (a-d) with this new value for K. Following 64 rounds
		// (round number 0 to round number 63), do the following, starting with
		// round number 64:
		//
		// e) Look at the very last byte of E. If the value of that byte (taken as
		// an unsigned integer) is greater than the round number - 32, repeat steps
		// (a-d) again.
		//
		// f) Repeat steps (a-e) until the value of the last byte is ≤ (round
		// number) - 32.
	}

	// The first 32 bytes of the final K are the output of the algorithm.
	return K[:32]
}

// algorithm 3: compute O.
// The algorithm is documented in section 7.6.3.4 of ISO 32000-1:2008.
func (sec *stdSecHandler) computeO(paddedUserPwd, paddedOwnerPwd []byte) ([]byte, error) {
	h := md5.New()
	h.Write(paddedOwnerPwd)
	sum := h.Sum(nil)
	if sec.R >= 3 {
		for i := 0; i < 50; i++ {
			h.Reset()
			// The spec does not mention the truncation, but this seems to be
			// required anyway.
			h.Write(sum[:sec.keyBytes])
			sum = h.Sum(sum[:0])
		}
	}
	rc4key := sum[:sec.keyBytes]

	c, _ := rc4.NewCipher(rc4key)
	O := make([]byte, 32)
	c.XORKeyStream(O, paddedUserPwd)
	if sec.R >= 3 {
		key := make([]byte, len(rc4key))
		for i := byte(1); i <= 19; i++ {
			for j := range key {
				key[j] = rc4key[j] ^ i
			}
			c, _ = rc4.NewCipher(key)
			c.XORKeyStream(O, O)
		}
	}
	return O, nil
}

// Algorithm 4/5: compute U.
func (sec *stdSecHandler) computeU(fileEncyptionKey []byte) []byte {
	U := make([]byte, 32)
	switch sec.R {
	case 2:
		c, _ := rc4.NewCipher(fileEncyptionKey)
		c.XORKeyStream(U, passwdPad)
	case 3, 4:
		h := md5.New()
		h.Write(passwdPad)
		h.Write(sec.ID)
		U = h.Sum(U[:0])
		c, _ := rc4.NewCipher(fileEncyptionKey)
		c.XORKeyStream(U, U)

		tmpKey := make([]byte, len(fileEncyptionKey))
		for i := byte(1); i <= 19; i++ {
			for j := range tmpKey {
				tmpKey[j] = fileEncyptionKey[j] ^ i
			}
			c, _ = rc4.NewCipher(tmpKey)
			c.XORKeyStream(U, U)
		}
		// This gives the first 16 bytes of U, the remaining 16 bytes
		// are "arbitrary padding".
		U = append(U[:16],
			0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0)
	default:
		panic("invalid security handler revision")
	}

	return U
}

// Algorithm 6: Authenticating the user password (Security handlers of revision 4 and earlier)
func (sec *stdSecHandler) authenticateUser(paddedUserPwd []byte) error {
	key := sec.computeFileEncyptionKey(paddedUserPwd)
	U := sec.computeU(key)
	switch sec.R {
	case 2:
		if bytes.Equal(U, sec.U) {
			sec.key = key
			return nil
		}
	case 3, 4:
		if bytes.Equal(U[:16], sec.U[:16]) {
			sec.key = key
			return nil
		}
	default:
		panic("invalid security handler revision")
	}
	return &AuthenticationError{sec.ID}
}

// Algorithm 7: Authenticating the owner password (Security handlers of revision 4 and earlier)
func (sec *stdSecHandler) authenticateOwner(paddedOwnerPwd []byte) error {
	h := md5.New()
	h.Write(paddedOwnerPwd)
	sum := h.Sum(nil)
	if sec.R >= 3 {
		for i := 0; i < 50; i++ {
			h.Reset()
			// The spec does not mention the truncation, but this seems to be
			// required anyway.
			h.Write(sum[:sec.keyBytes])
			sum = h.Sum(sum[:0])
		}
	}
	key := sum[:sec.keyBytes]

	buf := make([]byte, 32)
	copy(buf, sec.O)
	switch sec.R {
	case 2:
		c, _ := rc4.NewCipher(key)
		c.XORKeyStream(buf, buf)
	case 3, 4:
		tmpKey := make([]byte, len(key))
		for i := 19; i >= 0; i-- {
			for j := range tmpKey {
				tmpKey[j] = key[j] ^ byte(i)
			}
			c, _ := rc4.NewCipher(tmpKey)
			c.XORKeyStream(buf, buf)
		}
	}

	err := sec.authenticateUser(buf)
	if err != nil {
		return err
	}
	sec.ownerAuthenticated = true
	return nil
}

// Algorithm 8: Computing U and UE (Security handlers of revision 6)
func (sec *stdSecHandler) computeUAndUE(utf8UserPwd []byte) ([]byte, []byte, error) {
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		return nil, nil, err
	}

	out := slowHash(utf8UserPwd, buf[:8], nil) // user validation salt
	U := make([]byte, 0, 48)
	U = append(U, out...)
	U = append(U, buf...)

	key := slowHash(utf8UserPwd, buf[8:], nil) // user key salt
	c, _ := aes.NewCipher(key)
	cbc := cipher.NewCBCEncrypter(c, zero16)
	UE := make([]byte, 32)
	cbc.CryptBlocks(UE, sec.key)

	return U, UE, nil
}

// Algorithm 9: Computing O and OE (Security handlers of revision 6)
func (sec *stdSecHandler) computeOAndOE(utf8OwnerPwd []byte) ([]byte, []byte, error) {
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		return nil, nil, err
	}

	out := slowHash(utf8OwnerPwd, buf[:8], sec.U) // owner validation salt
	O := make([]byte, 0, 48)
	O = append(O, out...)
	O = append(O, buf...)

	key := slowHash(utf8OwnerPwd, buf[8:], sec.U) // owner key salt
	c, _ := aes.NewCipher(key)
	cbc := cipher.NewCBCEncrypter(c, zero16)
	OE := make([]byte, 32)
	cbc.CryptBlocks(OE, sec.key)

	return O, OE, nil
}

// Algorithm 10: Computing the Perms value (Security handlers of revision 6)
func (sec *stdSecHandler) computePerms(fileEncryptionKey []byte) []byte {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint32(buf, sec.P)
	buf[4] = 0xFF
	buf[5] = 0xFF
	buf[6] = 0xFF
	buf[7] = 0xFF
	if sec.unencryptedMetaData {
		buf[8] = 'F'
	} else {
		buf[8] = 'T'
	}
	buf[9] = 'a'
	buf[10] = 'd'
	buf[11] = 'b'

	c, _ := aes.NewCipher(fileEncryptionKey)
	c.Encrypt(buf, buf)
	return buf
}

// Algorithm 11: Authenticating the user password (Security handlers of revision 6)
func (sec *stdSecHandler) authenticateUser6(utf8Passwd []byte) error {
	hash := slowHash(utf8Passwd, sec.U[32:40], nil)
	if !bytes.Equal(hash, sec.U[:32]) {
		return &AuthenticationError{sec.ID}
	}

	key := slowHash(utf8Passwd, sec.U[40:48], nil) // user key salt
	c, _ := aes.NewCipher(key)
	cbc := cipher.NewCBCDecrypter(c, zero16)
	fileEncryptionKey := make([]byte, 32)
	cbc.CryptBlocks(fileEncryptionKey, sec.UE)

	err := sec.checkPerms(fileEncryptionKey)
	if err != nil {
		return err
	}

	sec.key = fileEncryptionKey
	return nil
}

// Algorithm 12: Authenticating the owner password (Security handlers of revision 6)
func (sec *stdSecHandler) authenticateOwner6(utf8Passwd []byte) error {
	hash := slowHash(utf8Passwd, sec.O[32:40], sec.U)
	if !bytes.Equal(hash, sec.O[:32]) {
		return &AuthenticationError{sec.ID}
	}

	key := slowHash(utf8Passwd, sec.O[40:48], sec.U) // owner key salt
	c, _ := aes.NewCipher(key)
	cbc := cipher.NewCBCDecrypter(c, zero16)
	fileEncryptionKey := make([]byte, 32)
	cbc.CryptBlocks(fileEncryptionKey, sec.OE)

	err := sec.checkPerms(fileEncryptionKey)
	if err != nil {
		return err
	}

	sec.key = fileEncryptionKey
	sec.ownerAuthenticated = true
	return nil
}

func (sec *stdSecHandler) checkPerms(fileEncryptionKey []byte) error {
	buf := make([]byte, 16)

	c, _ := aes.NewCipher(fileEncryptionKey)
	c.Decrypt(buf, sec.Perms)
	if !bytes.Equal(buf[9:12], []byte{'a', 'd', 'b'}) {
		return &AuthenticationError{sec.ID}
	}
	perms := binary.LittleEndian.Uint32(buf[:4])
	if perms != sec.P {
		return &AuthenticationError{sec.ID}
	}

	var emdCode byte
	if sec.unencryptedMetaData {
		emdCode = 'F'
	} else {
		emdCode = 'T'
	}
	if buf[8] != emdCode {
		return &AuthenticationError{sec.ID}
	}

	return nil
}

func utf8Passwd(passwd string) ([]byte, error) {
	prepped, err := stringprep.SASLprep.Prepare(passwd)
	if err != nil {
		return nil, errInvalidPassword
	}
	buf := []byte(prepped)
	if len(buf) > 127 {
		buf = buf[:127]
	}
	return buf, nil
}

// returns a slice of length 32
func padPasswd(passwd string) ([]byte, error) {
	buf, ok := pdfDocEncode(passwd)
	if !ok {
		return nil, errInvalidPassword
	}

	padded := make([]byte, 32)
	n := copy(padded, buf)
	copy(padded[n:], passwdPad)

	return padded, nil
}

var passwdPad = []byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41,
	0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80,
	0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

var zero16 = []byte{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}

func stdSecPToPerm(R int, P uint32) Perm {
	perm := PermAll
	if R == 2 {
		if P&(1<<(3-1)) == 0 {
			perm &= ^(PermPrint | PermPrintDegraded)
		}
	} else if R >= 3 {
		// bit 3 | 12
		//     0 | 0 -> neither full nor degraded printing
		//     0 | 1 -> full printing
		//     1 | 0 -> only degraded printing (full printing forbidden)
		//     1 | 1 -> full printing
		if P&(1<<(3-1)) == 0 && P&(1<<(12-1)) == 0 {
			perm &= ^(PermPrint | PermPrintDegraded)
		} else if P&(1<<(3-1)) != 0 && P&(1<<(12-1)) == 0 {
			perm &= ^PermPrint
		}
	}

	// bit 4 | 11
	//     0 | 0 -> no modifications, no assembly
	//     0 | 1 -> no modifications, assembly allowed
	//     1 | 0 -> modifications allowed, assembly allowed
	//     1 | 1 -> modifications allowed, assembly allowed
	if P&(1<<(4-1)) == 0 {
		perm &= ^PermModify
		if P&(1<<(11-1)) == 0 {
			perm &= ^PermAssemble
		}
	}

	if P&(1<<(5-1)) == 0 {
		perm &= ^PermCopy
	}

	// bit 6 | 9
	//     0 | 0 -> no annotations, don't fill form fields
	//     0 | 1 -> no annotations, fill form fields
	//     1 | 0 -> annotations allowed, fill form fields
	//     1 | 1 -> annotations allowed, fill form fields
	if P&(1<<(6-1)) == 0 {
		perm &= ^PermAnnotate
		if P&(1<<(9-1)) == 0 {
			perm &= ^PermForms
		}
	}

	return perm
}

func stdSecPermToP(perm Perm) uint32 {
	forbidden := uint32(3)
	if perm&PermCopy == 0 {
		forbidden |= 1 << (5 - 1)
	}
	if perm&PermPrint == 0 {
		forbidden |= 1 << (12 - 1)
		if perm&PermPrintDegraded == 0 {
			forbidden |= 1 << (3 - 1)
		}
	}
	if perm&PermAnnotate == 0 {
		forbidden |= 1 << (6 - 1)
		if perm&PermForms == 0 {
			forbidden |= 1 << (9 - 1)
		}
	}
	if perm&PermAssemble == 0 {
		forbidden |= 1 << (11 - 1)
	}
	if perm&PermModify == 0 {
		forbidden |= 1 << (4 - 1)
	}
	return ^forbidden
}

type encryptWriter struct {
	w   io.WriteCloser
	cbc cipher.BlockMode
	buf []byte // must have length cbc.BlockSize()
	pos int
}

func (w *encryptWriter) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		k := copy(w.buf[w.pos:], p)
		n += k
		w.pos += k
		p = p[k:]

		if w.pos >= len(w.buf) {
			w.cbc.CryptBlocks(w.buf, w.buf)
			_, err := w.w.Write(w.buf)
			if err != nil {
				return n, err
			}
			w.pos = 0
		}
	}
	return n, nil
}

func (w *encryptWriter) Close() error {
	// add the padding
	kPad := 16 - w.pos
	for i := w.pos; i < len(w.buf); i++ {
		w.buf[i] = byte(kPad)
	}

	// write the last block
	w.cbc.CryptBlocks(w.buf, w.buf)
	_, err := w.w.Write(w.buf)
	if err != nil {
		return err
	}

	return w.w.Close()
}

type decryptReader struct {
	cbc      cipher.BlockMode
	r        io.Reader
	buf      []byte
	ready    []byte
	reserved []byte
}

func (r *decryptReader) Read(p []byte) (int, error) {
	if len(r.ready) == 0 {
		k := copy(r.buf, r.reserved)
		for k <= 16 && r.r != nil {
			n, err := r.r.Read(r.buf[k:])
			k += n
			if err == io.EOF {
				r.r = nil
				if k%16 != 0 {
					return 0, errCorrupted
				}
			} else if err != nil {
				return 0, err
			}
		}

		if k < 16 {
			if k > 0 {
				panic("inconsistent buffer state")
			}
			return 0, io.EOF
		}

		l := k
		if r.r != nil {
			// reserve the last block, in case it turns out to be padding
			l--
		}
		l -= l % 16
		r.ready = r.buf[:l]
		r.reserved = r.buf[l:k]
		r.cbc.CryptBlocks(r.ready, r.ready)

		if r.r == nil {
			// remove the padding
			if l != k {
				panic("inconsistent buffer state")
			}
			nPad := int(r.buf[l-1])
			if nPad < 1 || nPad > 16 || nPad > l {
				return 0, errCorrupted
			}
			r.ready = r.ready[:l-nPad]
		}
	}

	n := copy(p, r.ready)
	r.ready = r.ready[n:]
	return n, nil
}

type cryptFilter struct {
	Cipher cipherType

	// Length is the key length in bits.
	Length int
}

func (cf *cryptFilter) String() string {
	return fmt.Sprintf("%s-%d", cf.Cipher, cf.Length)
}

func getCryptFilter(cryptFilterName Name, CF Dict) (*cryptFilter, error) {
	if cryptFilterName == "Identity" {
		return nil, nil
	}
	if cryptFilterName != "StdCF" {
		return nil, errors.New("unknown crypt filter " + string(cryptFilterName))
	}
	if CF == nil {
		return nil, errors.New("missing CF dictionary")
	}

	cfDict, ok := CF[cryptFilterName].(Dict)
	if !ok {
		return nil, errors.New("missing " + string(cryptFilterName) + " entry in CF dict")
	}

	res := &cryptFilter{}
	switch cfDict["CFM"] {
	case Name("V2"):
		res.Cipher = cipherRC4
		res.Length = 128
	case Name("AESV2"):
		res.Cipher = cipherAES
		res.Length = 128
	case Name("AESV3"):
		res.Cipher = cipherAES
		res.Length = 256
	default:
		return nil, errors.New("unknown cipher")
	}
	return res, nil
}

// cipherType denotes the type of encryption used in (parts of) a PDF file.
type cipherType int

const (
	// cipherUnknown indicates that the encryption scheme has not yet been
	// determined.
	cipherUnknown cipherType = iota

	// cipherRC4 indicates that RC4 encryption is used.  This corresponds to
	// the StdCF crypt filter with a CFM value of V2 in the PDF specification.
	cipherRC4

	// cipherAES indicates that AES encryption in CBC mode is used.  This
	// corresponds to the StdCF crypt filter with a CFM value of AESV2 or
	// AESV3.
	cipherAES
)

func (c cipherType) String() string {
	switch c {
	case cipherUnknown:
		return "unknown"
	case cipherRC4:
		return "RC4"
	case cipherAES:
		return "AES"
	default:
		return fmt.Sprintf("cipher#%d", c)
	}
}

// Perm describes which operations are permitted when accessing the document
// with User access (but not Owner access).  The user can always view the
// document.
//
// This library just reports the permissions as specified in the PDF file.
// It is up to the caller to enforce the permissions.
type Perm int

// canR2 checks whether the permissions can be represented by revision 2 of the
// standard security handler.
func (perm Perm) canR2() bool {
	if perm&PermPrint == 0 && perm&PermPrintDegraded != 0 {
		return false
	}
	if perm&PermAnnotate == 0 && perm&PermForms != 0 {
		return false
	}
	if perm&PermModify == 0 && perm&PermAssemble != 0 {
		return false
	}
	return true
}

const (
	// PermCopy allows to extract text and graphics.
	PermCopy Perm = 1 << iota

	// PermPrintDegraded allows printing of a low-level representation of the
	// appearance, possibly of degraded quality.
	PermPrintDegraded

	// PermPrint allows printing a representation from which a faithful digital
	// copy of the PDF content could be generated.  This implies
	// PermPrintDegraded.
	PermPrint

	// PermForms allows to fill in form fields, including signature fields.
	PermForms

	// PermAnnotate allows to add or modify text annotations. This implies
	// PermForms.
	PermAnnotate

	// PermAssemble allows to insert, rotate, or delete pages and to create
	// bookmarks or thumbnail images.
	PermAssemble

	// PermModify allows to modify the document.  This implies PermAssemble.
	PermModify

	permNext

	// PermAll gives the user all permissions, making User access equivalent to
	// Owner access.
	PermAll = permNext - 1
)
