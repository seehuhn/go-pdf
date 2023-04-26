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
	"errors"
	"fmt"
	"io"
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
	subFilter, err := GetName(r, enc["SubFilter"])
	if err != nil {
		return nil, err
	}

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
					Pos: r.errPos(encObj),
					Err: errors.New("unsupported Encrypt.Length value"),
				}
			}
		}

		res.stmF = cf
		res.strF = cf
		res.efF = cf

		keyBytes = cf.Length / 8 // TODO(voss): ???
	case 4, 5:
		var CF Dict
		if obj, ok := enc["CF"].(Dict); ok {
			CF = obj
		}

		if obj, ok := enc["StmF"].(Name); ok {
			ciph, err := getCipher(obj, CF)
			if err != nil {
				return nil, &MalformedFileError{
					Pos: r.errPos(encObj),
					Err: err,
				}
			}
			res.stmF = ciph
		}
		if obj, ok := enc["StrF"].(Name); ok {
			ciph, err := getCipher(obj, CF)
			if err != nil {
				return nil, &MalformedFileError{
					Pos: r.errPos(encObj),
					Err: err,
				}
			}
			res.strF = ciph
		}
		res.efF = res.stmF // default
		if obj, ok := enc["EFF"].(Name); ok {
			ciph, err := getCipher(obj, CF)
			if err != nil {
				return nil, &MalformedFileError{
					Pos: r.errPos(encObj),
					Err: err,
				}
			}
			res.efF = ciph
		}

		if V == 4 {
			keyBytes = 16
		} else {
			keyBytes = 32
		}

	default:
		return nil, &MalformedFileError{
			Pos: r.errPos(encObj),
			Err: errors.New("unsupported Encrypt.V value"),
		}
	}

	switch {
	case filter == "Standard" && subFilter == "":
		sec, err := openStdSecHandler(enc, keyBytes, r.ID[0], readPwd)
		if err != nil {
			return nil, &MalformedFileError{
				Pos: r.errPos(encObj),
				Err: err,
			}
		}
		res.sec = sec
		res.UserPermissions = stdSecPToPerm(sec.R, sec.P)
	default:
		return nil, &MalformedFileError{
			Pos: r.errPos(encObj),
			Err: errors.New("unsupported security handler"),
		}
	}

	return res, nil
}

func (enc *encryptInfo) ToDict(version Version) Dict {
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
			panic("invalid key length")
		}
	}
	if length == 0 {
		panic("not implemented: unequal key length")
	}
	if cipher == cipherUnknown {
		panic("not implemented: mixed ciphers")
	}

	if cipher == cipherAES256 && length == 256 && version >= V2_0 {
		// In PDF 2.0, all V values less than 5 are deprecated,
		// so we try this first.
		dict["V"] = Integer(5)
		dict["StmF"] = Name("StdCF")
		dict["StrF"] = Name("StdCF")
		dict["CF"] = Dict{
			"StdCF": Dict{"Length": Integer(length / 8), "CFM": Name("AESV3")},
		}
	} else if cipher == cipherRC4 && length == 40 {
		dict["V"] = Integer(1)
	} else if cipher == cipherRC4 && version >= V1_4 {
		dict["V"] = Integer(2)
		dict["Length"] = Integer(length)
	} else if cipher == cipherAES128 && version >= V1_6 {
		dict["V"] = Integer(4)
		dict["StmF"] = Name("StdCF")
		dict["StrF"] = Name("StdCF")
		dict["CF"] = Dict{
			"StdCF": Dict{"Length": Integer(length / 8), "CFM": Name("AESV2")},
		}
	} else {
		panic("no supported encryption scheme found")
	}

	sec := enc.sec
	dict["R"] = Integer(sec.R)
	dict["O"] = String(sec.o)
	dict["U"] = String(sec.u)
	dict["P"] = Integer(int32(sec.P))
	if sec.unencryptedMetaData {
		dict["EncryptMetadata"] = Bool(false)
	}

	return dict
}

func (enc *encryptInfo) keyForRef(cf *cryptFilter, ref Reference) ([]byte, error) {
	h := md5.New()
	key, err := enc.sec.GetKey(false)
	if err != nil {
		return nil, err
	}
	_, _ = h.Write(key)
	num := ref.Number()
	gen := ref.Generation()
	_, _ = h.Write([]byte{
		byte(num), byte(num >> 8), byte(num >> 16),
		byte(gen), byte(gen >> 8)})
	if cf.Cipher == cipherAES128 {
		_, _ = h.Write([]byte("sAlT"))
	}
	l := enc.sec.KeyBytes + 5
	if l > 16 {
		l = 16
	}
	return h.Sum(nil)[:l], nil
}

