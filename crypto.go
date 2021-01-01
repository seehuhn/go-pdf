package pdflib

import (
	"bytes"
	"crypto/md5"
	"crypto/rc4"
	"errors"
	"fmt"
)

type encryptInfo struct {
	StmF *cryptFilter
	StrF *cryptFilter
	EFF  *cryptFilter

	sh *standardSecurityHandler
}

func (r *Reader) checkPwd() error {
	if r.encInfo == nil {
		return nil
	}
	if r.encInfo.StmF == nil && r.encInfo.StrF == nil && r.encInfo.EFF == nil {
		return nil
	}

	var passwd string
	for {
		if r.encInfo.sh.Authenticate(passwd) == nil {
			return nil
		}
		if r.readPwd == nil {
			return ErrWrongPassword
		}
		passwd = r.readPwd()
		if passwd == "" {
			return ErrWrongPassword
		}
	}
}

func (r *Reader) checkEncrypt(encObj Object) (*encryptInfo, error) {
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
			Cipher: CipherRC4,
			Length: 40,
		}
		res.StmF = cf
		res.StrF = cf
		res.EFF = cf
	case 2:
		cf := &cryptFilter{
			Cipher: CipherRC4,
			Length: length,
		}
		res.StmF = cf
		res.StrF = cf
		res.EFF = cf
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
			res.StmF = ciph
		}
		if obj, ok := enc["StrF"].(Name); ok {
			ciph, err := getCipher(obj, CF)
			if err != nil {
				return nil, &MalformedFileError{
					Pos: r.errPos(encObj),
					Err: err,
				}
			}
			res.StrF = ciph
		}
		res.EFF = res.StmF
		if obj, ok := enc["EFF"].(Name); ok {
			ciph, err := getCipher(obj, CF)
			if err != nil {
				return nil, &MalformedFileError{
					Pos: r.errPos(encObj),
					Err: err,
				}
			}
			res.EFF = ciph
		}
	default:
		return nil, &MalformedFileError{
			Pos: r.errPos(encObj),
			Err: errors.New("unsupported Encrypt.V value"),
		}
	}

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

	res.sh = &standardSecurityHandler{
		ID:  []byte(r.ID[0]),
		N:   length / 8,
		R:   int(R),
		O:   []byte(O),
		U:   []byte(U),
		P:   uint32(P),
		EMD: emd,
	}

	return res, nil
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

	authEvent := Name("DocOpen")
	if obj, ok := cfDict["AuthEvent"].(Name); ok {
		authEvent = obj
	}

	res := &cryptFilter{}
	res.Length = 0 // TODO(voss): is there a default?
	if obj, ok := cfDict["Length"].(Integer); ok {
		res.Length = int(obj) * 8
		if res.Length < 40 || res.Length > 128 || res.Length%8 != 0 {
			return nil, errors.New("invalid key length")
		}
	}

	switch {
	case cfDict["CFM"] == Name("V2") && authEvent == "DocOpen":
		res.Cipher = CipherRC4
		return res, nil
	case cfDict["CFM"] == Name("AESV2") && authEvent == "DocOpen":
		res.Cipher = CipherAES
		return res, nil
	default:
		return nil, errors.New("unknown cipher")
	}
}

type standardSecurityHandler struct {
	ID  []byte
	N   int
	R   int
	O   []byte
	U   []byte
	P   uint32 // permission bits
	EMD bool
}

func (sec *standardSecurityHandler) ComputeKey(passwd string) []byte {
	pw := padPasswd(passwd)

	h := md5.New()
	h.Write(pw)
	h.Write(sec.O)
	h.Write([]byte{byte(sec.P), byte(sec.P >> 8), byte(sec.P >> 16), byte(sec.P >> 24)})
	h.Write(sec.ID)
	key := h.Sum(nil)

	if sec.R >= 3 {
		for i := 0; i < 50; i++ {
			h.Reset()
			h.Write(key[:sec.N])
			key = h.Sum(key[:0])
		}
	}
	return key[:sec.N]
}

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

func (sec *standardSecurityHandler) ComputeO(userPasswd, ownerPasswd string) []byte {
	if ownerPasswd == "" {
		ownerPasswd = userPasswd
	}
	opw := padPasswd(ownerPasswd)

	h := md5.New()
	h.Write(opw)
	sum := h.Sum(nil)
	if sec.R >= 3 {
		for i := 0; i < 50; i++ {
			h.Reset()
			h.Write(sum) // sum[:sec.Length/8]
			sum = h.Sum(sum[:0])
		}
	}
	rc4key := sum[:sec.N]

	c, err := rc4.NewCipher(rc4key)
	if err != nil {
		// Only invalid Length values can cause errors, and we already checked
		// the length, above.
		panic(err)
	}

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

func (sec *standardSecurityHandler) ComputeU(userPasswd string) []byte {
	rc4key := sec.ComputeKey(userPasswd)
	c, err := rc4.NewCipher(rc4key)
	if err != nil {
		// Only invalid Length values can cause errors, and we already checked
		// the length, above.
		panic(err)
	}

	u := make([]byte, 32)
	if sec.R < 3 {
		copy(u, passwdPad)
		c.XORKeyStream(u, u)
	} else {
		h := md5.New()
		h.Write(passwdPad)
		h.Write(sec.ID)
		u = h.Sum(u[:0])
		c.XORKeyStream(u, u)

		key := make([]byte, len(rc4key))
		for i := byte(1); i <= 19; i++ {
			for j := range key {
				key[j] = rc4key[j] ^ byte(i)
			}
			c, _ = rc4.NewCipher(key)
			c.XORKeyStream(u, u)
		}
		// This gives the first 16 bytes of U, the remaining 16 bytes
		// are "arbitrary padding".
		u = u[:32]
	}

	return u
}

func (sec *standardSecurityHandler) Authenticate(passwd string) error {
	U := sec.ComputeU(passwd)
	var ok bool
	if sec.R >= 3 {
		ok = bytes.Equal(sec.U[:16], U[:16])
	} else {
		ok = bytes.Equal(sec.U, U)
	}
	if ok {
		return nil
	}

	return ErrWrongPassword
}

var passwdPad = []byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

type cryptFilter struct {
	Cipher Cipher
	Length int
}

func (cf *cryptFilter) String() string {
	return fmt.Sprintf("%s-%d", cf.Cipher, cf.Length)
}

// Cipher denotes the type of encryption used in (parts of) a PDF file.
type Cipher int

const (
	// CipherUnknown indicates that the encryption scheme has not yet been
	// determined.
	CipherUnknown Cipher = iota

	// CipherAES indicates that AES encryption in CBC mode is used.  This
	// corresponds to the StdCF crypt filter with a CFM value of AESV2 in the
	// PDF specification.
	CipherAES

	// CipherRC4 indicates that RC4 encryption is used.  This corresponds to
	// the StdCF crypt filter with a CFM value of V2 in the PDF specification.
	CipherRC4
)

func (c Cipher) String() string {
	switch c {
	case CipherUnknown:
		return "unknown"
	case CipherAES:
		return "AES"
	case CipherRC4:
		return "RC4"
	default:
		return fmt.Sprintf("cipher#%d", c)
	}
}
