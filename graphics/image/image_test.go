// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"image"
	gocol "image/color"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/measure"
)

var (
	_ graphics.XObject = graphics.Image(nil)
)

// TestPNGRefactored verifies that the PNG function creates proper Dict objects.
func TestPNGRefactored(t *testing.T) {
	writer, _ := memfile.NewPDFWriter(pdf.V1_4, nil)
	rm := pdf.NewResourceManager(writer)

	// Create a test image with transparency
	testImg := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			// Create a gradient with varying alpha
			alpha := uint8((x + y) * 32)
			testImg.Set(x, y, gocol.RGBA{R: uint8(x * 64), G: uint8(y * 64), B: 128, A: alpha})
		}
	}

	// Test PNG function without explicit color space (should default to DeviceRGB)
	dict1, err := PNG(testImg, nil)
	if err != nil {
		t.Fatalf("PNG function failed: %v", err)
	}
	if dict1 == nil {
		t.Fatal("PNG function returned nil dict")
	}
	if dict1.ColorSpace != color.SpaceDeviceRGB {
		t.Errorf("PNG function didn't default to DeviceRGB, got %v", dict1.ColorSpace)
	}
	if dict1.SMask == nil {
		t.Error("PNG function should have created soft mask for image with alpha")
	}

	// Test embedding the dict
	ref1, _, err := pdf.ResourceManagerEmbed(rm, dict1)
	if err != nil {
		t.Fatalf("Failed to embed PNG dict: %v", err)
	}
	if ref1 == nil {
		t.Error("PNG dict embedding returned nil reference")
	}

	// Test PNG with explicit color space
	dict2, err := PNG(testImg, color.SpaceDeviceRGB)
	if err != nil {
		t.Fatalf("PNG function with ColorSpace failed: %v", err)
	}
	if dict2.ColorSpace != color.SpaceDeviceRGB {
		t.Errorf("PNG function didn't use explicit ColorSpace, got %v", dict2.ColorSpace)
	}

	ref2, _, err := pdf.ResourceManagerEmbed(rm, dict2)
	if err != nil {
		t.Fatalf("Failed to embed PNG dict with explicit ColorSpace: %v", err)
	}
	if ref2 == nil {
		t.Error("PNG dict embedding with ColorSpace returned nil reference")
	}

	// Test PNG with opaque image (no alpha channel)
	opaqueImg := image.NewRGBA(image.Rect(0, 0, 2, 2))
	opaqueImg.Set(0, 0, gocol.RGBA{R: 255, G: 0, B: 0, A: 255})
	opaqueImg.Set(1, 0, gocol.RGBA{R: 0, G: 255, B: 0, A: 255})
	opaqueImg.Set(0, 1, gocol.RGBA{R: 0, G: 0, B: 255, A: 255})
	opaqueImg.Set(1, 1, gocol.RGBA{R: 255, G: 255, B: 0, A: 255})

	dict3, err := PNG(opaqueImg, nil)
	if err != nil {
		t.Fatalf("PNG function with opaque image failed: %v", err)
	}
	if dict3.SMask != nil {
		t.Error("PNG function should not create soft mask for opaque image")
	}

	ref3, _, err := pdf.ResourceManagerEmbed(rm, dict3)
	if err != nil {
		t.Fatalf("Failed to embed opaque PNG dict: %v", err)
	}
	if ref3 == nil {
		t.Error("Opaque PNG dict embedding returned nil reference")
	}

	// Test error handling
	_, err = PNG(nil, nil)
	if err == nil {
		t.Error("PNG function should return error for nil image")
	}

	// Close resource manager and writer
	err = rm.Close()
	if err != nil {
		t.Fatalf("Failed to close resource manager: %v", err)
	}
	err = writer.Close()
	if err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Verify Dict implements Image interface correctly
	if dict1.Subtype() != "Image" {
		t.Errorf("Dict Subtype() returned %q, want %q", dict1.Subtype(), "Image")
	}

	bounds := dict1.Bounds()
	if bounds.XMin != 0 || bounds.YMin != 0 || bounds.XMax != 4 || bounds.YMax != 4 {
		t.Errorf("Dict Bounds() returned %+v, want {XMin:0 YMin:0 XMax:4 YMax:4}", bounds)
	}

	// Test that PNG Dict can now use AssociatedFiles (new functionality!)
	if dict1.AssociatedFiles == nil {
		// This is expected - just verify the field exists
		dict1.AssociatedFiles = []*file.Specification{} // Should compile without error
	}
}

