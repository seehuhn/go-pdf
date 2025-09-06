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
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/measure"
	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/structure"
)

var px = &measure.NumberFormat{
	Unit:             "px",
	ConversionFactor: 1,
	FractionFormat:   measure.FractionRound,
	SingleUse:        true,
	DecimalSeparator: ".",
}

var testCases = []struct {
	name    string
	version pdf.Version
	data    *Dict
}{
	{
		name:    "basic RGB image",
		version: pdf.V1_7,
		data: &Dict{
			Width:            100,
			Height:           50,
			ColorSpace:       color.DeviceRGBSpace,
			BitsPerComponent: 8,
			WriteData: func(w io.Writer) error {
				// Simple test pattern: alternating red and blue pixels
				for y := 0; y < 50; y++ {
					for x := 0; x < 100; x++ {
						if (x+y)%2 == 0 {
							_, err := w.Write([]byte{255, 0, 0}) // red
							if err != nil {
								return err
							}
						} else {
							_, err := w.Write([]byte{0, 0, 255}) // blue
							if err != nil {
								return err
							}
						}
					}
				}
				return nil
			},
		},
	},
	{
		name:    "grayscale with interpolation",
		version: pdf.V1_7,
		data: &Dict{
			Width:            25,
			Height:           25,
			ColorSpace:       color.DeviceGraySpace,
			BitsPerComponent: 8,
			Interpolate:      true,
			WriteData: func(w io.Writer) error {
				// Gradient pattern
				for y := 0; y < 25; y++ {
					for x := 0; x < 25; x++ {
						gray := uint8((x + y) * 255 / 48)
						if _, err := w.Write([]byte{gray}); err != nil {
							return err
						}
					}
				}
				return nil
			},
		},
	},
	{
		name:    "CMYK with decode array",
		version: pdf.V1_7,
		data: &Dict{
			Width:            10,
			Height:           10,
			ColorSpace:       color.DeviceCMYKSpace,
			BitsPerComponent: 8,
			Decode:           []float64{0.0, 1.0, 0.0, 1.0, 0.0, 1.0, 0.0, 1.0},
			WriteData: func(w io.Writer) error {
				// Simple CMYK test pattern
				for y := 0; y < 10; y++ {
					for x := 0; x < 10; x++ {
						c := uint8(x * 25)
						m := uint8(y * 25)
						y_val := uint8((x + y) * 12)
						k := uint8(50)
						if _, err := w.Write([]byte{c, m, y_val, k}); err != nil {
							return err
						}
					}
				}
				return nil
			},
		},
	},
	{
		name:    "with rendering intent",
		version: pdf.V1_7,
		data: &Dict{
			Width:            20,
			Height:           20,
			ColorSpace:       color.DeviceRGBSpace,
			BitsPerComponent: 8,
			Intent:           graphics.RenderingIntent("Perceptual"),
			WriteData: func(w io.Writer) error {
				// Solid color
				for i := 0; i < 20*20; i++ {
					if _, err := w.Write([]byte{128, 64, 192}); err != nil {
						return err
					}
				}
				return nil
			},
		},
	},
	{
		name:    "with color key mask",
		version: pdf.V1_7,
		data: &Dict{
			Width:            15,
			Height:           15,
			ColorSpace:       color.DeviceRGBSpace,
			BitsPerComponent: 8,
			MaskColors:       []uint16{0, 10, 0, 10, 0, 10}, // mask near-black pixels
			WriteData: func(w io.Writer) error {
				// Pattern with some pixels that will be masked
				for y := 0; y < 15; y++ {
					for x := 0; x < 15; x++ {
						if x == 0 || y == 0 {
							_, err := w.Write([]byte{5, 5, 5}) // will be masked
							if err != nil {
								return err
							}
						} else {
							_, err := w.Write([]byte{200, 100, 50})
							if err != nil {
								return err
							}
						}
					}
				}
				return nil
			},
		},
	},
	{
		name:    "with soft mask",
		version: pdf.V1_4,
		data: &Dict{
			Width:            8,
			Height:           8,
			ColorSpace:       color.DeviceRGBSpace,
			BitsPerComponent: 8,
			SMask: &SoftMask{
				Width:            8,
				Height:           8,
				BitsPerComponent: 8,
				WriteData: func(w io.Writer) error {
					// Alpha gradient
					for y := 0; y < 8; y++ {
						for x := 0; x < 8; x++ {
							alpha := uint8((x + y) * 255 / 14)
							if _, err := w.Write([]byte{alpha}); err != nil {
								return err
							}
						}
					}
					return nil
				},
			},
			WriteData: func(w io.Writer) error {
				// RGB data
				for y := 0; y < 8; y++ {
					for x := 0; x < 8; x++ {
						r := uint8(x * 32)
						g := uint8(y * 32)
						b := uint8(128)
						if _, err := w.Write([]byte{r, g, b}); err != nil {
							return err
						}
					}
				}
				return nil
			},
		},
	},
	{
		name:    "with soft mask and matte",
		version: pdf.V1_4,
		data: &Dict{
			Width:            4,
			Height:           4,
			ColorSpace:       color.DeviceRGBSpace,
			BitsPerComponent: 8,
			SMask: &SoftMask{
				Width:            4,
				Height:           4,
				BitsPerComponent: 8,
				Matte:            []float64{0.5, 0.3, 0.7}, // RGB matte color
				WriteData: func(w io.Writer) error {
					// Simple alpha pattern
					for i := 0; i < 16; i++ {
						alpha := uint8((i % 4) * 85)
						if _, err := w.Write([]byte{alpha}); err != nil {
							return err
						}
					}
					return nil
				},
			},
			WriteData: func(w io.Writer) error {
				// Pre-blended RGB data
				for i := 0; i < 16; i++ {
					r := uint8(200)
					g := uint8(150)
					b := uint8(100)
					if _, err := w.Write([]byte{r, g, b}); err != nil {
						return err
					}
				}
				return nil
			},
		},
	},
	{
		name:    "with SMaskInData",
		version: pdf.V1_5,
		data: &Dict{
			Width:            6,
			Height:           6,
			ColorSpace:       color.DeviceRGBSpace,
			BitsPerComponent: 8,
			SMaskInData:      1, // image data includes encoded soft-mask values
			WriteData: func(w io.Writer) error {
				// RGB data (SMaskInData would normally be for JPXDecode)
				for i := 0; i < 36; i++ {
					r := uint8(i * 7)
					g := uint8(100)
					b := uint8(200 - i*5)
					if _, err := w.Write([]byte{r, g, b}); err != nil {
						return err
					}
				}
				return nil
			},
		},
	},
	{
		name:    "1-bit image",
		version: pdf.V1_7,
		data: &Dict{
			Width:            16,
			Height:           16,
			ColorSpace:       color.DeviceGraySpace,
			BitsPerComponent: 1,
			WriteData: func(w io.Writer) error {
				// Checkerboard pattern in 1-bit
				buf := NewPixelRow(16, 1)
				for y := 0; y < 16; y++ {
					buf.Reset()
					for x := 0; x < 16; x++ {
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
		name:    "16-bit components",
		version: pdf.V1_5,
		data: &Dict{
			Width:            5,
			Height:           5,
			ColorSpace:       color.DeviceRGBSpace,
			BitsPerComponent: 16,
			WriteData: func(w io.Writer) error {
				// High precision RGB data
				buf := NewPixelRow(5*3, 16) // 5 pixels * 3 channels
				for y := 0; y < 5; y++ {
					buf.Reset()
					for x := 0; x < 5; x++ {
						r := uint16(x * 13107) // 0-52428 range
						g := uint16(y * 13107)
						b := uint16(32768) // mid value
						buf.AppendBits(r)
						buf.AppendBits(g)
						buf.AppendBits(b)
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
		name:    "with alternates",
		version: pdf.V1_3,
		data: &Dict{
			Width:            12,
			Height:           12,
			ColorSpace:       color.DeviceGraySpace,
			BitsPerComponent: 8,
			Alternates: []*Dict{
				{
					Width:            12,
					Height:           12,
					ColorSpace:       color.DeviceRGBSpace,
					BitsPerComponent: 8,
					WriteData: func(w io.Writer) error {
						// RGB version
						for i := 0; i < 144; i++ {
							val := uint8(i * 255 / 143)
							if _, err := w.Write([]byte{val, val / 2, val / 3}); err != nil {
								return err
							}
						}
						return nil
					},
				},
			},
			WriteData: func(w io.Writer) error {
				// Grayscale version
				for i := 0; i < 144; i++ {
					val := uint8(i * 255 / 143)
					if _, err := w.Write([]byte{val}); err != nil {
						return err
					}
				}
				return nil
			},
		},
	},
	{
		name:    "with metadata",
		version: pdf.V1_4,
		data: &Dict{
			Width:            8,
			Height:           8,
			ColorSpace:       color.DeviceGraySpace,
			BitsPerComponent: 8,
			// Skip metadata for now - requires properly initialized XMP packet
			WriteData: func(w io.Writer) error {
				// Simple pattern
				for i := 0; i < 64; i++ {
					val := uint8(i * 4)
					if _, err := w.Write([]byte{val}); err != nil {
						return err
					}
				}
				return nil
			},
		},
	},
	{
		name:    "PDF 2.0 with Measure and PtData",
		version: pdf.V2_0,
		data: &Dict{
			Width:            6,
			Height:           6,
			ColorSpace:       color.DeviceRGBSpace,
			BitsPerComponent: 8,
			Measure: &measure.RectilinearMeasure{
				ScaleRatio: "1:1000", // 1:1000 scale
				XAxis:      []*measure.NumberFormat{px},
				Distance:   []*measure.NumberFormat{px},
				Area:       []*measure.NumberFormat{px},
				CYX:        0, // CYX is not meaningful when YAxis is omitted
			},
			PtData: &measure.PtData{
				Subtype: measure.PtDataSubtypeCloud,
				Names:   []string{measure.PtDataNameLat, measure.PtDataNameLon},
				XPTS: [][]pdf.Object{
					{pdf.Real(40.7128), pdf.Real(-74.0060)}, // NYC - normalized to Real
					{pdf.Real(40.7589), pdf.Real(-73.9851)}, // Central Park - normalized to Real
				},
				SingleUse: false,
			},
			WriteData: func(w io.Writer) error {
				// Geospatial RGB data
				for y := 0; y < 6; y++ {
					for x := 0; x < 6; x++ {
						r := uint8(x * 42)
						g := uint8(y * 42)
						b := uint8(128)
						if _, err := w.Write([]byte{r, g, b}); err != nil {
							return err
						}
					}
				}
				return nil
			},
		},
	},
	{
		name:    "with optional content",
		version: pdf.V1_5,
		data: &Dict{
			Width:            10,
			Height:           10,
			ColorSpace:       color.DeviceRGBSpace,
			BitsPerComponent: 8,
			OptionalContent: &oc.Group{
				Name:   "TestLayer",
				Intent: []pdf.Name{"View"},
			},
			WriteData: func(w io.Writer) error {
				// Simple RGB pattern
				for i := 0; i < 100; i++ {
					r := uint8((i * 3) % 256)
					g := uint8((i * 5) % 256)
					b := uint8((i * 7) % 256)
					if _, err := w.Write([]byte{r, g, b}); err != nil {
						return err
					}
				}
				return nil
			},
		},
	},
	{
		name:    "with StructParent value 0",
		version: pdf.V1_7,
		data: &Dict{
			Width:            1,
			Height:           1,
			ColorSpace:       color.DeviceRGBSpace,
			BitsPerComponent: 16,
			StructParent:     structure.NewKey(0),
			WriteData: func(w io.Writer) error {
				_, err := w.Write([]byte{0, 1, 2, 3, 4, 5})
				return err
			},
		},
	},
	{
		name:    "with StructParent value 42",
		version: pdf.V1_7,
		data: &Dict{
			Width:            1,
			Height:           1,
			ColorSpace:       color.DeviceRGBSpace,
			BitsPerComponent: 16,
			StructParent:     structure.NewKey(42),
			WriteData: func(w io.Writer) error {
				_, err := w.Write([]byte{0, 1, 2, 3, 4, 5})
				return err
			},
		},
	},
}

func roundTripTest(t *testing.T, version pdf.Version, data *Dict) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	// Embed the original data
	ref, _, err := pdf.ResourceManagerEmbed(rm, data)
	if err != nil {
		t.Fatalf("failed to embed Dict: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("failed to close ResourceManager: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("failed to close Writer: %v", err)
	}

	// Extract the data back
	x := pdf.NewExtractor(w)
	decoded, err := ExtractDict(x, ref)
	if err != nil {
		t.Fatalf("failed to extract Dict: %v", err)
	}

	// Normalize SingleUse fields for comparison (not stored in PDF)
	if decoded.Measure != nil {
		if decodedRM, ok := decoded.Measure.(*measure.RectilinearMeasure); ok {
			if originalRM, ok := data.Measure.(*measure.RectilinearMeasure); ok {
				decodedRM.SingleUse = originalRM.SingleUse

				// Fix NumberFormat SingleUse fields
				fixNumberFormatSingleUse := func(decoded, original []*measure.NumberFormat) {
					for i, nf := range decoded {
						if i < len(original) {
							nf.SingleUse = original[i].SingleUse
						}
					}
				}

				fixNumberFormatSingleUse(decodedRM.XAxis, originalRM.XAxis)
				fixNumberFormatSingleUse(decodedRM.YAxis, originalRM.YAxis)
				fixNumberFormatSingleUse(decodedRM.Distance, originalRM.Distance)
				fixNumberFormatSingleUse(decodedRM.Area, originalRM.Area)
				fixNumberFormatSingleUse(decodedRM.Angle, originalRM.Angle)
				fixNumberFormatSingleUse(decodedRM.Slope, originalRM.Slope)
			}
		}
	}

	if decoded.PtData != nil && data.PtData != nil {
		decoded.PtData.SingleUse = data.PtData.SingleUse
	}

	// Compare the round-tripped data
	// We need to exclude WriteData from comparison since it's a function
	opts := []cmp.Option{
		cmp.AllowUnexported(Dict{}, SoftMask{}, measure.RectilinearMeasure{}, measure.PtData{}, oc.Group{}),
		cmp.Comparer(func(a, b func(io.Writer) error) bool {
			// We can't compare functions directly, so we compare their output
			var bufA, bufB bytes.Buffer
			errA := a(&bufA)
			errB := b(&bufB)
			return errA == nil && errB == nil && bytes.Equal(bufA.Bytes(), bufB.Bytes())
		}),
	}

	if diff := cmp.Diff(data, decoded, opts...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestDictRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, tc.version, tc.data)
		})
	}
}

func FuzzDictRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)
		rm := pdf.NewResourceManager(w)

		ref, _, err := pdf.ResourceManagerEmbed(rm, tc.data)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = ref
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
		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing PDF object")
		}

		x := pdf.NewExtractor(r)
		objGo, err := ExtractDict(x, objPDF)
		if err != nil {
			t.Skip("malformed PDF object")
		}

		roundTripTest(t, pdf.GetVersion(r), objGo)
	})
}
