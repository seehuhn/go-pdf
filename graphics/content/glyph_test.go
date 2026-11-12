// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/pdf"
)

func scanBytes(data string) Stream {
	return NewScanner(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte(data))), nil
	})
}

func TestParseGlyphMetrics(t *testing.T) {
	cases := []struct {
		name string
		op   OpName
		args []pdf.Object
		want GlyphMetrics
		ok   bool
	}{
		{"d0", OpType3ColoredGlyph, []pdf.Object{pdf.Integer(500), pdf.Integer(0)},
			GlyphMetrics{Width: 500}, true},
		{"d0 real", OpType3ColoredGlyph, []pdf.Object{pdf.Real(1.5), pdf.Number(0)},
			GlyphMetrics{Width: 1.5}, true},
		{"d1", OpType3UncoloredGlyph,
			[]pdf.Object{pdf.Integer(500), pdf.Integer(0), pdf.Integer(1), pdf.Integer(2), pdf.Integer(3), pdf.Integer(4)},
			GlyphMetrics{Width: 500, BBox: rect.Rect{LLx: 1, LLy: 2, URx: 3, URy: 4}, Uncolored: true}, true},
		{"d0 short", OpType3ColoredGlyph, []pdf.Object{pdf.Integer(500)}, GlyphMetrics{}, false},
		{"d1 short", OpType3UncoloredGlyph,
			[]pdf.Object{pdf.Integer(500), pdf.Integer(0), pdf.Integer(1)}, GlyphMetrics{}, false},
		{"d0 non-numeric", OpType3ColoredGlyph, []pdf.Object{pdf.Integer(500), pdf.Name("x")},
			GlyphMetrics{}, false},
		{"d0 wy", OpType3ColoredGlyph, []pdf.Object{pdf.Integer(500), pdf.Integer(1)},
			GlyphMetrics{}, false},
		{"other", OpFill, []pdf.Object{pdf.Integer(500), pdf.Integer(0)}, GlyphMetrics{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ParseGlyphMetrics(c.op, c.args)
			if ok != c.ok {
				t.Fatalf("ok = %t, want %t", ok, c.ok)
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("metrics differ (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReadGlyphMetrics(t *testing.T) {
	cases := []struct {
		name string
		data string
		want GlyphMetrics
		ok   bool
	}{
		{"d0", "500 0 d0 0 0 m", GlyphMetrics{Width: 500}, true},
		{"comment first", "% hello\n500 0 d0", GlyphMetrics{Width: 500}, true},
		{"empty", "", GlyphMetrics{}, false},
		{"comment only", "% hello\n", GlyphMetrics{}, false},
		{"other first", "0 0 m 500 0 d0", GlyphMetrics{}, false},
		{"wy", "500 1 d0", GlyphMetrics{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ReadGlyphMetrics(scanBytes(c.data))
			if ok != c.ok {
				t.Fatalf("ok = %t, want %t", ok, c.ok)
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("metrics differ (-want +got):\n%s", diff)
			}
		})
	}
	if _, ok := ReadGlyphMetrics(nil); ok {
		t.Error("nil stream is valid")
	}
}

func TestRepairGlyph(t *testing.T) {
	cases := []struct {
		name string
		data string
		want string
	}{
		{"valid", "500 0 d0\n0 0 m\n", "500 0 d0\n0 0 m\n"},
		{"valid d1", "500 0 1 2 3 4 d1\nf\n", "500 0 1 2 3 4 d1\nf\n"},
		{"comment first", "% c\n500 0 d0\n0 0 m\n", "500 0 d0\n% c\n0 0 m\n"},
		{"ops before d0", "0 0 m\n500 0 d0\n10 10 l\n", "500 0 d0\n0 0 m\n10 10 l\n"},
		{"second metrics", "500 0 d0\n0 0 m\n600 0 d0\n1 2 3 4 5 6 d1\n", "500 0 d0\n0 0 m\n"},
		{"second metrics after repair", "0 0 m\n500 0 d0\n600 0 d0\n1 2 3 4 5 6 d1\nf\n",
			"500 0 d0\n0 0 m\nf\n"},
		{"short d1 then d0", "1 0 5 d1\n0 0 m\n700 0 d0\n", "700 0 d0\n0 0 m\n"},
		{"wy", "500 3 d0\n0 0 m\n", "500 0 d0\n0 0 m\n"},
		{"d1 wy", "500 3 1 2 3 4 d1\n", "500 0 1 2 3 4 d1\n"},
		{"extra args", "500 0 7 d0\n", "500 0 d0\n"},
		{"no metrics", "0 0 m\nf\n", "0 0 d0\n0 0 m\nf\n"},
		{"empty", "", "0 0 d0\n"},
		{"comment only", "% x\n", "0 0 d0\n% x\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := RepairGlyph(scanBytes(c.data))
			if again := RepairGlyph(out); again != out {
				t.Error("repaired stream was wrapped again")
			}
			if _, ok := ReadGlyphMetrics(out); !ok {
				t.Fatal("result is not a valid glyph procedure")
			}

			if !StreamsEqual(out, scanBytes(c.want)) {
				t.Errorf("unexpected operators")
			}

			// the byte view agrees with the operator view
			rc, err := out.RawBytes()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(iotest.OneByteReader(rc))
			rc.Close()
			if err != nil {
				t.Fatal(err)
			}
			if !StreamsEqual(out, scanBytes(string(raw))) {
				t.Errorf("RawBytes differs from NewIter: %q", raw)
			}

			// WriteStream gives the same operators
			var buf bytes.Buffer
			if err := WriteStream(&buf, out, 0); err != nil {
				t.Fatal(err)
			}
			if !StreamsEqual(out, scanBytes(buf.String())) {
				t.Errorf("WriteStream differs from NewIter: %q", buf.String())
			}
		})
	}
}

func TestRepairGlyphNil(t *testing.T) {
	out := RepairGlyph(nil)
	m, ok := ReadGlyphMetrics(out)
	if !ok || m.Width != 0 {
		t.Errorf("got %v, %t", m, ok)
	}
}

func TestRepairGlyphInlineImage(t *testing.T) {
	data := "0 0 m\nBI /W 1 /H 1 /BPC 8 /CS /G ID \x80\nEI\nf\n"
	out := RepairGlyph(scanBytes(data))
	var buf bytes.Buffer
	if err := WriteStream(&buf, out, 0); err != nil {
		t.Fatal(err)
	}
	again := scanBytes(buf.String())
	if _, ok := ReadGlyphMetrics(again); !ok {
		t.Fatal("not valid after writing")
	}
	if !StreamsEqual(out, again) {
		t.Error("write/read cycle changed the stream")
	}
}

var errBoom = errors.New("boom")

func TestRepairGlyphError(t *testing.T) {
	in := NewScanner(func() (io.ReadCloser, error) {
		return nil, errBoom
	})
	out := RepairGlyph(in)

	it := out.NewIter()
	for range it.All() {
	}
	if !errors.Is(it.Err(), errBoom) {
		t.Errorf("iterator error = %v", it.Err())
	}

	rc, err := out.RawBytes()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if _, err := io.ReadAll(rc); !errors.Is(err, errBoom) {
		t.Errorf("reader error = %v", err)
	}

	if err := WriteStream(io.Discard, out, 0); !errors.Is(err, errBoom) {
		t.Errorf("WriteStream error = %v", err)
	}
}

func TestRepairGlyphCloseEarly(t *testing.T) {
	out := RepairGlyph(scanBytes("0 0 m\n"))
	rc, err := out.RawBytes()
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 2)
	if _, err := rc.Read(buf); err != nil {
		t.Fatal(err)
	}
	if err := rc.Close(); err != nil {
		t.Fatal(err)
	}
}
