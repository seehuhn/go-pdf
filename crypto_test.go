package pdf

import (
	"fmt"
	"os"
	"testing"
)

func TestComputeOU(t *testing.T) {
	passwd := "test"
	P := -4
	sec := &standardSecurityHandler{
		P: uint32(P),
		ID: []byte{0xac, 0xac, 0x29, 0xb4, 0x19, 0x2f, 0xd9, 0x23,
			0xc2, 0x4f, 0xe6, 0x04, 0x24, 0x79, 0xb2, 0xa9},
		R: 4,
		N: 16,
	}

	O := sec.ComputeO(passwd, "")
	goodO := "badad1e86442699427116d3e5d5271bc80a27814fc5e80f815efeef839354c5f"
	if fmt.Sprintf("%x", O) != goodO {
		t.Fatal("wrong O value")
	}
	sec.O = O

	U := sec.ComputeU(passwd)
	goodU := "a5b5fc1fcc399c6845fedcdfac82027c00000000000000000000000000000000"
	if fmt.Sprintf("%x", U) != goodU {
		t.Fatal("wrong U value")
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
