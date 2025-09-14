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
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 8.9.5 12.3.4

// Thumbnail represents a thumbnail image for a PDF page.
// Thumbnail images are miniature representations of page contents that can be
// displayed for navigation purposes.
type Thumbnail struct {
	// Width is the width of the thumbnail image in pixels.
	Width int

	// Height is the height of the thumbnail image in pixels.
	Height int

	// ColorSpace is the color space of the thumbnail image. Must be
	// [color.SpaceDeviceGray], [color.SpaceDeviceRGB], or a
	// [color.SpaceIndexed] based on one of these.
	ColorSpace color.Space

	// BitsPerComponent is the number of bits used to represent each color component.
	// The value must be 1, 2, 4, 8 or 16.
	BitsPerComponent int

	// Decode (optional) is an array of numbers describing how to map image
	// samples into the range of values appropriate for the image's color
	// space. The slice must have twice the number of color components
	// required by ColorSpace.
	Decode []float64

	// WriteData is a function that writes the thumbnail data to the provided writer.
	// The data should be written row by row, with each row containing
	// Width * ColorSpace.Channels() samples, each sample using BitsPerComponent bits.
	WriteData func(io.Writer) error
}

var _ Image = (*Thumbnail)(nil)

// ExtractThumbnail reads a thumbnail image from a PDF object.
func ExtractThumbnail(x *pdf.Extractor, obj pdf.Object) (*Thumbnail, error) {
	stm, err := pdf.GetStream(x.R, obj)
	if err != nil {
		return nil, err
	} else if stm == nil {
		return nil, pdf.Error("missing thumbnail stream")
	}

	dict := stm.Dict

	// Check Type and Subtype
	if err := pdf.CheckDictType(x.R, dict, "XObject"); err != nil {
		return nil, err
	}
	if subtypeName, err := pdf.Optional(pdf.GetName(x.R, dict["Subtype"])); err != nil {
		return nil, err
	} else if subtypeName != "Image" && subtypeName != "" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("invalid Subtype %q for soft mask XObject", subtypeName),
		}
	}

	thumb := &Thumbnail{}

	// width (required)
	width, err := pdf.GetInteger(x.R, dict["Width"])
	if err != nil {
		return nil, err
	} else if width <= 0 {
		return nil, pdf.Error("invalid thumbnail width")
	}
	thumb.Width = int(width)

	// height (required)
	height, err := pdf.GetInteger(x.R, dict["Height"])
	if err != nil {
		return nil, err
	} else if height <= 0 {
		return nil, pdf.Error("invalid thumbnail height")
	}
	thumb.Height = int(height)

	// color space (required)
	cs, err := color.ExtractSpace(x.R, dict["ColorSpace"])
	if err != nil {
		return nil, err
	} else if cs == nil {
		return nil, pdf.Error("missing thumbnail color space")
	}

	// validate color space
	if !isValidThumbnailColorSpace(cs) {
		return nil, pdf.Error("invalid thumbnail color space")
	}
	thumb.ColorSpace = cs

	// bits per component (required)
	bpc, err := pdf.GetInteger(x.R, dict["BitsPerComponent"])
	if err != nil {
		return nil, err
	} else if !isValidBitsPerComponent(int(bpc)) {
		return nil, pdf.Error("invalid BitsPerComponent value")
	}
	thumb.BitsPerComponent = int(bpc)

	// decode array (optional)
	if decodeArray, err := pdf.Optional(pdf.GetArray(x.R, dict["Decode"])); err != nil {
		return nil, err
	} else if decodeArray != nil {
		expectedLen := 2 * cs.Channels()
		if len(decodeArray) != expectedLen {
			// ignore malformed decode array
			decodeArray = nil
		} else {
			decode := make([]float64, len(decodeArray))
			for i, val := range decodeArray {
				num, err := pdf.GetNumber(x.R, val)
				if err != nil {
					// ignore malformed decode array
					decode = nil
					break
				}
				decode[i] = float64(num)
			}
			thumb.Decode = decode
		}
	}

	// set up WriteData function
	thumb.WriteData = func(w io.Writer) error {
		r, err := pdf.GetStreamReader(x.R, stm)
		if err != nil {
			return err
		}
		defer r.Close()

		_, err = io.Copy(w, r)
		return err
	}

	return thumb, nil
}