// TestMaskWithPtData verifies that PtData is properly handled during
// image mask read/write cycles.
func TestMaskWithPtData(t *testing.T) {
	writer1, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm1 := pdf.NewResourceManager(writer1)

	// create test PtData with some geospatial point data
	testPtData := &measure.PtData{
		Subtype: measure.PtDataSubtypeCloud,
		Names:   []string{measure.PtDataNameLat, measure.PtDataNameLon, measure.PtDataNameAlt},
		XPTS: [][]pdf.Object{
			{pdf.Number(40.7128), pdf.Number(-74.0060), pdf.Number(10.5)}, // NYC coordinates
			{pdf.Number(40.7589), pdf.Number(-73.9851), pdf.Number(15.2)}, // Central Park
		},
		SingleUse: false, // use as indirect object
	}

	// create a simple test mask (2x2 pixels)
	testImg := image.NewRGBA(image.Rect(0, 0, 2, 2))
	testImg.Set(0, 0, gocol.RGBA{R: 0, G: 0, B: 0, A: 255}) // opaque
	testImg.Set(1, 0, gocol.RGBA{R: 0, G: 0, B: 0, A: 0})   // transparent
	testImg.Set(0, 1, gocol.RGBA{R: 0, G: 0, B: 0, A: 128}) // semi-transparent -> opaque
	testImg.Set(1, 1, gocol.RGBA{R: 0, G: 0, B: 0, A: 64})  // semi-transparent -> transparent

	mask0 := FromImageMask(testImg)
	mask0.PtData = testPtData

	ref, _, err := pdf.ResourceManagerEmbed(rm1, mask0)
	if err != nil {
		t.Fatal(err)
	}
	err = rm1.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer1.Close()
	if err != nil {
		t.Fatal(err)
	}

	x1 := pdf.NewExtractor(writer1)
	mask1, err := ExtractMask(x1, ref)
	if err != nil {
		t.Fatal(err)
	}

	// verify PtData was preserved
	if mask1.PtData == nil {
		t.Error("PtData was not preserved during extraction")
		return
	}

	// check PtData content
	if mask1.PtData.Subtype != measure.PtDataSubtypeCloud {
		t.Errorf("PtData subtype mismatch: got %s, want %s", mask1.PtData.Subtype, measure.PtDataSubtypeCloud)
	}
	if len(mask1.PtData.Names) != 3 {
		t.Errorf("PtData names length mismatch: got %d, want 3", len(mask1.PtData.Names))
	}
	if len(mask1.PtData.XPTS) != 2 {
		t.Errorf("PtData XPTS length mismatch: got %d, want 2", len(mask1.PtData.XPTS))
	}

	// test round-trip
	writer2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(writer2)
	ref2, _, err := pdf.ResourceManagerEmbed(rm2, mask1)
	if err != nil {
		t.Fatal(err)
	}
	err = rm2.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer2.Close()
	if err != nil {
		t.Fatal(err)
	}

	x2 := pdf.NewExtractor(writer2)
	mask2, err := ExtractMask(x2, ref2)
	if err != nil {
		t.Fatal(err)
	}

	// check that PtData round-tripped correctly
	if mask2.PtData == nil {
		t.Error("PtData was lost during round-trip")
		return
	}

	// use cmp to compare the PtData structures
	if diff := cmp.Diff(mask1.PtData, mask2.PtData, cmp.AllowUnexported(measure.PtData{})); diff != "" {
		t.Errorf("PtData round trip failed (-got +want):\n%s", diff)
	}

	// verify basic mask properties were preserved
	if mask1.Width != mask2.Width || mask1.Height != mask2.Height {
		t.Errorf("Mask dimensions changed: %dx%d -> %dx%d", mask1.Width, mask1.Height, mask2.Width, mask2.Height)
	}
}
