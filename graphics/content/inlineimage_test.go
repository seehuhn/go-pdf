// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package content

import (
	"bytes"
	"compress/zlib"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestDecodeInlineImageNoFilter(t *testing.T) {
	raw := []byte("hello world")
	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{pdf.Dict{}, pdf.String(raw)},
	}
	got, err := DecodeInlineImage(op)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Errorf("expected %q, got %q", raw, got)
	}
}

func TestDecodeInlineImageFlateDecode(t *testing.T) {
	original := []byte("the quick brown fox jumps over the lazy dog")
	compressed := deflate(t, original)

	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{
			pdf.Dict{"Filter": pdf.Name("FlateDecode")},
			pdf.String(compressed),
		},
	}
	got, err := DecodeInlineImage(op)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("expected %q, got %q", original, got)
	}
}

func TestDecodeInlineImageAbbreviatedFilter(t *testing.T) {
	original := []byte("abbreviated filter test data here")
	compressed := deflate(t, original)

	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{
			pdf.Dict{"F": pdf.Name("Fl")},
			pdf.String(compressed),
		},
	}
	got, err := DecodeInlineImage(op)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("expected %q, got %q", original, got)
	}
}

func TestDecodeInlineImageUnknownFilter(t *testing.T) {
	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{
			pdf.Dict{"Filter": pdf.Name("NoSuchFilter")},
			pdf.String("data"),
		},
	}
	_, err := DecodeInlineImage(op)
	if err == nil {
		t.Fatal("expected error for unknown filter")
	}
}

func TestDecodeInlineImageFilterArray(t *testing.T) {
	original := []byte("chained filter test")
	compressed := deflate(t, original)

	op := Operator{
		Name: OpInlineImage,
		Args: []pdf.Object{
			pdf.Dict{
				"Filter": pdf.Array{pdf.Name("Fl")},
			},
			pdf.String(compressed),
		},
	}
	got, err := DecodeInlineImage(op)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("expected %q, got %q", original, got)
	}
}

func deflate(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
