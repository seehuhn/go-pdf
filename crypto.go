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
	stmF *cryptFilter
	strF *cryptFilter
	eff  *cryptFilter

	sec *securityHandler
}

func (r *Reader) parseEncryptDict(encObj Object) (*encryptInfo, error) {
	enc, err := r.GetDict(encObj)
	if err != nil {
		return nil, err
	}
	if len(r.ID) != 2 {
		return nil, &MalformedFileError{Err: errors.New("found Encrypt but no ID")}
	}

	res := &encryptInfo{}

	filter, err := r.GetName(enc["Filter"])
	if err != nil {
		return nil, err
	}
	var subFilter string
	if obj, ok := enc["SubFilter"].(Name); ok {
		subFilter = string(obj)
	}
	if filter != "Standard" || subFilter != "" {
		return nil, &MalformedFileError{
			Pos: r.errPos(encObj),
			Err: errors.New("unsupported security handler"),
		}
	}

	V, _ := enc["V"].(Integer)

	length := 40
	if obj, ok := enc["Length"].(Integer); ok && V > 1 {
		length = int(obj)
		if length < 40 || length > 128 || length%8 != 0 {
			return nil, &MalformedFileError{
				Pos: r.errPos(encObj),
				Err: errors.New("unsupported Encrypt.Length value"),
			}
		}
	}

	switch V {
	case 1:
		cf := &cryptFilter{
			Cipher: cipherRC4,
			Length: 40,
		}
		res.stmF = cf
		res.strF = cf
		res.eff = cf
	case 2:
		cf := &cryptFilter{
			Cipher: cipherRC4,
			Length: length,
		}
		res.stmF = cf
		res.strF = cf
		res.eff = cf
	case 4:
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
		res.eff = res.stmF
		if obj, ok := enc["EFF"].(Name); ok {
			ciph, err := getCipher(obj, CF)
			if err != nil {
				return nil, &MalformedFileError{
					Pos: r.errPos(encObj),
					Err: err,
				}
			}
			res.eff = ciph
		}
	default:
		return nil, &MalformedFileError{
			Pos: r.errPos(encObj),
			Err: errors.New("unsupported Encrypt.V value"),
		}
	}

	// TODO(voss): move the following code into a new function
	// newSecurityHandlerFromDict or so.

	R, err := r.GetInt(enc["R"])
	if err != nil || R < 2 || R > 4 {
		return nil, &MalformedFileError{
			Pos: r.errPos(encObj),
			Err: errors.New("invalid Encrypt.R"),
		}
	}
	O, err := r.GetString(enc["O"])
	if err != nil || len(O) != 32 {
		return nil, &MalformedFileError{
			Pos: r.errPos(encObj),
			Err: errors.New("invalid Encrypt.O"),
		}
	}
	U, err := r.GetString(enc["U"])
	if err != nil || len(U) != 32 {
		return nil, &MalformedFileError{
			Pos: r.errPos(encObj),
			Err: errors.New("invalid Encrypt.U"),
		}
	}
	P, err := r.GetInt(enc["P"])
	if err != nil {
		return nil, &MalformedFileError{
			Pos: r.errPos(encObj),
			Err: errors.New("invalid Encrypt.P"),
		}
	}

	emd := true
	if obj, ok := enc["EncryptMetaData"].(Bool); ok && R == 4 {
		emd = bool(obj)
	}

	res.sec = &securityHandler{
		id: []byte(r.ID[0]),
		n:  length / 8,
		R:  int(R),
		o:  []byte(O),
		u:  []byte(U),
		P:  uint32(P),

		encryptMetaData: emd,
	}

	return res, nil
}

func (sec *securityHandler) keyForRef(cf *cryptFilter, ref *Reference) ([]byte, error) {
	h := md5.New()
	key, err := sec.GetKey(false)
	if err != nil {
		return nil, err
	}
	_, _ = h.Write(key)
	_, _ = h.Write([]byte{
		byte(ref.Number), byte(ref.Number >> 8), byte(ref.Number >> 16),
		byte(ref.Generation), byte(ref.Generation >> 8)})
	if cf.Cipher == cipherAES {
		_, _ = h.Write([]byte("sAlT"))
	}
	l := sec.n + 5
	if l > 16 {
		l = 16
	}
	return h.Sum(nil)[:l], nil
}

