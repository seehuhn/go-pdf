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

package thumbnail

import (
	"bytes"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var thumbnailTestCases = []struct {
	name      string
	version   pdf.Version
	thumbnail *Thumbnail
}{
	{
		name:    "grayscale_8bit",
		version: pdf.V1_7,
		thumbnail: &Thumbnail{
			Width:            76,
			Height:           99,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 8,
			WriteData: func(w io.Writer) error {
				data := make([]byte, 76*99)
				for i := range data {
					data[i] = byte(i % 256)
				}
				_, err := w.Write(data)
				return err
			},
		},
	},
	{
		name:    "rgb_8bit",
		version: pdf.V1_7,
		thumbnail: &Thumbnail{
			Width:            100,
			Height:           100,
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 8,
			WriteData: func(w io.Writer) error {
				data := make([]byte, 100*100*3)
				for i := range data {
					data[i] = byte(i % 256)
				}
				_, err := w.Write(data)
				return err
			},
		},
	},
	{
		name:    "grayscale_with_decode",
		version: pdf.V1_7,
		thumbnail: &Thumbnail{
			Width:            50,
			Height:           50,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 8,
			Decode:           []float64{1.0, 0.0}, // inverted
			WriteData: func(w io.Writer) error {
				data := make([]byte, 50*50)
				for i := range data {
					data[i] = byte(i % 256)
				}
				_, err := w.Write(data)
				return err
			},
		},
	},
	{
		name:    "rgb_with_decode",
		version: pdf.V2_0,
		thumbnail: &Thumbnail{
			Width:            32,
			Height:           32,
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 8,
			Decode:           []float64{0.0, 1.0, 0.5, 1.0, 0.0, 0.5},
			WriteData: func(w io.Writer) error {
				data := make([]byte, 32*32*3)
				for i := range data {
					data[i] = byte(i % 256)
				}
				_, err := w.Write(data)
				return err
			},
		},
	},
	{
		name:    "indexed_grayscale",
		version: pdf.V1_7,
		thumbnail: &Thumbnail{
			Width:            64,
			Height:           64,
			ColorSpace:       makeIndexedGrayscale(),
			BitsPerComponent: 8,
			WriteData: func(w io.Writer) error {
				data := make([]byte, 64*64)
				for i := range data {
					data[i] = byte(i % 16) // use palette indices 0-15
				}
				_, err := w.Write(data)
				return err
			},
		},
	},
	{
		name:    "indexed_rgb",
		version: pdf.V1_7,
		thumbnail: &Thumbnail{
			Width:            48,
			Height:           48,
			ColorSpace:       makeIndexedRGB(),
			BitsPerComponent: 8,
			WriteData: func(w io.Writer) error {
				data := make([]byte, 48*48)
				for i := range data {
					data[i] = byte(i % 8) // use palette indices 0-7
				}
				_, err := w.Write(data)
				return err
			},
		},
	},
	{
		name:    "grayscale_1bit",
		version: pdf.V1_7,
		thumbnail: &Thumbnail{
			Width:            80,
			Height:           80,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 1,
			WriteData: func(w io.Writer) error {
				// 80 pixels = 10 bytes per row
				data := make([]byte, 10*80)
				for i := range data {
					data[i] = byte(0xAA) // alternating pattern
				}
				_, err := w.Write(data)
				return err
			},
		},
	},
	{
		name:    "grayscale_4bit",
		version: pdf.V1_7,
		thumbnail: &Thumbnail{
			Width:            60,
			Height:           60,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 4,
			WriteData: func(w io.Writer) error {
				// 60 pixels = 30 bytes per row (2 pixels per byte)
				data := make([]byte, 30*60)
				for i := range data {
					data[i] = byte((i%16)<<4 | ((i + 1) % 16))
				}
				_, err := w.Write(data)
				return err
			},
		},
	},
}

func makeIndexedGrayscale() *color.SpaceIndexed {
	// create a 16-color grayscale palette
	table := make([]byte, 16)
	for i := range table {
		table[i] = byte(i * 17) // 0, 17, 34, ..., 255
	}
	grayColors := make([]color.Color, 16)
	for i := range grayColors {
		grayColors[i] = color.DeviceGray(float64(table[i]) / 255.0)
	}
	idx, err := color.Indexed(grayColors)
	if err != nil {
		panic(err)
	}
	return idx
}

func makeIndexedRGB() *color.SpaceIndexed {
	// create an 8-color RGB palette
	table := make([]byte, 8*3)
	colors := [][3]byte{
		{0, 0, 0},       // black
		{255, 0, 0},     // red
		{0, 255, 0},     // green
		{0, 0, 255},     // blue
		{255, 255, 0},   // yellow
		{255, 0, 255},   // magenta
		{0, 255, 255},   // cyan
		{255, 255, 255}, // white
	}
	for i, c := range colors {
		table[i*3+0] = c[0]
		table[i*3+1] = c[1]
		table[i*3+2] = c[2]
	}
	rgbColors := make([]color.Color, 8)
	for i, c := range colors {
		rgbColors[i] = color.DeviceRGB(float64(c[0])/255.0, float64(c[1])/255.0, float64(c[2])/255.0)
	}
	idx, err := color.Indexed(rgbColors)
	if err != nil {
		panic(err)
	}
	return idx
}

func roundTripThumbnail(t *testing.T, version pdf.Version, thumb *Thumbnail) {
	t.Helper()

	// capture original data
	var origData bytes.Buffer
	err := thumb.WriteData(&origData)
	if err != nil {
		t.Fatalf("failed to write original data: %v", err)
	}

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	// embed the thumbnail
	ref, _, err := pdf.ResourceManagerEmbed(rm, thumb)
	if err != nil {
		t.Fatalf("failed to embed thumbnail: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("failed to close resource manager: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	// extract back using the writer as getter
	x := pdf.NewExtractor(w)
	decoded, err := ExtractThumbnail(x, ref)
	if err != nil {
		t.Fatalf("failed to extract thumbnail: %v", err)
	}

	// compare fields (except WriteData)
	if decoded.Width != thumb.Width {
		t.Errorf("width mismatch: got %d, want %d", decoded.Width, thumb.Width)
	}
	if decoded.Height != thumb.Height {
		t.Errorf("height mismatch: got %d, want %d", decoded.Height, thumb.Height)
	}
	if decoded.BitsPerComponent != thumb.BitsPerComponent {
		t.Errorf("BitsPerComponent mismatch: got %d, want %d", decoded.BitsPerComponent, thumb.BitsPerComponent)
	}

	// compare decode arrays
	if diff := cmp.Diff(decoded.Decode, thumb.Decode); diff != "" {
		t.Errorf("decode array mismatch (-got +want):\n%s", diff)
	}

	// compare data
	var decodedData bytes.Buffer
	err = decoded.WriteData(&decodedData)
	if err != nil {
		t.Fatalf("failed to write decoded data: %v", err)
	}

	if !bytes.Equal(decodedData.Bytes(), origData.Bytes()) {
		t.Errorf("data mismatch: got %d bytes, want %d bytes", decodedData.Len(), origData.Len())
	}
}

func TestThumbnailRoundTrip(t *testing.T) {
	for _, tc := range thumbnailTestCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripThumbnail(t, tc.version, tc.thumbnail)
		})
	}
}

