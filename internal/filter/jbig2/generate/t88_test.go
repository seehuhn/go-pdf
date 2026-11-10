// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// t88File builds a file in the form written by the T.88 sample encoder: a
// file header, page information with OR as default operator, an immediate
// lossless generic region using XOR, and end of page.
func t88File(coded []byte) []byte {
	region := regionInfoData(8, 2, 0, 0, combOpXOR)
	region = append(region, 0x00, 3, 0xFF, 0xFD, 0xFF, 2, 0xFE, 0xFE, 0xFE)
	segs := []*segment{
		{number: 0, typ: segPageInfo, page: 1, data: pageInfoData(8, 2, pageLossless)},
		{number: 1, typ: segLosslessGeneric, page: 1, data: append(region, coded...)},
		{number: 2, typ: segEndOfPage, page: 1},
	}
	file := []byte(t88FileHeader)
	return append(file, encodeSegments(segs)...)
}

func TestT88Embedded(t *testing.T) {
	tests := []struct {
		coded, want []byte
		edits       int
	}{
		{[]byte{0x12, 0x34}, []byte{0x12, 0x34, 0xFF, 0xAC}, 2},
		{[]byte{0x12, 0xFF}, []byte{0x12, 0xFF, 0xAC}, 2},
		{[]byte{0x12, 0xFF, 0xAC}, []byte{0x12, 0xFF, 0xAC}, 1},
	}
	for _, tt := range tests {
		page, edits, err := t88Embedded(t88File(tt.coded))
		if err != nil {
			t.Fatal(err)
		}
		if len(edits) != tt.edits {
			t.Errorf("%X: got %d edits, want %d", tt.coded, len(edits), tt.edits)
		}
		segs, err := parseSegments(page)
		if err != nil {
			t.Fatal(err)
		}
		pi, err := findSegment(segs, segPageInfo)
		if err != nil {
			t.Fatal(err)
		}
		if pi.data[16]&pageCombOpOverride == 0 {
			t.Errorf("%X: override flag not set", tt.coded)
		}
		gr, err := findSegment(segs, segLosslessGeneric)
		if err != nil {
			t.Fatal(err)
		}
		if got := gr.data[26:]; !bytes.Equal(got, tt.want) {
			t.Errorf("%X: got coded data %X, want %X", tt.coded, got, tt.want)
		}
	}
}

func TestT88Binary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "jbig2")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	t.Setenv(t88EnvVar, "")
	if got, err := t88Binary(); err != nil || got != "" {
		t.Errorf("unset variable: got %q, %v", got, err)
	}

	t.Setenv(t88EnvVar, "./jbig2")
	got, err := t88Binary()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) || filepath.Base(got) != "jbig2" {
		t.Errorf("relative path: got %q", got)
	}

	t.Setenv(t88EnvVar, "./missing")
	if _, err := t88Binary(); err == nil {
		t.Error("missing binary accepted")
	}
}