// EncryptBytes encrypts the bytes in buf using Algorithm 1 in the PDF spec.
// This function modfies the contents of buf and may return buf.
func (enc *encryptInfo) EncryptBytes(ref *Reference, buf []byte) ([]byte, error) {
	cf := enc.strF
	if cf == nil {
		return buf, nil
	}

	key, err := enc.sec.keyForRef(cf, ref)
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
func (enc *encryptInfo) DecryptBytes(ref *Reference, buf []byte) ([]byte, error) {
	cf := enc.strF
	if cf == nil {
		return buf, nil
	}

	key, err := enc.sec.keyForRef(cf, ref)
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

func (enc *encryptInfo) EncryptStream(ref *Reference, _ string, r io.Reader) (io.Reader, error) {
	// TODO(voss): implement the name argument

	cf := enc.stmF
	if cf == nil {
		return r, nil
	}

	key, err := enc.sec.keyForRef(cf, ref)
	if err != nil {
		return nil, err
	}

	switch cf.Cipher {
	case cipherAES:
		c, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}

		buf := make([]byte, 32)
		iv := buf[:16]
		_, err = io.ReadFull(rand.Reader, iv)
		if err != nil {
			return nil, err
		}

		return &encryptReader{
			cbc:   cipher.NewCBCEncrypter(c, iv),
			r:     r,
			buf:   buf,
			ready: iv,
		}, nil
	case cipherRC4:
		c, _ := rc4.NewCipher(key)
		return &cipher.StreamReader{S: c, R: r}, nil
	default:
		panic("unknown cipher")
	}
}

func (enc *encryptInfo) DecryptStream(ref *Reference, _ string, r io.Reader) (io.Reader, error) {
	// TODO(voss): implement the name argument

	cf := enc.stmF
	if cf == nil {
		return r, nil
	}

	key, err := enc.sec.keyForRef(cf, ref)
	if err != nil {
		return nil, err
	}

	switch cf.Cipher {
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
	case cipherRC4:
		c, _ := rc4.NewCipher(key)
		return &cipher.StreamReader{S: c, R: r}, nil
	default:
		panic("unknown cipher")
	}
}

// The securityHandler authenticates the user via a pair of passwords.
// The "user password" is used to access the contents of the document, the
// "owner password" can be used to control additional permissions, e.g.
// permission to print the document.
type securityHandler struct {
	id []byte
	o  []byte
	u  []byte
	n  int

	getPasswd func(needOwner bool) string
	key       []byte

	encryptMetaData bool

	R int
	P uint32

	OwnerAuthenticated bool
}

// newSecurityHandler allocates a new, pre-authenticated StandardSecurityHandler.
func newSecurityHandler(id []byte, userPwd, ownerPwd string, P uint32) *securityHandler {
	sec := &securityHandler{
		id: id,
		n:  16,
		R:  4,
		P:  P,

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
func (sec *securityHandler) GetKey(needOwner bool) ([]byte, error) {
	// TODO(voss): key length for crypt filters???
	if sec.key != nil {
		return sec.key, nil
	}

	key := make([]byte, 16)
	u := make([]byte, 32)

	passwd := ""
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
				key := sum[:sec.n]

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
		if sec.getPasswd != nil {
			passwd = sec.getPasswd(needOwner)
		} else {
			passwd = ""
		}
		if passwd == "" {
			return nil, ErrWrongPassword
		}
	}
}

// algorithm 2: compute the encryption key
// key must either be nil or have length of at least 16 bytes.
// pw must be the padded password
func (sec *securityHandler) computeKey(key []byte, pw []byte) []byte {
	h := md5.New()
	_, _ = h.Write(pw)
	_, _ = h.Write(sec.o)
	_, _ = h.Write([]byte{
		byte(sec.P), byte(sec.P >> 8), byte(sec.P >> 16), byte(sec.P >> 24)})
	_, _ = h.Write(sec.id)
	if !sec.encryptMetaData {
		_, _ = h.Write([]byte{255, 255, 255, 255})
	}
	key = h.Sum(key[:0])

	if sec.R >= 3 {
		for i := 0; i < 50; i++ {
			h.Reset()
			_, _ = h.Write(key[:sec.n])
			key = h.Sum(key[:0])
		}
	}

	return key[:sec.n]
}

// this uses only the .R field of sec.
func (sec *securityHandler) computeO(userPasswd, ownerPasswd string) []byte {
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
			_, _ = h.Write(sum[:sec.n])
			sum = h.Sum(sum[:0])
		}
	}
	rc4key := sum[:sec.n]

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

// algorithm 4/5: compute U
// U must be a slice of length 32.
// key must be the encryption key computed from the user password.
func (sec *securityHandler) computeU(U []byte, key []byte) []byte {
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
				tmpKey[j] = key[j] ^ byte(i)
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
		// Convert password to Latin-1.  We can do this by just taking the
		// lower byte of every rune.
		// TODO(voss): is this the right thing to do?
		// TODO(voss): what happens for invalid password characters?
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

	// cipherAES indicates that AES encryption in CBC mode is used.  This
	// corresponds to the StdCF crypt filter with a CFM value of AESV2 in the
	// PDF specification.
	cipherAES

	// cipherRC4 indicates that RC4 encryption is used.  This corresponds to
	// the StdCF crypt filter with a CFM value of V2 in the PDF specification.
	cipherRC4
)

func (c cipherType) String() string {
	switch c {
	case cipherUnknown:
		return "unknown"
	case cipherAES:
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
		return nil, errors.New("missing StdCF entry in CF dict")
	}

	res := &cryptFilter{}
	res.Length = 0 // TODO(voss): is there a default?
	if obj, ok := cfDict["Length"].(Integer); ok {
		res.Length = int(obj) * 8
	}
	if res.Length < 40 || res.Length > 128 || res.Length%8 != 0 {
		return nil, errors.New("invalid key length")
	}

	switch {
	case cfDict["CFM"] == Name("V2"):
		res.Cipher = cipherRC4
		return res, nil
	case cfDict["CFM"] == Name("AESV2"):
		res.Cipher = cipherAES
		return res, nil
	default:
		return nil, errors.New("unknown cipher")
	}
}

type encryptReader struct {
	cbc      cipher.BlockMode
	r        io.Reader
	buf      []byte
	ready    []byte
	reserved []byte
}

func (r *encryptReader) Read(p []byte) (int, error) {
	if len(r.ready) == 0 {
		k := copy(r.buf, r.reserved)
		for k <= 16 && r.r != nil {
			n, err := r.r.Read(r.buf[k:])
			k += n
			if err == io.EOF {
				r.r = nil
				// add the padding
				kPad := 16 - k%16
				full := k + kPad
				for k < full {
					r.buf[k] = byte(kPad)
					k++
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
	}

	n := copy(p, r.ready)
	r.ready = r.ready[n:]
	return n, nil
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