func TestInvalidThumbnails(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	tests := []struct {
		name      string
		thumbnail *Thumbnail
		wantErr   bool
	}{
		{
			name: "zero_width",
			thumbnail: &Thumbnail{
				Width:            0,
				Height:           100,
				ColorSpace:       color.SpaceDeviceGray,
				BitsPerComponent: 8,
				WriteData:        func(w io.Writer) error { return nil },
			},
			wantErr: true,
		},
		{
			name: "negative_height",
			thumbnail: &Thumbnail{
				Width:            100,
				Height:           -1,
				ColorSpace:       color.SpaceDeviceGray,
				BitsPerComponent: 8,
				WriteData:        func(w io.Writer) error { return nil },
			},
			wantErr: true,
		},
		{
			name: "invalid_bpc",
			thumbnail: &Thumbnail{
				Width:            100,
				Height:           100,
				ColorSpace:       color.SpaceDeviceGray,
				BitsPerComponent: 3,
				WriteData:        func(w io.Writer) error { return nil },
			},
			wantErr: true,
		},
		{
			name: "missing_color_space",
			thumbnail: &Thumbnail{
				Width:            100,
				Height:           100,
				ColorSpace:       nil,
				BitsPerComponent: 8,
				WriteData:        func(w io.Writer) error { return nil },
			},
			wantErr: true,
		},
		{
			name: "invalid_color_space",
			thumbnail: &Thumbnail{
				Width:            100,
				Height:           100,
				ColorSpace:       color.SpaceDeviceCMYK,
				BitsPerComponent: 8,
				WriteData:        func(w io.Writer) error { return nil },
			},
			wantErr: true,
		},
		{
			name: "wrong_decode_length",
			thumbnail: &Thumbnail{
				Width:            100,
				Height:           100,
				ColorSpace:       color.SpaceDeviceGray,
				BitsPerComponent: 8,
				Decode:           []float64{0.0, 1.0, 0.0, 1.0}, // wrong: should be 2 values for grayscale
				WriteData:        func(w io.Writer) error { return nil },
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tc.thumbnail.Embed(rm)
			if tc.wantErr && err == nil {
				t.Error("expected error but got nil")
			} else if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func FuzzThumbnailRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range thumbnailTestCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)

		rm := pdf.NewResourceManager(w)
		ref, _, err := pdf.ResourceManagerEmbed(rm, tc.thumbnail)
		if err != nil {
			continue
		}
		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["TestThumb"] = ref
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		objPDF := r.GetMeta().Trailer["TestThumb"]
		if objPDF == nil {
			t.Skip("missing PDF object")
		}

		x := pdf.NewExtractor(r)
		thumb, err := ExtractThumbnail(x, objPDF)
		if err != nil {
			t.Skip("malformed thumbnail")
		}

		roundTripThumbnail(t, r.GetMeta().Version, thumb)
	})
}
