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
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/opaque"
)

// PDF 2.0 sections: 8.9.5 12.3.4

// Thumbnail represents a thumbnail image for a PDF page.
// Thumbnail images are miniature representations of page contents that can be
// displayed for navigation purposes.
//
// Thumbnail implements the graphics.Image interface.
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

	// Source writes the body of the thumbnail stream when the Thumbnail is
	// embedded.  For raw pixel data use
	// [seehuhn.de/go/pdf/graphics/image.FlateSource]; pre-encoded formats
	// provide their own Source implementations.
	Source graphics.ImageData
}

// ExtractThumbnail reads a thumbnail image from a PDF object.
func ExtractThumbnail(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (*Thumbnail, error) {
	stm, err := x.GetStream(path, obj)
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
	if subtypeName, err := pdf.Optional(x.GetName(path, dict["Subtype"])); err != nil {
		return nil, err
	} else if subtypeName != "Image" && subtypeName != "" {
		return nil, pdf.Errorf("invalid Subtype %q for thumbnail", subtypeName)
	}

	thumb := &Thumbnail{}

	// width (required)
	width, err := x.GetInteger(path, dict["Width"])
	if err != nil {
		return nil, err
	} else if width <= 0 {
		return nil, pdf.Error("invalid thumbnail width")
	}
	thumb.Width = int(width)

	// height (required)
	height, err := x.GetInteger(path, dict["Height"])
	if err != nil {
		return nil, err
	} else if height <= 0 {
		return nil, pdf.Error("invalid thumbnail height")
	}
	thumb.Height = int(height)

	// color space (required)
	cs, err := color.ExtractSpace(x, path, dict["ColorSpace"], false)
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
	bpc, err := x.GetInteger(path, dict["BitsPerComponent"])
	if err != nil {
		return nil, err
	} else if !isValidBitsPerComponent(int(bpc)) {
		return nil, pdf.Error("invalid BitsPerComponent value")
	}
	thumb.BitsPerComponent = int(bpc)

	// decode array (optional)
	if decodeArray, err := pdf.Optional(x.GetArray(path, dict["Decode"])); err != nil {
		return nil, err
	} else if decodeArray != nil && len(decodeArray) == 2*cs.Channels() {
		decode := make([]float64, len(decodeArray))
		for i, val := range decodeArray {
			num, err := x.GetNumber(path, val)
			if err != nil {
				// ignore malformed decode array
				decode = nil
				break
			}
			decode[i] = num
		}
		thumb.Decode = decode
	}

	thumb.Source = &thumbnailStreamData{inner: opaque.ExtractStream(x, stm)}

	return thumb, nil
}

// NewRawSource returns a [Source] that writes raw pixel bytes via the
// given callback, compressed with Flate when embedded.  This is a
// convenience constructor for callers that cannot import
// [seehuhn.de/go/pdf/graphics/image.FlateSource] because of a
// dependency cycle (notably [seehuhn.de/go/pdf/file]).  Package
// `image` users should prefer `image.FlateSource` directly.
func NewRawSource(writeData func(io.Writer) error) graphics.ImageData {
	return &readThumbnailSource{read: writeData}
}

// readThumbnailSource is the Source constructed by [ExtractThumbnail] to
// re-emit decoded thumbnail bytes.  It compresses the bytes with plain
// Flate, matching the default encoding used by [Thumbnail.Embed].
type readThumbnailSource struct {
	read func(io.Writer) error
}

func (s *readThumbnailSource) Pixels() ([]byte, error) {
	var buf bytes.Buffer
	if err := s.read(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *readThumbnailSource) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	w, err := rm.Out().OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	if err := s.read(w); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// thumbnailStreamData lazily reads a thumbnail from a PDF stream,
// delegating to [opaque.Stream] for both decoding (Pixels) and
// verbatim cross-file re-emission (WriteStream).
type thumbnailStreamData struct {
	inner *opaque.Stream
}

func (s *thumbnailStreamData) Pixels() ([]byte, error) {
	r, err := s.inner.Reader()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (s *thumbnailStreamData) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	return s.inner.WriteAt(rm, ref, dict)
}

var _ graphics.ImageData = (*thumbnailStreamData)(nil)

// Embed converts the thumbnail to a PDF object.
func (t *Thumbnail) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := t.check(e.Out()); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Width":            pdf.Integer(t.Width),
		"Height":           pdf.Integer(t.Height),
		"BitsPerComponent": pdf.Integer(t.BitsPerComponent),
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("XObject")
		dict["Subtype"] = pdf.Name("Image")
	}

	// embed color space
	csObj, err := e.Embed(t.ColorSpace)
	if err != nil {
		return nil, err
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

	ref := e.Alloc()
	if err := t.Source.WriteStream(e, ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// Bounds returns the dimensions of the thumbnail.
func (t *Thumbnail) Bounds() rect.IntRect {
	return rect.IntRect{
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

	// compare raw pixel data
	if (t.Source == nil) != (other.Source == nil) {
		return false
	}
	if t.Source != nil {
		data1, err1 := t.Source.Pixels()
		data2, err2 := other.Source.Pixels()
		if err1 != nil || err2 != nil {
			return false
		}
		if !bytes.Equal(data1, data2) {
			return false
		}
	}

	return true
}

// Subtype returns the PDF XObject subtype for thumbnails.
func (t *Thumbnail) Subtype() pdf.Name {
	return pdf.Name("Image")
}

// ResourceName returns the empty string: thumbnails are attached to pages
// via /Thumb, not referenced via a content stream /XObject key.  See
// [graphics.XObject.ResourceName].
func (t *Thumbnail) ResourceName() pdf.Name {
	return ""
}

func (t *Thumbnail) check(out *pdf.Writer) error {
	if t.Width <= 0 {
		return fmt.Errorf("invalid thumbnail width %d", t.Width)
	}
	if t.Height <= 0 {
		return fmt.Errorf("invalid thumbnail height %d", t.Height)
	}
	if t.Source == nil {
		return errors.New("source cannot be nil")
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

// Detach loads the thumbnail data into memory, allowing the source file to be
// closed and surfacing any errors early.
//
// After a successful call the Source is replaced with a detached copy.
func (t *Thumbnail) Detach() error {
	data, err := t.Source.Pixels()
	if err != nil {
		return err
	}
	t.Source = &readThumbnailSource{
		read: func(w io.Writer) error {
			_, err := w.Write(data)
			return err
		},
	}
	return nil
}
