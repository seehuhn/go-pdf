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

package predict

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"seehuhn.de/go/membudget"
)

// sampleData returns rows of test data for the given parameters.  Where a row
// does not fill its last byte, the spare bits are set as well: a predictor
// carries them through untouched, so a round trip must return them unchanged.
func sampleData(p *Params, rows int) []byte {
	rowBytes := p.bytesPerRow()

	data := make([]byte, rowBytes*rows)
	for row := range rows {
		for i := range rowBytes {
			data[row*rowBytes+i] = byte(i*31 + row*17 + i*i)
		}
	}
	return data
}

func encodeDecode(t *testing.T, p *Params, data []byte) []byte {
	t.Helper()

	buf := &writeCloser{Buffer: &bytes.Buffer{}}
	w, err := NewWriter(buf, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(io.NopCloser(bytes.NewReader(buf.Bytes())), p, membudget.New(1<<30))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestRoundTripAllPredictors checks that every predictor recovers the data it
// encoded, at every bit depth and channel count.  The row length is chosen so
// that a row does not fill a whole number of bytes at the narrow depths, which
// covers the spare bits at the end of a row as well as the components.
func TestRoundTripAllPredictors(t *testing.T) {
	for _, predictor := range []int{2, 10, 11, 12, 13, 14, 15} {
		for _, colors := range []int{1, 3, 4} {
			for _, bpc := range []int{1, 2, 4, 8, 16} {
				name := fmt.Sprintf("pred%d/colors%d/bpc%d", predictor, colors, bpc)
				t.Run(name, func(t *testing.T) {
					p := &Params{
						Colors:           colors,
						BitsPerComponent: bpc,
						Columns:          37,
						Predictor:        predictor,
					}
					data := sampleData(p, 11)
					if got := encodeDecode(t, p, data); !bytes.Equal(got, data) {
						t.Error("round trip failed")
					}
				})
			}
		}
	}
}

// TestWriterRowBuffers checks that encoding does not allocate per row: a tall
// image must cost no more allocations than a short one.
func TestWriterRowBuffers(t *testing.T) {
	for _, predictor := range []int{2, 12, 15} {
		t.Run(fmt.Sprintf("pred%d", predictor), func(t *testing.T) {
			p := &Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          64,
				Predictor:        predictor,
			}

			count := func(rows int) float64 {
				data := sampleData(p, rows)
				return testing.AllocsPerRun(3, func() {
					w, err := NewWriter(nopWriteCloser{io.Discard}, p)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := w.Write(data); err != nil {
						t.Fatal(err)
					}
					if err := w.Close(); err != nil {
						t.Fatal(err)
					}
				})
			}

			short, tall := count(4), count(400)
			if tall > short {
				t.Errorf("4 rows allocate %g, 400 rows allocate %g", short, tall)
			}
		})
	}
}

// TestReaderRowBuffers checks that decoding does not allocate per row: a tall
// image must cost no more allocations than a short one.  Decoding runs on data
// read from a file, so its cost must stay tied to the size of that file.
func TestReaderRowBuffers(t *testing.T) {
	for _, predictor := range []int{2, 12, 15} {
		t.Run(fmt.Sprintf("pred%d", predictor), func(t *testing.T) {
			p := &Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          64,
				Predictor:        predictor,
			}

			count := func(rows int) float64 {
				buf := &writeCloser{Buffer: &bytes.Buffer{}}
				w, err := NewWriter(buf, p)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := w.Write(sampleData(p, rows)); err != nil {
					t.Fatal(err)
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				encoded := buf.Bytes()

				return testing.AllocsPerRun(3, func() {
					r, err := NewReader(io.NopCloser(bytes.NewReader(encoded)), p, membudget.New(1<<30))
					if err != nil {
						t.Fatal(err)
					}
					if _, err := io.Copy(io.Discard, r); err != nil {
						t.Fatal(err)
					}
					if err := r.Close(); err != nil {
						t.Fatal(err)
					}
				})
			}

			short, tall := count(4), count(400)
			if tall > short {
				t.Errorf("4 rows allocate %g, 400 rows allocate %g", short, tall)
			}
		})
	}
}

// TestWriterShortFinalRow checks what a caller is promised when the data does
// not end on a row boundary: closing the writer must succeed, and everything
// which was written must come back unchanged.
//
// How the writer completes that last row is its own business, so the test says
// nothing about the bytes past the end of the input.
func TestWriterShortFinalRow(t *testing.T) {
	for _, predictor := range []int{2, 10, 11, 12, 13, 14, 15} {
		t.Run(fmt.Sprintf("pred%d", predictor), func(t *testing.T) {
			p := &Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          4,
				Predictor:        predictor,
			}

			// two whole rows, then five bytes of a third
			data := sampleData(p, 3)[:2*p.bytesPerRow()+5]

			got := encodeDecode(t, p, data)
			if len(got) < len(data) {
				t.Fatalf("got %d bytes back, want at least %d", len(got), len(data))
			}
			if !bytes.Equal(got[:len(data)], data) {
				t.Error("data written before the short row was not recovered")
			}
		})
	}
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }
