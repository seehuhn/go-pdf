// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package image

import (
	"bytes"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var softMaskTests = []struct {
	name     string
	softMask *SoftMask
}{
	{
		name: "basic 8-bit grayscale",
		softMask: &SoftMask{
			Width:            100,
			Height:           50,
			BitsPerComponent: 8,
			WriteData: func(w io.Writer) error {
				// generate test pattern
				buf := make([]byte, 100*50)
				for i := range buf {
					buf[i] = byte(i % 256)
				}
				_, err := w.Write(buf)
				return err
			},
		},
	},
	{
		name: "1-bit mask",
		softMask: &SoftMask{
			Width:            32,
			Height:           16,
			BitsPerComponent: 1,
			WriteData: func(w io.Writer) error {
				// generate 1-bit test pattern (checkerboard)
				buf := NewPixelRow(32, 1)
				for y := 0; y < 16; y++ {
					buf.Reset()
					for x := 0; x < 32; x++ {
						bit := uint16((x + y) % 2)
						buf.AppendBits(bit)
					}
					if _, err := w.Write(buf.Bytes()); err != nil {
						return err
					}
				}
				return nil
			},
		},
	},
	{
		name: "with decode array",
		softMask: &SoftMask{
			Width:            64,
			Height:           32,
			BitsPerComponent: 4,
			Decode:           []float64{1.0, 0.0}, // inverted
			WriteData: func(w io.Writer) error {
				// generate 4-bit test pattern
				buf := NewPixelRow(64, 4)
				for y := 0; y < 32; y++ {
					buf.Reset()
					for x := 0; x < 64; x++ {
						val := uint16((x + y) % 16)
						buf.AppendBits(val)
					}
					if _, err := w.Write(buf.Bytes()); err != nil {
						return err
					}
				}
				return nil
			},
		},
	},
	{
		name: "with interpolation",
		softMask: &SoftMask{
			Width:            25,
			Height:           25,
			BitsPerComponent: 8,
			Interpolate:      true,
			WriteData: func(w io.Writer) error {
				// radial gradient
				buf := make([]byte, 25*25)
				center := 12.0
				for y := 0; y < 25; y++ {
					for x := 0; x < 25; x++ {
						dx := float64(x) - center
						dy := float64(y) - center
						dist := dx*dx + dy*dy
						val := 255 - min(255, int(dist*2))
						buf[y*25+x] = byte(val)
					}
				}
				_, err := w.Write(buf)
				return err
			},
		},
	},
	{
		name: "with matte color",
		softMask: &SoftMask{
			Width:            40,
			Height:           30,
			BitsPerComponent: 8,
			Matte:            []float64{0.5, 0.3, 0.8}, // RGB matte color
			WriteData: func(w io.Writer) error {
				// diagonal gradient
				buf := make([]byte, 40*30)
				for y := 0; y < 30; y++ {
					for x := 0; x < 40; x++ {
						val := (x + y) * 255 / (40 + 30 - 2)
						buf[y*40+x] = byte(val)
					}
				}
				_, err := w.Write(buf)
				return err
			},
		},
	},
	{
		name: "2-bit depth",
		softMask: &SoftMask{
			Width:            16,
			Height:           8,
			BitsPerComponent: 2,
			WriteData: func(w io.Writer) error {
				// generate 2-bit test pattern
				buf := NewPixelRow(16, 2)
				for y := 0; y < 8; y++ {
					buf.Reset()
					for x := 0; x < 16; x++ {
						val := uint16((x + y) % 4)
						buf.AppendBits(val)
					}
					if _, err := w.Write(buf.Bytes()); err != nil {
						return err
					}
				}
				return nil
			},
		},
	},
	{
		name: "16-bit depth",
		softMask: &SoftMask{
			Width:            10,
			Height:           5,
			BitsPerComponent: 16,
			WriteData: func(w io.Writer) error {
				// write 16-bit samples
				buf := make([]byte, 10*5*2) // 2 bytes per sample
				for i := 0; i < len(buf); i += 2 {
					val := uint16(i * 256)  // increasing 16-bit values
					buf[i] = byte(val >> 8) // high byte
					buf[i+1] = byte(val)    // low byte
				}
				_, err := w.Write(buf)
				return err
			},
		},
	},
}

func TestSoftMaskRoundTrip(t *testing.T) {
	versions := []pdf.Version{pdf.V1_7, pdf.V2_0}

	for _, version := range versions {
		for _, tt := range softMaskTests {
			testName := tt.name + "_" + version.String()
			t.Run(testName, func(t *testing.T) {
				w, _ := memfile.NewPDFWriter(version, nil)
				defer w.Close()

				rm := pdf.NewResourceManager(w)

				ref, _, err := tt.softMask.Embed(rm)
				if err != nil {
					t.Fatalf("embed failed: %v", err)
				}

				x := pdf.NewExtractor(w)
				extracted, err := ExtractSoftMask(x, ref)
				if err != nil {
					t.Fatalf("extract failed: %v", err)
				}

				// compare fields (excluding WriteData function)
				if extracted.Width != tt.softMask.Width {
					t.Errorf("width mismatch: got %d, want %d", extracted.Width, tt.softMask.Width)
				}
				if extracted.Height != tt.softMask.Height {
					t.Errorf("height mismatch: got %d, want %d", extracted.Height, tt.softMask.Height)
				}
				if extracted.BitsPerComponent != tt.softMask.BitsPerComponent {
					t.Errorf("bits per component mismatch: got %d, want %d", extracted.BitsPerComponent, tt.softMask.BitsPerComponent)
				}
				if extracted.Interpolate != tt.softMask.Interpolate {
					t.Errorf("interpolate mismatch: got %t, want %t", extracted.Interpolate, tt.softMask.Interpolate)
				}

				if diff := cmp.Diff(extracted.Decode, tt.softMask.Decode); diff != "" {
					t.Errorf("decode array mismatch (-got +want):\n%s", diff)
				}
				if diff := cmp.Diff(extracted.Matte, tt.softMask.Matte); diff != "" {
					t.Errorf("matte array mismatch (-got +want):\n%s", diff)
				}

				// compare actual data by extracting both
				var originalData, extractedData bytes.Buffer
				if err := tt.softMask.WriteData(&originalData); err != nil {
					t.Fatalf("failed to write original data: %v", err)
				}
				if err := extracted.WriteData(&extractedData); err != nil {
					t.Fatalf("failed to write extracted data: %v", err)
				}

				if !bytes.Equal(originalData.Bytes(), extractedData.Bytes()) {
					t.Error("data content mismatch after round trip")
				}
			})
		}
	}
}

func FuzzSoftMaskRoundTrip(f *testing.F) {
	// seed the fuzzer with valid test cases from softMaskTests
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tt := range softMaskTests {
		w, buf := memfile.NewPDFWriter(pdf.V2_0, opt)
		rm := pdf.NewResourceManager(w)

		ref, _, err := tt.softMask.Embed(rm)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}
		w.GetMeta().Trailer["Quir:SM"] = ref
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// make sure we don't panic on random input
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		obj := r.GetMeta().Trailer["Quir:SM"]
		if obj == nil {
			t.Skip("missing soft mask")
		}

		ref, ok := obj.(pdf.Reference)
		if !ok {
			t.Skip("invalid soft mask reference")
		}

		// attempt to extract the soft mask - main goal is no panic
		x := pdf.NewExtractor(r)
		_, err = ExtractSoftMask(x, ref)
		// errors are acceptable for malformed input, panics are not
		_ = err
	})
}