// Embed converts the thumbnail to a PDF object.
func (t *Thumbnail) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := t.check(rm.Out); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Width":            pdf.Integer(t.Width),
		"Height":           pdf.Integer(t.Height),
		"BitsPerComponent": pdf.Integer(t.BitsPerComponent),
	}

	// embed color space
	csObj, _, err := pdf.ResourceManagerEmbed(rm, t.ColorSpace)
	if err != nil {
		return nil, zero, err
	}
	dict["ColorSpace"] = csObj

	// add decode array if present
	if t.Decode != nil {
		decode := make(pdf.Array, len(t.Decode))
		for i, val := range t.Decode {
			decode[i] = pdf.Number(val)
		}
		dict["Decode"] = decode
	}

	// create the stream
	ref := rm.Out.Alloc()
	stm, err := rm.Out.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, zero, err
	}

	err = t.WriteData(stm)
	if err != nil {
		stm.Close()
		return nil, zero, err
	}

	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// Bounds returns the dimensions of the thumbnail.
func (t *Thumbnail) Bounds() Rectangle {
	return Rectangle{
		XMin: 0,
		YMin: 0,
		XMax: t.Width,
		YMax: t.Height,
	}
}

// Equal reports whether t and other represent the same thumbnail image.
// Two thumbnails are considered equal if they have the same dimensions,
// color space, bits per component, decode array, and produce identical data.
func (t *Thumbnail) Equal(other *Thumbnail) bool {
	if t == nil || other == nil {
		return t == other
	}

	// compare structural fields
	if t.Width != other.Width || t.Height != other.Height {
		return false
	}
	if t.BitsPerComponent != other.BitsPerComponent {
		return false
	}

	// compare color spaces
	if (t.ColorSpace == nil) != (other.ColorSpace == nil) {
		return false
	}
	if t.ColorSpace != nil {
		if !color.SpacesEqual(t.ColorSpace, other.ColorSpace) {
			return false
		}
	}

	// compare decode arrays
	if len(t.Decode) != len(other.Decode) {
		return false
	}
	for i := range t.Decode {
		if t.Decode[i] != other.Decode[i] {
			return false
		}
	}

	// compare data
	if (t.WriteData == nil) != (other.WriteData == nil) {
		return false
	}
	if t.WriteData != nil {
		var buf1, buf2 bytes.Buffer
		if err := t.WriteData(&buf1); err != nil {
			return false
		}
		if err := other.WriteData(&buf2); err != nil {
			return false
		}
		if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
			return false
		}
	}

	return true
}

// Subtype returns the PDF XObject subtype for thumbnails.
func (t *Thumbnail) Subtype() pdf.Name {
	return pdf.Name("Image")
}

func (t *Thumbnail) check(out *pdf.Writer) error {
	if t.Width <= 0 {
		return fmt.Errorf("invalid thumbnail width %d", t.Width)
	}
	if t.Height <= 0 {
		return fmt.Errorf("invalid thumbnail height %d", t.Height)
	}
	if t.WriteData == nil {
		return errors.New("WriteData function cannot be nil")
	}

	switch t.BitsPerComponent {
	case 1, 2, 4, 8:
		// pass
	case 16:
		if err := pdf.CheckVersion(out, "16 bits per component", pdf.V1_5); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid BitsPerComponent %d", t.BitsPerComponent)
	}

	if t.ColorSpace == nil {
		return errors.New("missing color space")
	}
	if !isValidThumbnailColorSpace(t.ColorSpace) {
		return errors.New("invalid thumbnail color space")
	}

	if t.Decode != nil {
		expectedLen := 2 * t.ColorSpace.Channels()
		if len(t.Decode) != expectedLen {
			return fmt.Errorf("wrong Decode length: expected %d, got %d", expectedLen, len(t.Decode))
		}
	}

	return nil
}

// isValidThumbnailColorSpace checks if a color space is valid for thumbnails.
func isValidThumbnailColorSpace(cs color.Space) bool {
	family := cs.Family()
	switch family {
	case color.FamilyDeviceGray, color.FamilyDeviceRGB:
		return true
	case color.FamilyIndexed:
		// check if base color space is DeviceGray or DeviceRGB
		if indexed, ok := cs.(*color.SpaceIndexed); ok {
			baseFamily := indexed.Base.Family()
			return baseFamily == color.FamilyDeviceGray || baseFamily == color.FamilyDeviceRGB
		}
	}
	return false
}

// isValidBitsPerComponent checks if a BitsPerComponent value is valid.
func isValidBitsPerComponent(bpc int) bool {
	switch bpc {
	case 1, 2, 4, 8, 16:
		return true
	default:
		return false
	}
}
