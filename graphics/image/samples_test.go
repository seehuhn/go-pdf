// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf/graphics/color"
)

func collectSamples(t *testing.T, r *SampleReader) []byte {
	t.Helper()
	var res []byte
	next := 0
	for y, row := range r.All() {
		if y != next {
			t.Fatalf("got row %d, want row %d", y, next)
		}
		next++
		if len(row) != r.RowBytes {
			t.Fatalf("row has %d bytes, want %d", len(row), r.RowBytes)
		}
		res = append(res, row...)
	}
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
	if next != r.Height {
		t.Fatalf("got %d rows, want %d", next, r.Height)
	}
	return res
}

func rawSource(width int, cs color.Space, bpc int, data []byte) *FlateSource {
	return NewFlateSource(width, cs, bpc, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

func TestSampleRowsDict(t *testing.T) {
	// 3x2 RGB at 4 bits per component: rows of 5 bytes
	raw := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	d := &Dict{
		Width: 3, Height: 2,
		ColorSpace:       color.SpaceDeviceRGB,
		BitsPerComponent: 4,
		Decode:           []float64{1, 0, 1, 0, 1, 0},
		Data:             rawSource(3, color.SpaceDeviceRGB, 4, raw),
	}
	r, err := d.SampleRows()
	if err != nil {
		t.Fatal(err)
	}
	if r.Width != 3 || r.Height != 2 || r.RowBytes != 5 {
		t.Errorf("got %dx%d with %d bytes per row", r.Width, r.Height, r.RowBytes)
	}
	if diff := cmp.Diff(raw, collectSamples(t, r)); diff != "" {
		t.Errorf("wrong samples (-want +got):\n%s", diff)
	}
}

func TestSampleRowsShortAndLong(t *testing.T) {
	cs := color.SpaceDeviceGray
	for _, tc := range []struct {
		name      string
		raw, want []byte
	}{
		{"short", []byte{1, 2, 3}, []byte{1, 2, 3, 0, 0, 0}},
		{"long", []byte{1, 2, 3, 4, 5, 6, 7, 8}, []byte{1, 2, 3, 4, 5, 6}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &Dict{
				Width: 3, Height: 2,
				ColorSpace: cs, BitsPerComponent: 8,
				Data: rawSource(3, cs, 8, tc.raw),
			}
			r, err := d.SampleRows()
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, collectSamples(t, r)); diff != "" {
				t.Errorf("wrong samples (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSampleRowsMasks(t *testing.T) {
	raw := []byte{0xA0, 0x50}
	m := &Mask{
		Width: 3, Height: 2,
		Source: NewCCITTFaxSource(3, func(w io.Writer) error {
			_, err := w.Write(raw)
			return err
		}),
	}
	r, err := m.SampleRows()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(raw, collectSamples(t, r)); diff != "" {
		t.Errorf("mask: wrong samples (-want +got):\n%s", diff)
	}

	sm := &SoftMask{
		Width: 2, Height: 2, BitsPerComponent: 8,
		Source: rawSource(2, color.SpaceDeviceGray, 8, []byte{1, 2, 3, 4}),
	}
	r, err = sm.SampleRows()
	if err != nil {
		t.Fatal(err)
	}
	if got := collectSamples(t, r); !bytes.Equal(got, []byte{1, 2, 3, 4}) {
		t.Errorf("soft mask: got %v", got)
	}
}

func TestSampleReaderRepeat(t *testing.T) {
	raw := []byte{1, 2, 3, 4}
	d := &Dict{
		Width: 2, Height: 2,
		ColorSpace:       color.SpaceDeviceGray,
		BitsPerComponent: 8,
		Data:             rawSource(2, color.SpaceDeviceGray, 8, raw),
	}
	r, err := d.SampleRows()
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		var got []byte
		for _, row := range r.All() {
			got = append(got, row...)
		}
		if err := r.Err(); err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(raw, got); diff != "" {
			t.Errorf("run %d: wrong samples (-want +got):\n%s", i, diff)
		}
	}
}

func TestSampleReaderEarlyStop(t *testing.T) {
	raw := []byte{1, 2, 3, 4, 5, 6}
	d := &Dict{
		Width: 2, Height: 3,
		ColorSpace:       color.SpaceDeviceGray,
		BitsPerComponent: 8,
		Data:             rawSource(2, color.SpaceDeviceGray, 8, raw),
	}
	r, err := d.SampleRows()
	if err != nil {
		t.Fatal(err)
	}
	for range r.All() {
		break
	}
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(raw, collectSamples(t, r)); diff != "" {
		t.Errorf("wrong samples after early stop (-want +got):\n%s", diff)
	}
}

func TestSampleReaderBadData(t *testing.T) {
	d := &Dict{
		Width: 2, Height: 2,
		ColorSpace:       color.SpaceDeviceGray,
		BitsPerComponent: 8,
		Data: NewFlateSource(2, color.SpaceDeviceGray, 8, func(io.Writer) error {
			return errors.New("write failed")
		}),
	}
	r, err := d.SampleRows()
	if err != nil {
		t.Fatal(err)
	}
	for range r.All() {
	}
	if r.Err() == nil {
		t.Error("missing error")
	}
}

func TestSampleRowsJPX(t *testing.T) {
	m := &Mask{Width: 2, Height: 2, Source: NewJPXSource(nil)}
	if _, err := m.SampleRows(); err == nil {
		t.Error("JPX image accepted")
	}
}
