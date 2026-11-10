// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestOperatorsEmbedNameVersion checks that embedding a content stream
// uses the output file's name syntax: names which need #xx escapes cannot
// be written before PDF 1.2.
func TestOperatorsEmbedNameVersion(t *testing.T) {
	ops := &Operators{Ops: []Operator{
		{Name: OpTextSetFont, Args: []pdf.Object{pdf.Name("F 1"), pdf.Integer(12)}},
	}}
	for _, v := range []pdf.Version{pdf.V1_1, pdf.V1_2} {
		w, _ := memfile.NewPDFWriter(t, v, nil)
		rm := pdf.NewResourceManager(w)
		_, err := rm.Embed(ops)
		if hasErr := err != nil; hasErr != (v < pdf.V1_2) {
			t.Errorf("PDF %s: unexpected result, err = %v", v, err)
		}
	}
}

// TestWriteStream checks that WriteStream formats operators for the given
// output options and copies other streams unchanged.
func TestWriteStream(t *testing.T) {
	ops := &Operators{Ops: []Operator{
		{Name: OpXObject, Args: []pdf.Object{pdf.Name("X 1")}},
	}}
	w11, _ := memfile.NewPDFWriter(t, pdf.V1_1, nil)
	w12, _ := memfile.NewPDFWriter(t, pdf.V1_2, nil)

	buf := &bytes.Buffer{}
	if err := WriteStream(buf, ops, w12.GetOptions()); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "/X#201 Do\n"; got != want {
		t.Errorf("operators: got %q, want %q", got, want)
	}

	if err := WriteStream(&bytes.Buffer{}, ops, w11.GetOptions()); err == nil {
		t.Error("operators: missing error for PDF 1.1")
	}

	raw := []byte("/X#201 Do % comment\n")
	buf.Reset()
	if err := WriteStream(buf, NewScanner(bytesOpener(raw)), w11.GetOptions()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), raw) {
		t.Errorf("scanned stream: got %q, want %q", buf.Bytes(), raw)
	}
}