// EncryptBytes encrypts the bytes in buf using Algorithm 1 in the PDF spec.
// This function modfies the contents of buf and may return buf.
func (enc *encryptInfo) EncryptBytes(ref Reference, buf []byte) ([]byte, error) {
	cf := enc.strF
	if cf == nil {
		return buf, nil
	}

	key, err := enc.keyForRef(cf, ref)
	if err != nil {
		return nil, err
	}
	switch cf.Cipher {
	case cipherAES128:
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

	key, err := enc.keyForRef(cf, ref)
	if err != nil {
		return nil, err
	}
	switch cf.Cipher {
	case cipherAES128:
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

func (enc *encryptInfo) DecryptStream(ref Reference, r io.Reader) (io.Reader, error) {
	cf := enc.stmF
	if cf == nil {
		return r, nil
	}

	key, err := enc.keyForRef(cf, ref)
	if err != nil {
		return nil, err
	}

	switch cf.Cipher {
	case cipherAES128:
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
	case cipherRC4:
		c, _ := rc4.NewCipher(key)
		return &cipher.StreamReader{S: c, R: r}, nil
	default:
		panic("unknown cipher")
	}
}

// The stdSecHandler authenticates the user via a pair of passwords.
// The "user password" is used to access the contents of the document, the
// "owner password" can be used to control additional permissions, e.g.
// permission to print the document.
type stdSecHandler struct {
	id       []byte
	o        []byte
	u        []byte
	KeyBytes int

	readPwd func([]byte, int) string
	key     []byte

	// We use the negation of /EncryptMetadata from the PDF spec, so that
	// the Go default value (unencryptedMetaData==false) corresponds to the
	// PDF default value (/EncryptMetadata true).
	unencryptedMetaData bool

	R int
	P uint32

	OwnerAuthenticated bool
}

func openStdSecHandler(enc Dict, keyBytes int, ID []byte, readPwd func([]byte, int) string) (*stdSecHandler, error) {
	R, ok := enc["R"].(Integer)
	if !ok || R < 2 || R > 4 {
		return nil, errors.New("invalid Encrypt.R")
	}
	O, ok := enc["O"].(String)
	if !ok || len(O) != 32 {
		return nil, errors.New("invalid Encrypt.O")
	}
	U, ok := enc["U"].(String)
	if !ok || len(U) != 32 {
		return nil, errors.New("invalid Encrypt.U")
	}
	P, ok := enc["P"].(Integer)
	if !ok {
		return nil, errors.New("invalid Encrypt.P")
	}
	emd := true
	if obj, ok := enc["EncryptMetaData"].(Bool); ok && R == 4 {
		emd = bool(obj)
	}

	sec := &stdSecHandler{
		id:       ID,
		KeyBytes: keyBytes,
		readPwd:  readPwd,

		R: int(R),
		o: []byte(O),
		u: []byte(U),
		P: uint32(P),

		unencryptedMetaData: !emd,
	}
	return sec, nil
}

// createStdSecHandler allocates a new, pre-authenticated PDF Standard Security
// Handler.
func createStdSecHandler(id []byte, userPwd, ownerPwd string, perm Perm, V int) *stdSecHandler {
	var R int
	if V < 2 && perm.canR2() {
		R = 2
	} else if V <= 3 {
		R = 3
	} else {
		R = 4
	}
	keyBytes := 16
	if V == 1 {
		keyBytes = 5
	}
	sec := &stdSecHandler{
		id:       id,
		KeyBytes: keyBytes,
		R:        R,
		P:        stdSecPermToP(perm),

		OwnerAuthenticated: true,
	}
	sec.o = sec.computeO(userPwd, ownerPwd)

	key := sec.computeKey(nil, padPasswd(userPwd))
	sec.u = sec.computeU(make([]byte, 32), key)
	sec.key = key

	return sec
}

// GetKey returns the key to decrypt string and stream data.  Passwords will
// be requested via the getPasswd callback.  If the correct owner password was
// supplied, the OwnerAuthenticated field will be set to true, in addition to
// returning the key.
func (sec *stdSecHandler) GetKey(needOwner bool) ([]byte, error) {
	// TODO(voss): key length for crypt filters???
	if sec.key != nil && (sec.OwnerAuthenticated || !needOwner) {
		return sec.key, nil
	}

	key := make([]byte, 16)
	u := make([]byte, 32)

	passwd := ""
	passWdTry := 0
	for {
		for try := 0; try < 2; try++ {
			// try == 0: check whether passwd is the owner password
			// try == 1: check whether passwd is the user password

			pw := padPasswd(passwd)

			if try == 0 {
				// try to decrypt sec.O
				h := md5.New()
				_, _ = h.Write(pw)
				sum := h.Sum(nil)
				if sec.R >= 3 {
					for i := 0; i < 50; i++ {
						h.Reset()
						_, _ = h.Write(sum) // sum[:sec.n]?
						sum = h.Sum(sum[:0])
					}
				}
				key := sum[:sec.KeyBytes]

				copy(pw, sec.o)
				if sec.R >= 3 {
					tmpKey := make([]byte, len(key))
					for i := byte(19); i > 0; i-- {
						for j := range tmpKey {
							tmpKey[j] = key[j] ^ i
						}
						c, _ := rc4.NewCipher(tmpKey)
						c.XORKeyStream(pw, pw)
					}
				}
				c, _ := rc4.NewCipher(key)
				c.XORKeyStream(pw, pw)
			}

			key = sec.computeKey(key, pw)
			u = sec.computeU(u, key)
			var ok bool
			if sec.R >= 3 {
				ok = bytes.Equal(sec.u[:16], u[:16])
			} else {
				ok = bytes.Equal(sec.u, u)
			}

			if ok {
				sec.key = key
				if try == 0 {
					sec.OwnerAuthenticated = true
				}
				return key, nil
			}

			if needOwner {
				break
			}
		}

		// wrong password, try another one
		if sec.readPwd != nil {
			passwd = sec.readPwd(sec.id, passWdTry)
			passWdTry++
		} else {
			passwd = ""
		}
		if passwd == "" {
			return nil, &AuthenticationError{sec.id}
		}
	}
}

// Algorithm 2: compute the encryption key.
// key must either be nil or have length of at least 16 bytes.
// pw must be the padded password
func (sec *stdSecHandler) computeKey(key []byte, pw []byte) []byte {
	h := md5.New()
	_, _ = h.Write(pw)
	_, _ = h.Write(sec.o)
	_, _ = h.Write([]byte{
		byte(sec.P), byte(sec.P >> 8), byte(sec.P >> 16), byte(sec.P >> 24)})
	_, _ = h.Write(sec.id)
	if sec.unencryptedMetaData {
		_, _ = h.Write([]byte{255, 255, 255, 255})
	}
	key = h.Sum(key[:0])

	if sec.R >= 3 {
		for i := 0; i < 50; i++ {
			h.Reset()
			_, _ = h.Write(key[:sec.KeyBytes])
			key = h.Sum(key[:0])
		}
	}

	return key[:sec.KeyBytes]
}

// this uses only the .R field of sec.
func (sec *stdSecHandler) computeO(userPasswd, ownerPasswd string) []byte {
	if ownerPasswd == "" {
		ownerPasswd = userPasswd
	}
	pwo := padPasswd(ownerPasswd)

	h := md5.New()
	_, _ = h.Write(pwo)
	sum := h.Sum(nil)
	if sec.R >= 3 {
		for i := 0; i < 50; i++ {
			h.Reset()
			_, _ = h.Write(sum[:sec.KeyBytes])
			sum = h.Sum(sum[:0])
		}
	}
	rc4key := sum[:sec.KeyBytes]

	c, _ := rc4.NewCipher(rc4key)
	o := padPasswd(userPasswd)
	c.XORKeyStream(o, o)
	if sec.R >= 3 {
		key := make([]byte, len(rc4key))
		for i := byte(1); i <= 19; i++ {
			for j := range key {
				key[j] = rc4key[j] ^ i
			}
			c, _ = rc4.NewCipher(key)
			c.XORKeyStream(o, o)
		}
	}
	return o
}

// Algorithm 4/5: compute U.
// U must be a slice of length 32.
// key must be the encryption key computed from the user password.
func (sec *stdSecHandler) computeU(U []byte, key []byte) []byte {
	c, _ := rc4.NewCipher(key)

	if sec.R < 3 {
		copy(U, passwdPad)
		c.XORKeyStream(U, U)
	} else {
		h := md5.New()
		_, _ = h.Write(passwdPad)
		_, _ = h.Write(sec.id)
		U = h.Sum(U[:0])
		c.XORKeyStream(U, U)

		tmpKey := make([]byte, len(key))
		for i := byte(1); i <= 19; i++ {
			for j := range tmpKey {
				tmpKey[j] = key[j] ^ i
			}
			c, _ = rc4.NewCipher(tmpKey)
			c.XORKeyStream(U, U)
		}
		// This gives the first 16 bytes of U, the remaining 16 bytes
		// are "arbitrary padding".
		U = append(U[:16], 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	}

	return U
}

// returns a slice of length 32
func padPasswd(passwd string) []byte {
	pw := make([]byte, 32)
	i := 0
	for _, r := range passwd {
		// Convert password to Latin-1.  We do this by just taking the
		// lower byte of every rune.
		pw[i] = byte(r)
		i++

		if i >= 32 {
			break
		}
	}

	copy(pw[i:], passwdPad)

	return pw
}

var passwdPad = []byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

func stdSecPToPerm(R int, P uint32) Perm {
	perm := PermAll
	if R == 2 {
		if P&(1<<(3-1)) == 0 {
			perm &= ^(PermPrint | PermPrintDegraded)
		}
	} else if R >= 3 {
		// bit 3 | 12
		//     0 | 0 -> no printing (both forbidden)
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

func (enc *encryptInfo) cryptFilter(ref Reference, w io.WriteCloser) (io.WriteCloser, error) {
	cf := enc.stmF
	if cf == nil {
		return w, nil
	}

	key, err := enc.keyForRef(cf, ref)
	if err != nil {
		return nil, err
	}

	switch cf.Cipher {
	case cipherAES128:
		c, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}

		// generate and write the IV
		buf := make([]byte, 16)
		_, err = io.ReadFull(rand.Reader, buf)
		if err != nil {
			return nil, err
		}
		_, err = w.Write(buf)
		if err != nil {
			return nil, err
		}

		return &encryptWriter{
			w:   w,
			cbc: cipher.NewCBCEncrypter(c, buf),
			buf: buf,
		}, nil
	case cipherRC4:
		c, _ := rc4.NewCipher(key)
		return &cipher.StreamWriter{S: c, W: w}, nil
	default:
		panic("unknown cipher")
	}
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
	Length int
}

func (cf *cryptFilter) String() string {
	return fmt.Sprintf("%s-%d", cf.Cipher, cf.Length)
}

// cipherType denotes the type of encryption used in (parts of) a PDF file.
type cipherType int

const (
	// cipherUnknown indicates that the encryption scheme has not yet been
	// determined.
	cipherUnknown cipherType = iota

	// cipherAES128 indicates that AES encryption in CBC mode is used.  This
	// corresponds to the StdCF crypt filter with a CFM value of AESV2 in the
	// PDF specification.
	cipherAES128

	// cipherAES256 indicates that AES encryption in CBC mode is used.  This
	// corresponds to the StdCF crypt filter with a CFM value of AESV3 in the
	// PDF specification.
	cipherAES256

	// cipherRC4 indicates that RC4 encryption is used.  This corresponds to
	// the StdCF crypt filter with a CFM value of V2 in the PDF specification.
	cipherRC4
)

func (c cipherType) String() string {
	switch c {
	case cipherUnknown:
		return "unknown"
	case cipherAES128:
		return "AES"
	case cipherRC4:
		return "RC4"
	default:
		return fmt.Sprintf("cipher#%d", c)
	}
}

func getCipher(name Name, CF Dict) (*cryptFilter, error) {
	if name == "Identity" {
		return nil, nil
	}
	if name != "StdCF" {
		return nil, errors.New("unknown crypt filter " + string(name))
	}
	if CF == nil {
		return nil, errors.New("missing CF dictionary")
	}

	cfDict, ok := CF[name].(Dict)
	if !ok {
		return nil, errors.New("missing " + string(name) + " entry in CF dict")
	}

	res := &cryptFilter{}
	if obj, ok := cfDict["Length"].(Integer); ok {
		res.Length = int(obj) * 8
	} else {
		res.Length = 40 // TODO(voss): is this the correct default?
	}
	if res.Length < 40 || res.Length > 256 || res.Length%8 != 0 {
		return nil, errors.New("invalid key length")
	}
	switch cfDict["CFM"] {
	case Name("V2"):
		res.Cipher = cipherRC4
	case Name("AESV2"):
		res.Cipher = cipherAES128
	case Name("AESV3"):
		res.Cipher = cipherAES256
		res.Length = 256
	default:
		return nil, errors.New("unknown cipher")
	}
	return res, nil
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
		return true
	}
	if perm&PermAnnotate == 0 && perm&PermForms != 0 {
		return true
	}
	if perm&PermModify == 0 && perm&PermAssemble != 0 {
		return true
	}
	return false
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
