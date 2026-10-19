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

package image

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"

	"seehuhn.de/go/jpx"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	pdfjpx "seehuhn.de/go/pdf/internal/filter/jpx"
)

// JPXSource embeds a JPEG 2000 image, given as a JP2 or JPX file, as the
// data of an image dictionary.  The data is written unchanged, using the
// JPXDecode filter.  It implements [graphics.ImageData].
//
// The image dictionary must have the size of the JPEG 2000 image and
// BitsPerComponent 0.  If the dictionary has a ColorSpace, the JPEG 2000
// image must have one colour channel per colour space component;
// otherwise the JPEG 2000 file must contain a colour specification which
// PDF can use for its colour channels: an enumerated colour space other
// than CIEJab, or an ICC profile, with the matching number of channels.
// If SMaskInData is non-zero, the JPEG 2000 image must have a single
// opacity channel which applies to all colour channels.
//
// A JPXSource can also provide the data of a [Mask] or a [SoftMask].  The
// JPEG 2000 image then has a single colour channel, without palette or
// opacity channel; for a Mask, its samples have one bit.  The ColorSpace
// of a SoftMask, DeviceGray, determines how the samples are interpreted.
type JPXSource struct {
	// Data is the JPEG 2000 file.
	Data []byte
}

// NewJPXSource returns a source which embeds the given JPEG 2000 file.
func NewJPXSource(data []byte) *JPXSource {
	return &JPXSource{Data: data}
}

// header reads the JPEG 2000 header of the data.
func (s *JPXSource) header() (*jpx.Header, error) {
	h, err := jpx.ReadHeader(bytes.NewReader(s.Data), nil)
	if err != nil {
		return nil, err
	}
	if !h.Wrapped {
		return nil, errors.New("JPXSource needs a JP2/JPX file, not a bare codestream")
	}
	return h, nil
}

// CheckGeometry implements [graphics.GeometryChecker].  A channels value
// of 0 indicates an image dictionary without ColorSpace.  A palette
// image provides its index channel, for an Indexed colour space, or the
// palette columns as colour channels.
func (s *JPXSource) CheckGeometry(width, height, channels, bitsPerComponent int) error {
	h, err := s.header()
	if err != nil {
		return err
	}
	if bitsPerComponent != 0 {
		return fmt.Errorf("JPXSource requires BitsPerComponent 0, got %d", bitsPerComponent)
	}
	if h.Width != width || h.Height != height {
		return fmt.Errorf("JPXSource size %dx%d differs from image size %dx%d",
			h.Width, h.Height, width, height)
	}

	if channels == 0 {
		_, depths := pdfjpx.ColorChannels(h)
		usable := func(spec jpx.ColorSpec) bool { return pdfjpx.Usable(spec, depths) }
		if !slices.ContainsFunc(h.ColorSpecs, usable) {
			return errors.New("JPXSource has no colour space usable in PDF")
		}
		return nil
	}
	n, _ := pdfjpx.ColorChannels(h)
	if n != channels && !(pdfjpx.KeepsPalette(h) && channels == 1) {
		return fmt.Errorf("JPXSource has %d colour channels, color space has %d", n, channels)
	}
	return nil
}

// checkOpacity verifies that the data has a single opacity channel which
// applies to all colour channels, as required for a non-zero SMaskInData.
func (s *JPXSource) checkOpacity() error {
	h, err := s.header()
	if err != nil {
		return err
	}
	var opacity []jpx.Channel
	for _, ch := range h.Channels {
		if ch.Kind == jpx.ChannelOpacity || ch.Kind == jpx.ChannelPremultipliedOpacity {
			opacity = append(opacity, ch)
		}
	}
	if len(opacity) != 1 || opacity[0].Assoc != 0 {
		return errors.New("SMaskInData needs one opacity channel for the whole image")
	}
	return nil
}

// checkMask verifies that the data can be used for a stencil mask or a
// soft mask: it must have a single colour channel, used without a
// palette, and no opacity channel.  If depth is non-zero, the samples
// must have this depth.
func (s *JPXSource) checkMask(depth int) error {
	h, err := s.header()
	if err != nil {
		return err
	}
	var colour []jpx.Channel
	for _, ch := range h.Channels {
		switch ch.Kind {
		case jpx.ChannelColor:
			colour = append(colour, ch)
		case jpx.ChannelOpacity, jpx.ChannelPremultipliedOpacity:
			return errors.New("JPXSource for a mask has an opacity channel")
		}
	}
	if len(colour) != 1 || colour[0].PaletteColumn >= 0 {
		return errors.New("JPXSource for a mask needs a single colour channel")
	}
	if d := h.Components[colour[0].Component].Depth; depth != 0 && d != depth {
		return fmt.Errorf("JPXSource for a mask has %d-bit samples, need %d", d, depth)
	}
	return nil
}

// Pixels implements [graphics.ImageData].  It returns Data, the JPEG 2000
// file.
func (s *JPXSource) Pixels() ([]byte, error) {
	return s.Data, nil
}

// IsJPX implements [graphics.ImageData].
func (s *JPXSource) IsJPX() bool { return true }

// WriteStream implements [graphics.ImageData].
func (s *JPXSource) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	name, _, err := pdf.FilterJPX{}.Info(rm.Out().GetMeta().Version)
	if err != nil {
		return err
	}
	if smid, _ := dict["SMaskInData"].(pdf.Integer); smid != 0 {
		if err := s.checkOpacity(); err != nil {
			return err
		}
	}

	// the filter is added to the dictionary since FilterJPX cannot encode
	dict = maps.Clone(dict)
	dict["Filter"] = name
	w, err := rm.Out().OpenStream(ref, dict)
	if err != nil {
		return err
	}
	if _, err := w.Write(s.Data); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

var (
	_ graphics.ImageData       = (*JPXSource)(nil)
	_ graphics.GeometryChecker = (*JPXSource)(nil)
)
