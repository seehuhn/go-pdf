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
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/measure"
)

var (
	_ graphics.XObject = Image(nil)
)

// TestImageWithPtData verifies that PtData is properly handled during
// image dictionary read/write cycles.
func TestImageWithPtData(t *testing.T) {
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

	// create a simple test image (2x2 gray pixels)
	testImg := image.NewGray(image.Rect(0, 0, 2, 2))
	testImg.Set(0, 0, gocol.Gray{Y: 128})
	testImg.Set(1, 0, gocol.Gray{Y: 64})
	testImg.Set(0, 1, gocol.Gray{Y: 192})
	testImg.Set(1, 1, gocol.Gray{Y: 255})

	dict0 := FromImage(testImg, color.DeviceGraySpace, 8)
	dict0.PtData = testPtData

	ref, _, err := pdf.ResourceManagerEmbed(rm1, dict0)
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

	dict1, err := ExtractDict(writer1, ref)
	if err != nil {
		t.Fatal(err)
	}

	// verify PtData was preserved
	if dict1.PtData == nil {
		t.Error("PtData was not preserved during extraction")
		return
	}

	// check PtData content
	if dict1.PtData.Subtype != measure.PtDataSubtypeCloud {
		t.Errorf("PtData subtype mismatch: got %s, want %s", dict1.PtData.Subtype, measure.PtDataSubtypeCloud)
	}
	if len(dict1.PtData.Names) != 3 {
		t.Errorf("PtData names length mismatch: got %d, want 3", len(dict1.PtData.Names))
	}
	if len(dict1.PtData.XPTS) != 2 {
		t.Errorf("PtData XPTS length mismatch: got %d, want 2", len(dict1.PtData.XPTS))
	}

	// test round-trip
	writer2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(writer2)
	ref2, _, err := pdf.ResourceManagerEmbed(rm2, dict1)
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

	dict2, err := ExtractDict(writer2, ref2)
	if err != nil {
		t.Fatal(err)
	}

	// check that PtData round-tripped correctly
	if dict2.PtData == nil {
		t.Error("PtData was lost during round-trip")
		return
	}

	// use cmp to compare the PtData structures
	if diff := cmp.Diff(dict1.PtData, dict2.PtData, cmp.AllowUnexported(measure.PtData{})); diff != "" {
		t.Errorf("PtData round trip failed (-got +want):\n%s", diff)
	}

	// verify basic image properties were preserved
	if dict1.Width != dict2.Width || dict1.Height != dict2.Height {
		t.Errorf("Image dimensions changed: %dx%d -> %dx%d", dict1.Width, dict1.Height, dict2.Width, dict2.Height)
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

	mask1, err := ExtractMask(writer1, ref)
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

	mask2, err := ExtractMask(writer2, ref2)
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
