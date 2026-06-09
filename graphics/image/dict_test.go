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
	stdimage "image"
	stdcolor "image/color"
	"image/jpeg"
	"io"
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/streamlimits"
	"seehuhn.de/go/pdf/measure"
	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/optional"
	"seehuhn.de/go/pdf/webcapture"
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
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 8,
			Data: &FlateSource{Predictor: 15, Width: 100, Colors: 3, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				// Simple test pattern: alternating red and blue pixels
				for y := range 50 {
					for x := range 100 {
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
			}},
		},
	},
	{
		name:    "grayscale with interpolation",
		version: pdf.V1_7,
		data: &Dict{
			Width:            25,
			Height:           25,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 8,
			Interpolate:      true,
			Data: &FlateSource{Predictor: 15, Width: 25, Colors: 1, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				// Gradient pattern
				for y := range 25 {
					for x := range 25 {
						gray := uint8((x + y) * 255 / 48)
						if _, err := w.Write([]byte{gray}); err != nil {
							return err
						}
					}
				}
				return nil
			}},
		},
	},
	{
		name:    "CMYK with decode array",
		version: pdf.V1_7,
		data: &Dict{
			Width:            10,
			Height:           10,
			ColorSpace:       color.SpaceDeviceCMYK,
			BitsPerComponent: 8,
			Decode:           []float64{0.0, 1.0, 0.0, 1.0, 0.0, 1.0, 0.0, 1.0},
			Data: &FlateSource{Predictor: 15, Width: 10, Colors: 4, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				// Simple CMYK test pattern
				for y := range 10 {
					for x := range 10 {
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
			}},
		},
	},
	{
		name:    "with rendering intent",
		version: pdf.V1_7,
		data: &Dict{
			Width:            20,
			Height:           20,
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 8,
			Intent:           graphics.RenderingIntent("Perceptual"),
			Data: &FlateSource{Predictor: 15, Width: 20, Colors: 3, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				// Solid color
				for range 20 * 20 {
					if _, err := w.Write([]byte{128, 64, 192}); err != nil {
						return err
					}
				}
				return nil
			}},
		},
	},
	{
		name:    "with color key mask",
		version: pdf.V1_7,
		data: &Dict{
			Width:            15,
			Height:           15,
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 8,
			MaskColors:       []uint16{0, 10, 0, 10, 0, 10}, // mask near-black pixels
			Data: &FlateSource{Predictor: 15, Width: 15, Colors: 3, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				// Pattern with some pixels that will be masked
				for y := range 15 {
					for x := range 15 {
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
			}},
		},
	},
	{
		name:    "with soft mask",
		version: pdf.V1_4,
		data: &Dict{
			Width:            8,
			Height:           8,
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 8,
			SMask: &SoftMask{
				Width:            8,
				Height:           8,
				BitsPerComponent: 8,
				Source: &FlateSource{Predictor: 12, Width: 8, Colors: 1, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
					// Alpha gradient
					for y := range 8 {
						for x := range 8 {
							alpha := uint8((x + y) * 255 / 14)
							if _, err := w.Write([]byte{alpha}); err != nil {
								return err
							}
						}
					}
					return nil
				}},
			},
			Data: &FlateSource{Predictor: 15, Width: 8, Colors: 3, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				// RGB data
				for y := range 8 {
					for x := range 8 {
						r := uint8(x * 32)
						g := uint8(y * 32)
						b := uint8(128)
						if _, err := w.Write([]byte{r, g, b}); err != nil {
							return err
						}
					}
				}
				return nil
			}},
		},
	},
	{
		name:    "with soft mask and matte",
		version: pdf.V1_4,
		data: &Dict{
			Width:            4,
			Height:           4,
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 8,
			SMask: &SoftMask{
				Width:            4,
				Height:           4,
				BitsPerComponent: 8,
				Matte:            []float64{0.5, 0.3, 0.7}, // RGB matte color
				Source: &FlateSource{Predictor: 12, Width: 4, Colors: 1, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
					// Simple alpha pattern
					for i := range 16 {
						alpha := uint8((i % 4) * 85)
						if _, err := w.Write([]byte{alpha}); err != nil {
							return err
						}
					}
					return nil
				}},
			},
			Data: &FlateSource{Predictor: 15, Width: 4, Colors: 3, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				// Pre-blended RGB data
				for range 16 {
					r := uint8(200)
					g := uint8(150)
					b := uint8(100)
					if _, err := w.Write([]byte{r, g, b}); err != nil {
						return err
					}
				}
				return nil
			}},
		},
	},
	{
		name:    "1-bit image",
		version: pdf.V1_7,
		data: &Dict{
			Width:            16,
			Height:           16,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 1,
			Data: &FlateSource{Predictor: 15, Width: 16, Colors: 1, BitsPerComponent: 1, WriteData: func(w io.Writer) error {
				// Checkerboard pattern in 1-bit
				buf := NewPixelRow(16, 1)
				for y := range 16 {
					buf.Reset()
					for x := range 16 {
						bit := uint16((x + y) % 2)
						buf.AppendBits(bit)
					}
					if _, err := w.Write(buf.Bytes()); err != nil {
						return err
					}
				}
				return nil
			}},
		},
	},
	{
		name:    "16-bit components",
		version: pdf.V1_5,
		data: &Dict{
			Width:            5,
			Height:           5,
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 16,
			Data: &FlateSource{Predictor: 15, Width: 5, Colors: 3, BitsPerComponent: 16, WriteData: func(w io.Writer) error {
				// High precision RGB data
				buf := NewPixelRow(5*3, 16) // 5 pixels * 3 channels
				for y := range 5 {
					buf.Reset()
					for x := range 5 {
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
			}},
		},
	},
	{
		name:    "with alternates",
		version: pdf.V1_3,
		data: &Dict{
			Width:            12,
			Height:           12,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 8,
			Alternates: []*Alternate{
				{
					Image: &Dict{
						Width:            12,
						Height:           12,
						ColorSpace:       color.SpaceDeviceRGB,
						BitsPerComponent: 8,
						Data: &FlateSource{Predictor: 15, Width: 12, Colors: 3, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
							// RGB version
							for i := range 144 {
								val := uint8(i * 255 / 143)
								if _, err := w.Write([]byte{val, val / 2, val / 3}); err != nil {
									return err
								}
							}
							return nil
						}},
					},
				},
			},
			Data: &FlateSource{Predictor: 15, Width: 12, Colors: 1, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				// Grayscale version
				for i := range 144 {
					val := uint8(i * 255 / 143)
					if _, err := w.Write([]byte{val}); err != nil {
						return err
					}
				}
				return nil
			}},
		},
	},
	{
		name:    "with alternate DefaultForPrinting",
		version: pdf.V1_3,
		data: &Dict{
			Width:            4,
			Height:           4,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 8,
			Alternates: []*Alternate{
				{
					Image: &Dict{
						Width:            4,
						Height:           4,
						ColorSpace:       color.SpaceDeviceRGB,
						BitsPerComponent: 8,
						Data: &FlateSource{Predictor: 15, Width: 4, Colors: 3, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
							for range 16 {
								if _, err := w.Write([]byte{100, 150, 200}); err != nil {
									return err
								}
							}
							return nil
						}},
					},
					DefaultForPrinting: true,
				},
			},
			Data: &FlateSource{Predictor: 15, Width: 4, Colors: 1, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				for range 16 {
					if _, err := w.Write([]byte{128}); err != nil {
						return err
					}
				}
				return nil
			}},
		},
	},
	{
		name:    "with alternate OC",
		version: pdf.V1_5,
		data: &Dict{
			Width:            4,
			Height:           4,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 8,
			Alternates: []*Alternate{
				{
					Image: &Dict{
						Width:            4,
						Height:           4,
						ColorSpace:       color.SpaceDeviceRGB,
						BitsPerComponent: 8,
						Data: &FlateSource{Predictor: 15, Width: 4, Colors: 3, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
							for range 16 {
								if _, err := w.Write([]byte{50, 100, 150}); err != nil {
									return err
								}
							}
							return nil
						}},
					},
					OC: &oc.Group{
						Name:   "PrintLayer",
						Intent: []pdf.Name{"View"},
					},
				},
			},
			Data: &FlateSource{Predictor: 15, Width: 4, Colors: 1, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				for range 16 {
					if _, err := w.Write([]byte{64}); err != nil {
						return err
					}
				}
				return nil
			}},
		},
	},
	{
		name:    "with metadata",
		version: pdf.V1_4,
		data: &Dict{
			Width:            8,
			Height:           8,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 8,
			// Skip metadata for now - requires properly initialized XMP packet
			Data: &FlateSource{Predictor: 15, Width: 8, Colors: 1, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				// Simple pattern
				for i := range 64 {
					val := uint8(i * 4)
					if _, err := w.Write([]byte{val}); err != nil {
						return err
					}
				}
				return nil
			}},
		},
	},
	{
		name:    "PDF 2.0 with Measure and PtData",
		version: pdf.V2_0,
		data: &Dict{
			Width:            6,
			Height:           6,
			ColorSpace:       color.SpaceDeviceRGB,
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
			Data: &FlateSource{Predictor: 15, Width: 6, Colors: 3, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				// Geospatial RGB data
				for y := range 6 {
					for x := range 6 {
						r := uint8(x * 42)
						g := uint8(y * 42)
						b := uint8(128)
						if _, err := w.Write([]byte{r, g, b}); err != nil {
							return err
						}
					}
				}
				return nil
			}},
		},
	},
	{
		name:    "with optional content",
		version: pdf.V1_5,
		data: &Dict{
			Width:            10,
			Height:           10,
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 8,
			OptionalContent: &oc.Group{
				Name:   "TestLayer",
				Intent: []pdf.Name{"View"},
			},
			Data: &FlateSource{Predictor: 15, Width: 10, Colors: 3, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				// Simple RGB pattern
				for i := range 100 {
					r := uint8((i * 3) % 256)
					g := uint8((i * 5) % 256)
					b := uint8((i * 7) % 256)
					if _, err := w.Write([]byte{r, g, b}); err != nil {
						return err
					}
				}
				return nil
			}},
		},
	},
	{
		name:    "with StructParent value 0",
		version: pdf.V1_7,
		data: &Dict{
			Width:            1,
			Height:           1,
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 16,
			StructParent:     optional.NewUInt(0),
			Data: &FlateSource{Predictor: 15, Width: 1, Colors: 3, BitsPerComponent: 16, WriteData: func(w io.Writer) error {
				_, err := w.Write([]byte{0, 1, 2, 3, 4, 5})
				return err
			}},
		},
	},
	{
		name:    "with StructParent value 42",
		version: pdf.V1_7,
		data: &Dict{
			Width:            1,
			Height:           1,
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 16,
			StructParent:     optional.NewUInt(42),
			Data: &FlateSource{Predictor: 15, Width: 1, Colors: 3, BitsPerComponent: 16, WriteData: func(w io.Writer) error {
				_, err := w.Write([]byte{0, 1, 2, 3, 4, 5})
				return err
			}},
		},
	},
	{
		name:    "with image mask",
		version: pdf.V1_7,
		data: &Dict{
			Width:            4,
			Height:           4,
			ColorSpace:       color.SpaceDeviceRGB,
			BitsPerComponent: 8,
			MaskImage: &Mask{
				Width:  4,
				Height: 4,
				Source: &CCITTFaxSource{Width: 4, K: -1, WriteData: func(w io.Writer) error {
					// 4x4 1-bit mask: checkerboard
					buf := NewPixelRow(4, 1)
					for y := range 4 {
						buf.Reset()
						for x := range 4 {
							buf.AppendBits(uint16((x + y) % 2))
						}
						if _, err := w.Write(buf.Bytes()); err != nil {
							return err
						}
					}
					return nil
				}},
			},
			Data: &FlateSource{Predictor: 15, Width: 4, Colors: 3, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				for range 16 {
					if _, err := w.Write([]byte{100, 150, 200}); err != nil {
						return err
					}
				}
				return nil
			}},
		},
	},
	{
		name:    "with associated files",
		version: pdf.V2_0,
		data: &Dict{
			Width:            2,
			Height:           2,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 8,
			AssociatedFiles: []*file.Specification{
				{
					FileName:        "image-data.xml",
					FileNameUnicode: "image-data.xml",
					AFRelationship:  file.RelationshipData,
				},
			},
			Data: &FlateSource{Predictor: 15, Width: 2, Colors: 1, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				_, err := w.Write([]byte{10, 20, 30, 40})
				return err
			}},
		},
	},
	{
		name:    "with web capture ID",
		version: pdf.V1_7,
		data: &Dict{
			Width:            2,
			Height:           2,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 8,
			WebCaptureID: &webcapture.Identifier{
				ID: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
					0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
			},
			Data: &FlateSource{Predictor: 15, Width: 2, Colors: 1, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				_, err := w.Write([]byte{50, 100, 150, 200})
				return err
			}},
		},
	},
	{
		name:    "with deprecated Name",
		version: pdf.V1_7,
		data: &Dict{
			Width:            2,
			Height:           2,
			ColorSpace:       color.SpaceDeviceGray,
			BitsPerComponent: 8,
			Name:             "Im0",
			Data: &FlateSource{Predictor: 15, Width: 2, Colors: 1, BitsPerComponent: 8, WriteData: func(w io.Writer) error {
				_, err := w.Write([]byte{0, 255, 128, 64})
				return err
			}},
		},
	},
}

