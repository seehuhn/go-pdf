// Copyright 2022 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package lzw

import (
	"bytes"
	"fmt"
	"testing"
)

func TestLZWSimple(t *testing.T) {
	// This is example 1 from section 7.4.4.2 of PDF 32000-1:2008
	in := []byte{45, 45, 45, 45, 45, 65, 45, 45, 45, 66}

	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, false)
	if err != nil {
		t.Fatal(err)
	}

	_, err = w.Write(in)
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()

	expected := []byte{0x80, 0x0B, 0x60, 0x50, 0x22, 0x0C, 0x0C, 0x85, 0x01}

	if !bytes.Equal(out, expected) {
		fmt.Printf("% 0X\n", out)
		fmt.Printf("% 0X\n", expected)
		t.Fatal("wrong result")
	}
}
