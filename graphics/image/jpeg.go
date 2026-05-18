// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"errors"
	goimage "image"
	"image/draw"
	"image/jpeg"
	"maps"

	"seehuhn.de/go/pdf"
)

// DCTSource writes a Go [image.Image] as JPEG-encoded (DCTDecode) image
// data to a PDF stream.  It implements [graphics.ImageData] and is
// used as the Source of an [image.Dict] to embed a JPEG image XObject.
//
// The source image is converted to NRGBA before encoding.  To embed a
// DCT-encoded image with a particular PDF colour space, set
// [Dict.ColorSpace] on the enclosing [Dict] rather than relying on any
// colour information carried inside the JPEG stream.
type DCTSource struct {
	// Image is the image data to encode.
	Image goimage.Image

	// Options controls the JPEG encoder (quality, etc.).  If nil, the
	// standard library's default options are used.
	Options *jpeg.Options
}

// Pixels returns the raw, uncompressed pixel data.
// The output format matches the image's colour model: 3 bytes per pixel
// for colour images (RGB), 1 byte per pixel for grayscale.
func (s *DCTSource) Pixels() ([]byte, error) {
	if s.Image == nil {
		return nil, errors.New("DCTSource.Image is nil")
	}
	b := s.Image.Bounds()
	width, height := b.Dx(), b.Dy()

	switch s.Image.(type) {
	case *goimage.Gray, *goimage.Gray16:
		buf := make([]byte, 0, width*height)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, _, _, _ := s.Image.At(x, y).RGBA()
				buf = append(buf, byte(r>>8))
			}
		}
		return buf, nil
	default:
		buf := make([]byte, 0, width*height*3)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, _ := s.Image.At(x, y).RGBA()
				buf = append(buf, byte(r>>8), byte(g>>8), byte(bl>>8))
			}
		}
		return buf, nil
	}
}

// IsJPX implements [graphics.ImageData].
func (s *DCTSource) IsJPX() bool { return false }

// WriteStream implements [graphics.ImageData].
func (s *DCTSource) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	if s.Image == nil {
		return errors.New("DCTSource.Image is nil")
	}

	// Convert to NRGBA for deterministic colour handling.
	b := s.Image.Bounds()
	img := goimage.NewNRGBA(b)
	draw.Draw(img, img.Bounds(), s.Image, b.Min, draw.Src)

	dict = maps.Clone(dict)
	dict["Filter"] = pdf.Name("DCTDecode")

	w, err := rm.Out().OpenStream(ref, dict)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(w, img, s.Options); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