// checkDictData validates that all image data in a Dict can be read.
//
// Image data is loaded lazily via WriteData closures that reference the
// original PDF stream. Corrupted compressed data (e.g. bad zlib checksum)
// is only detected when WriteData is actually called. This function forces
// early detection before round-trip testing.
func checkDictData(d *Dict) error {
	if _, err := d.Data.Pixels(); err != nil {
		return err
	}
	if d.SMask != nil {
		if _, err := d.SMask.Source.Pixels(); err != nil {
			return err
		}
	}
	if d.MaskImage != nil {
		if _, err := d.MaskImage.Source.Pixels(); err != nil {
			return err
		}
	}
	for _, alt := range d.Alternates {
		if alt == nil {
			continue
		}
		if img, ok := alt.Image.(*Dict); ok {
			if err := checkDictData(img); err != nil {
				return err
			}
		}
	}
	return nil
}

func roundTripTest(t *testing.T, version pdf.Version, data *Dict) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	// Embed the original data
	ref, err := rm.Embed(data)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
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
	decoded, err := ExtractDict(x, nil, ref, false)
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

	if decoded.WebCaptureID != nil && data.WebCaptureID != nil {
		decoded.WebCaptureID.SingleUse = data.WebCaptureID.SingleUse
	}

	// normalize Decode (read fills in the default)
	if data.Decode == nil {
		data.Decode = DefaultDecode(data.ColorSpace, data.BitsPerComponent)
	}
	for _, alt := range data.Alternates {
		if alt == nil {
			continue
		}
		if img, ok := alt.Image.(*Dict); ok && img.Decode == nil {
			img.Decode = DefaultDecode(img.ColorSpace, img.BitsPerComponent)
		}
	}

	// Compare the round-tripped data.
	// Source types differ (FlateSource vs streamData), so compare via
	// Pixels() output.
	opts := []cmp.Option{
		cmp.AllowUnexported(Dict{}, SoftMask{}, measure.RectilinearMeasure{}, measure.PtData{}, oc.Group{}),
		cmp.Comparer(func(a, b graphics.ImageData) bool {
			pixA, errA := a.Pixels()
			pixB, errB := b.Pixels()
			return errA == nil && errB == nil && bytes.Equal(pixA, pixB)
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

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		ref, err := rm.Embed(tc.data)
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

	// Seed: JPXDecode image without /ColorSpace, a case that previously
	// panicked.  We do not embed via the ResourceManager because there
	// is no JPXSource for API-side construction; instead, write the
	// dict directly.
	{
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)
		_ = memfile.AddBlankPage(w)
		ref := w.Alloc()
		body, err := w.OpenStream(ref, pdf.Dict{
			"Type":    pdf.Name("XObject"),
			"Subtype": pdf.Name("Image"),
			"Width":   pdf.Integer(10),
			"Height":  pdf.Integer(10),
			"Filter":  pdf.Name("JPXDecode"),
			"Mask":    pdf.Array{pdf.Integer(0), pdf.Integer(255)},
		})
		if err == nil {
			body.Close()
			w.GetMeta().Trailer["Quir:E"] = ref
			if err := w.Close(); err == nil {
				f.Add(buf.Data)
			}
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing PDF object")
		}

		x := pdf.NewExtractor(r)
		objGo, err := ExtractDict(x, nil, objPDF, false)
		if err != nil {
			t.Skip("malformed PDF object")
		}

		// skip if image data cannot be read (e.g. unsupported filter)
		if err := checkDictData(objGo); err != nil {
			t.Skip("image data not readable")
		}

		roundTripTest(t, pdf.GetVersion(r), objGo)
	})
}

