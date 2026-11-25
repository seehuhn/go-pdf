// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dct

import (
	"io"
	"testing"
)

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) { panic("boom") }

// TestDecodePanicIsError checks that a panic inside the decoder goroutine is
// returned as a read error instead of terminating the process.
func TestDecodePanicIsError(t *testing.T) {
	rc, err := Decode(panicReader{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if _, err := io.ReadAll(rc); err == nil {
		t.Error("expected an error from a panicking source")
	}
}