// TestExtractDictDecodedFloat64Oversize verifies that an image dict
// whose per-channel float64 decode buffer would exceed
// streamlimits.MaxImageDecodedFloat64Bytes is rejected, even when the
// encoded-byte and pixel-count caps both pass.  At bpc=1 the float64
// expansion ratio is 64×, so a CMYK image just under the pixel cap
// passes ImageBytesExceedLimit (~64 MiB encoded) yet would allocate
// ~4 GiB of float64s in Load().
func TestExtractDictDecodedFloat64Oversize(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref := w.Alloc()
	body, err := w.OpenStream(ref, pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(16384),
		"Height":           pdf.Integer(8191),
		"BitsPerComponent": pdf.Integer(1),
		"ColorSpace":       pdf.Name("DeviceCMYK"),
		"Filter":           pdf.Name("FlateDecode"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	if _, err := ExtractDict(x, nil, ref, false); err == nil {
		t.Fatal("expected error for float64-oversize image dict, got nil")
	}
}

// TestExtractDictTooManyAlternates verifies that when an image's
// Alternates array exceeds streamlimits.MaxAlternates, every entry is
// silently dropped — not truncated.  An over-long Alternates list is a
// strong signal of a malicious construction (the spec describes
// Alternates as a small set of variants), so the safer choice is to
// surface no alternates at all rather than make an arbitrary cut.
func TestExtractDictTooManyAlternates(t *testing.T) {
	for _, kind := range []string{"under cap", "over cap"} {
		t.Run(kind, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

			// build a single small alternate-target image XObject
			altRef := w.Alloc()
			altBody, err := w.OpenStream(altRef, pdf.Dict{
				"Type":             pdf.Name("XObject"),
				"Subtype":          pdf.Name("Image"),
				"Width":            pdf.Integer(2),
				"Height":           pdf.Integer(2),
				"ColorSpace":       pdf.Name("DeviceGray"),
				"BitsPerComponent": pdf.Integer(8),
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := altBody.Write([]byte{0, 0, 0, 0}); err != nil {
				t.Fatal(err)
			}
			if err := altBody.Close(); err != nil {
				t.Fatal(err)
			}

			n := streamlimits.MaxAlternates
			if kind == "over cap" {
				n = streamlimits.MaxAlternates + 1
			}
			alts := make(pdf.Array, n)
			for i := range alts {
				alts[i] = pdf.Dict{"Image": altRef}
			}

			ref := w.Alloc()
			body, err := w.OpenStream(ref, pdf.Dict{
				"Type":             pdf.Name("XObject"),
				"Subtype":          pdf.Name("Image"),
				"Width":            pdf.Integer(4),
				"Height":           pdf.Integer(4),
				"ColorSpace":       pdf.Name("DeviceGray"),
				"BitsPerComponent": pdf.Integer(8),
				"Alternates":       alts,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := body.Write(make([]byte, 16)); err != nil {
				t.Fatal(err)
			}
			if err := body.Close(); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(w)
			img, err := ExtractDict(x, nil, ref, false)
			if err != nil {
				t.Fatalf("ExtractDict failed: %v", err)
			}

			want := n
			if kind == "over cap" {
				want = 0
			}
			if got := len(img.Alternates); got != want {
				t.Errorf("len(Alternates) = %d, want %d", got, want)
			}
		})
	}
}

// TestExtractMaskTooManyAlternates verifies the same all-or-nothing
// cap on Alternates for image masks.  See [TestExtractDictTooManyAlternates].
func TestExtractMaskTooManyAlternates(t *testing.T) {
	for _, kind := range []string{"under cap", "over cap"} {
		t.Run(kind, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

			altRef := w.Alloc()
			altBody, err := w.OpenStream(altRef, pdf.Dict{
				"Type":             pdf.Name("XObject"),
				"Subtype":          pdf.Name("Image"),
				"Width":            pdf.Integer(8),
				"Height":           pdf.Integer(8),
				"ImageMask":        pdf.Boolean(true),
				"BitsPerComponent": pdf.Integer(1),
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := altBody.Write(make([]byte, 8)); err != nil {
				t.Fatal(err)
			}
			if err := altBody.Close(); err != nil {
				t.Fatal(err)
			}

			n := streamlimits.MaxAlternates
			if kind == "over cap" {
				n = streamlimits.MaxAlternates + 1
			}
			alts := make(pdf.Array, n)
			for i := range alts {
				alts[i] = pdf.Dict{"Image": altRef}
			}

			ref := w.Alloc()
			body, err := w.OpenStream(ref, pdf.Dict{
				"Type":             pdf.Name("XObject"),
				"Subtype":          pdf.Name("Image"),
				"Width":            pdf.Integer(8),
				"Height":           pdf.Integer(8),
				"ImageMask":        pdf.Boolean(true),
				"BitsPerComponent": pdf.Integer(1),
				"Alternates":       alts,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := body.Write(make([]byte, 8)); err != nil {
				t.Fatal(err)
			}
			if err := body.Close(); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(w)
			mask, err := ExtractMask(x, nil, ref, false)
			if err != nil {
				t.Fatalf("ExtractMask failed: %v", err)
			}

			want := n
			if kind == "over cap" {
				want = 0
			}
			if got := len(mask.Alternates); got != want {
				t.Errorf("len(Alternates) = %d, want %d", got, want)
			}
		})
	}
}

// TestExtractDictTooManyAssociatedFiles verifies that when an image's
// AF (associated-files) array exceeds streamlimits.MaxAssociatedFiles,
// every entry is silently dropped — not truncated.  An over-long AF
// list is a strong signal of a malicious construction; dropping the
// list rather than capping it avoids silently presenting only a prefix.
func TestExtractDictTooManyAssociatedFiles(t *testing.T) {
	for _, kind := range []string{"under cap", "over cap"} {
		t.Run(kind, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)

			// build a single tiny file-spec object referenced many times
			specRef := w.Alloc()
			if err := w.Put(specRef, pdf.Dict{
				"Type": pdf.Name("Filespec"),
				"F":    pdf.String("attachment.txt"),
			}); err != nil {
				t.Fatal(err)
			}

			n := streamlimits.MaxAssociatedFiles
			if kind == "over cap" {
				n = streamlimits.MaxAssociatedFiles + 1
			}
			afs := make(pdf.Array, n)
			for i := range afs {
				afs[i] = specRef
			}

			ref := w.Alloc()
			body, err := w.OpenStream(ref, pdf.Dict{
				"Type":             pdf.Name("XObject"),
				"Subtype":          pdf.Name("Image"),
				"Width":            pdf.Integer(2),
				"Height":           pdf.Integer(2),
				"ColorSpace":       pdf.Name("DeviceGray"),
				"BitsPerComponent": pdf.Integer(8),
				"AF":               afs,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := body.Write([]byte{0, 0, 0, 0}); err != nil {
				t.Fatal(err)
			}
			if err := body.Close(); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(w)
			img, err := ExtractDict(x, nil, ref, false)
			if err != nil {
				t.Fatalf("ExtractDict failed: %v", err)
			}

			want := n
			if kind == "over cap" {
				want = 0
			}
			if got := len(img.AssociatedFiles); got != want {
				t.Errorf("len(AssociatedFiles) = %d, want %d", got, want)
			}
		})
	}
}

// TestExtractMaskTooManyAssociatedFiles verifies the same all-or-nothing
// cap on AF for image masks.  See [TestExtractDictTooManyAssociatedFiles].
func TestExtractMaskTooManyAssociatedFiles(t *testing.T) {
	for _, kind := range []string{"under cap", "over cap"} {
		t.Run(kind, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)

			specRef := w.Alloc()
			if err := w.Put(specRef, pdf.Dict{
				"Type": pdf.Name("Filespec"),
				"F":    pdf.String("attachment.txt"),
			}); err != nil {
				t.Fatal(err)
			}

			n := streamlimits.MaxAssociatedFiles
			if kind == "over cap" {
				n = streamlimits.MaxAssociatedFiles + 1
			}
			afs := make(pdf.Array, n)
			for i := range afs {
				afs[i] = specRef
			}

			ref := w.Alloc()
			body, err := w.OpenStream(ref, pdf.Dict{
				"Type":             pdf.Name("XObject"),
				"Subtype":          pdf.Name("Image"),
				"Width":            pdf.Integer(8),
				"Height":           pdf.Integer(8),
				"ImageMask":        pdf.Boolean(true),
				"BitsPerComponent": pdf.Integer(1),
				"AF":               afs,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := body.Write(make([]byte, 8)); err != nil {
				t.Fatal(err)
			}
			if err := body.Close(); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(w)
			mask, err := ExtractMask(x, nil, ref, false)
			if err != nil {
				t.Fatalf("ExtractMask failed: %v", err)
			}

			want := n
			if kind == "over cap" {
				want = 0
			}
			if got := len(mask.AssociatedFiles); got != want {
				t.Errorf("len(AssociatedFiles) = %d, want %d", got, want)
			}
		})
	}
}

// writeJPXImage embeds a JPXDecode image XObject with the given extra
// dictionary entries and an empty payload, then re-extracts it.  It is a
// helper for JPX-no-ColorSpace tests where we exercise the dict-level
// rules without needing valid JP2 codestream bytes.
func writeJPXImage(t *testing.T, version pdf.Version, extras pdf.Dict) (*Dict, error) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(version, nil)
	ref := w.Alloc()
	dict := pdf.Dict{
		"Type":    pdf.Name("XObject"),
		"Subtype": pdf.Name("Image"),
		"Width":   pdf.Integer(10),
		"Height":  pdf.Integer(10),
		"Filter":  pdf.Name("JPXDecode"),
	}
	maps.Copy(dict, extras)
	body, err := w.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	x := pdf.NewExtractor(w)
	return ExtractDict(x, nil, ref, false)
}

// TestExtractDictJPXNoColorSpace covers a JPXDecode image XObject
// without /ColorSpace, which previously panicked at dict.go:328 with a
// nil pointer dereference.
func TestExtractDictJPXNoColorSpace(t *testing.T) {
	img, err := writeJPXImage(t, pdf.V1_7, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img.ColorSpace != nil {
		t.Errorf("ColorSpace = %v, want nil", img.ColorSpace)
	}
	if img.BitsPerComponent != 0 {
		t.Errorf("BitsPerComponent = %d, want 0", img.BitsPerComponent)
	}
	if img.Decode != nil {
		t.Errorf("Decode = %v, want nil (spec §7.4.9 ignores Decode when ColorSpace absent)", img.Decode)
	}
}

// TestExtractDictJPXMaskArray exercises the colour-key /Mask Array path
// for a JPX image without /ColorSpace, where the channel count is
// unknown until the JP2 codestream is parsed.
func TestExtractDictJPXMaskArray(t *testing.T) {
	tests := []struct {
		name   string
		mask   pdf.Array
		expect []uint16 // expected MaskColors, or nil if dropped
	}{
		{
			name:   "valid 1-channel pair",
			mask:   pdf.Array{pdf.Integer(0), pdf.Integer(255)},
			expect: []uint16{0, 255},
		},
		{
			name:   "valid 4-channel",
			mask:   pdf.Array{pdf.Integer(0), pdf.Integer(10), pdf.Integer(0), pdf.Integer(10), pdf.Integer(0), pdf.Integer(10), pdf.Integer(0), pdf.Integer(10)},
			expect: []uint16{0, 10, 0, 10, 0, 10, 0, 10},
		},
		{
			name:   "odd length dropped",
			mask:   pdf.Array{pdf.Integer(0), pdf.Integer(255), pdf.Integer(100)},
			expect: nil,
		},
		{
			name:   "empty dropped",
			mask:   pdf.Array{},
			expect: nil,
		},
		{
			name:   "out-of-uint16-range dropped",
			mask:   pdf.Array{pdf.Integer(0), pdf.Integer(100000)},
			expect: nil,
		},
		{
			name:   "min greater than max dropped",
			mask:   pdf.Array{pdf.Integer(200), pdf.Integer(100)},
			expect: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			img, err := writeJPXImage(t, pdf.V1_7, pdf.Dict{"Mask": tc.mask})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.expect, img.MaskColors); diff != "" {
				t.Errorf("MaskColors (-want +got):\n%s", diff)
			}
		})
	}
}

// TestExtractDictJPXMaskOverLength verifies that a colour-key /Mask
// Array with more pairs than streamlimits.MaxImageChannels is dropped
// rather than allocated.
func TestExtractDictJPXMaskOverLength(t *testing.T) {
	mask := make(pdf.Array, 2*streamlimits.MaxImageChannels+2)
	for i := range mask {
		mask[i] = pdf.Integer(0)
	}
	img, err := writeJPXImage(t, pdf.V1_7, pdf.Dict{"Mask": mask})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img.MaskColors != nil {
		t.Errorf("MaskColors = %v, want nil (over-length array should be dropped)", img.MaskColors)
	}
}

// TestExtractDictJPXBPCIgnored verifies that an explicit
// /BitsPerComponent on a JPXDecode image is dropped on extract, matching
// the check()-time requirement that JPX images carry BPC=0.  Without
// this, a JPX source dict with BPC=8 extracted to BPC=8 and then failed
// to re-embed.
func TestExtractDictJPXBPCIgnored(t *testing.T) {
	for _, bpc := range []pdf.Integer{1, 2, 4, 8, 16, 7, 99} {
		img, err := writeJPXImage(t, pdf.V1_7, pdf.Dict{
			"BitsPerComponent": bpc,
		})
		if err != nil {
			t.Fatalf("BPC=%d: unexpected error: %v", bpc, err)
		}
		if img.BitsPerComponent != 0 {
			t.Errorf("BPC=%d in source: extracted BitsPerComponent=%d, want 0",
				bpc, img.BitsPerComponent)
		}

		// re-embed must succeed, demonstrating round-trip parity
		w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		rm := pdf.NewResourceManager(w)
		if _, err := rm.Embed(img); err != nil {
			t.Errorf("BPC=%d in source: re-embed failed: %v", bpc, err)
		}
	}
}

// TestExtractDictJPXDecodeIgnored verifies that a /Decode array on a
// JPX-no-ColorSpace image is silently dropped per spec §7.4.9.
func TestExtractDictJPXDecodeIgnored(t *testing.T) {
	img, err := writeJPXImage(t, pdf.V1_7, pdf.Dict{
		"Decode": pdf.Array{pdf.Number(0), pdf.Number(1), pdf.Number(0), pdf.Number(1), pdf.Number(0), pdf.Number(1)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img.Decode != nil {
		t.Errorf("Decode = %v, want nil", img.Decode)
	}
}

// TestExtractDictDecodeLength checks how ExtractDict normalizes the Decode
// array length: a short array falls back to the default, while surplus entries
// beyond 2*ncomp are dropped so the stored array round-trips on write.
func TestExtractDictDecodeLength(t *testing.T) {
	rgbDefault := DefaultDecode(color.SpaceDeviceRGB, 8) // [0 1 0 1 0 1]
	tests := []struct {
		name   string
		decode pdf.Array
		want   []float64
	}{
		{"exact", pdf.Array{pdf.Number(0), pdf.Number(1), pdf.Number(0), pdf.Number(1), pdf.Number(0), pdf.Number(1)}, []float64{0, 1, 0, 1, 0, 1}},
		{"short", pdf.Array{pdf.Number(0), pdf.Number(1)}, rgbDefault},
		{"long", pdf.Array{pdf.Number(1), pdf.Number(0), pdf.Number(1), pdf.Number(0), pdf.Number(1), pdf.Number(0), pdf.Number(9), pdf.Number(9)}, []float64{1, 0, 1, 0, 1, 0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			img := extractRGBImageWithDecode(t, tc.decode)
			if diff := cmp.Diff(tc.want, img.Decode); diff != "" {
				t.Errorf("Decode (-want +got):\n%s", diff)
			}
		})
	}
}

// extractRGBImageWithDecode writes a 2x2 DeviceRGB image XObject carrying the
// given /Decode array, then re-extracts it.
func extractRGBImageWithDecode(t *testing.T, decode pdf.Array) *Dict {
	t.Helper()
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref := w.Alloc()
	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(2),
		"Height":           pdf.Integer(2),
		"ColorSpace":       pdf.Name("DeviceRGB"),
		"BitsPerComponent": pdf.Integer(8),
		"Decode":           decode,
	}
	body, err := w.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := body.Write(make([]byte, 2*2*3)); err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	x := pdf.NewExtractor(w)
	img, err := ExtractDict(x, nil, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// TestDictJPXRoundTrip verifies that a JPX-no-ColorSpace Dict survives
// Embed → ExtractDict.
func TestDictJPXRoundTrip(t *testing.T) {
	orig, err := writeJPXImage(t, pdf.V1_7, pdf.Dict{
		"Mask": pdf.Array{pdf.Integer(0), pdf.Integer(255)},
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	w2, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w2)
	ref, err := rm.Embed(orig)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("w2.Close: %v", err)
	}

	x := pdf.NewExtractor(w2)
	got, err := ExtractDict(x, nil, ref, false)
	if err != nil {
		t.Fatalf("ExtractDict: %v", err)
	}
	if got.ColorSpace != nil {
		t.Errorf("ColorSpace = %v, want nil", got.ColorSpace)
	}
	if got.BitsPerComponent != 0 {
		t.Errorf("BitsPerComponent = %d, want 0", got.BitsPerComponent)
	}
	if got.Decode != nil {
		t.Errorf("Decode = %v, want nil", got.Decode)
	}
	if diff := cmp.Diff(orig.MaskColors, got.MaskColors); diff != "" {
		t.Errorf("MaskColors (-want +got):\n%s", diff)
	}
}

// TestDictRejectInvalidWriteCombinations confirms the spec-derived
// write-time rejections.
func TestDictRejectInvalidWriteCombinations(t *testing.T) {
	rgbData := &FlateSource{
		Predictor: 15, Width: 1, Colors: 3, BitsPerComponent: 8,
		WriteData: func(w io.Writer) error {
			_, err := w.Write([]byte{0, 0, 0})
			return err
		},
	}

	tests := []struct {
		name string
		d    *Dict
	}{
		{
			name: "nil ColorSpace + non-JPX data",
			d: &Dict{
				Width: 1, Height: 1,
				BitsPerComponent: 8,
				Data:             rgbData,
			},
		},
		{
			name: "non-JPX + non-zero SMaskInData",
			d: &Dict{
				Width: 1, Height: 1,
				ColorSpace:       color.SpaceDeviceRGB,
				BitsPerComponent: 8,
				SMaskInData:      1,
				Data:             rgbData,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(tc.d); err == nil {
				t.Error("expected Embed to fail, got nil error")
			}
		})
	}
}

// TestExtractDictJPXOversizePixels verifies that a JPXDecode image
// whose pixel count exceeds streamlimits.MaxImagePixels is rejected
// even though ColorSpace and BitsPerComponent are absent.  The
// byte-count cap does not fire on its own for JPX (channels and bit
// depth live in the JP2 codestream), so the pixel-count cap is the
// only defence at the dictionary level.
func TestExtractDictJPXOversizePixels(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref := w.Alloc()
	body, err := w.OpenStream(ref, pdf.Dict{
		"Type":    pdf.Name("XObject"),
		"Subtype": pdf.Name("Image"),
		"Width":   pdf.Integer(streamlimits.MaxImageWidth),
		"Height":  pdf.Integer(streamlimits.MaxImageHeight),
		"Filter":  pdf.Name("JPXDecode"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	if _, err := ExtractDict(x, nil, ref, false); err == nil {
		t.Fatal("expected error for oversize JPX image dict, got nil")
	}
}

// TestExtractDictDCTDimensionMismatch documents the behaviour for an
// Image XObject whose /Width and /Height differ from the dimensions
// declared in the embedded JPEG's SOF marker.  DCTDecode is treated as
// a generic byte-stream filter: the consumer reads
// W·H·nComp·bpc/8 bytes from the decoded stream, regardless of the
// JPEG's intrinsic size.  This matches Adobe Acrobat Reader and
// Ghostscript; see viewer-tests/image/jpeg-mismatch.
func TestExtractDictDCTDimensionMismatch(t *testing.T) {
	// build a 64×64 baseline RGB JPEG, then embed it as an XObject with
	// /Width 32 /Height 32
	src := stdimage.NewRGBA(stdimage.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			src.Set(x, y, stdcolor.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: 128, A: 255})
		}
	}
	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, src, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref := w.Alloc()
	body, err := w.OpenStream(ref, pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(32),
		"Height":           pdf.Integer(32),
		"ColorSpace":       pdf.Name("DeviceRGB"),
		"BitsPerComponent": pdf.Integer(8),
		"Filter":           pdf.Name("DCTDecode"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := body.Write(jpegBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	img, err := ExtractDict(x, nil, ref, false)
	if err != nil {
		t.Fatal(err)
	}

	pixels, err := img.Data.Pixels()
	if err != nil {
		t.Fatal(err)
	}
	const want = 32 * 32 * 3
	if len(pixels) != want {
		t.Errorf("Pixels() returned %d bytes, want %d (W·H·nComp = 32·32·3)", len(pixels), want)
	}
}
