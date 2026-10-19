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
	"errors"
	"image"
	"math"

	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/filter/jpx"
	"seehuhn.de/go/pdf/internal/limits"
)

// jpxSamples describes how the colour samples of a decoded JPEG 2000
// image map to a PDF colour space.
type jpxSamples struct {
	cs color.Space

	// planes holds one plane per channel of cs; when nil, the samples
	// are looked up in entries (or values), using the indices in index
	planes  [][]int32
	index   []int32
	entries [][]int32

	// values, if non-nil, holds the colour values of the palette
	// entries, already mapped to cs; decode and convert are not used
	values [][]float64

	bits   int       // depth of the samples
	decode []float64 // maps samples to colour values, 2 entries per channel

	// convert, if non-nil, rewrites the decoded values of a pixel in
	// place as component values of cs
	convert func([]float64)

	// direct is set when the samples are colour values taken straight
	// from the codestream, rather than palette indices or entries
	direct bool
}

// decodeJPX decodes the JPEG 2000 data of a JPXDecode image, discarding
// reduce resolution levels, and determines how its samples map to the
// colour space of the image.
func (d *Dict) decodeJPX(reduce int) (*jpx.Image, *jpxSamples, error) {
	raw, err := d.Data.Pixels()
	if err != nil {
		return nil, nil, err
	}
	budget, err := jpx.NewBudget(raw)
	if err != nil {
		return nil, nil, err
	}
	im, err := jpx.Decode(raw, reduce, d.ColorSpace == nil, budget)
	if err != nil {
		return nil, nil, err
	}

	var s *jpxSamples
	if d.ColorSpace != nil {
		s, err = jpxExplicit(im, d.ColorSpace, d.Decode)
	} else {
		s, err = jpxDerived(im)
	}
	if err != nil {
		return nil, nil, err
	}
	return im, s, nil
}

// loadJPX decodes the pixel data of a JPXDecode image, discarding
// reduce resolution levels (clamped to the levels available).
//
// The image size is taken from the JPEG 2000 data.  The colour space is
// d.ColorSpace if set, and otherwise derived from the colour
// specification of the JPEG 2000 data.
func (d *Dict) loadJPX(reduce int) (*Data, error) {
	im, s, err := d.decodeJPX(reduce)
	if err != nil {
		return nil, err
	}

	width, height := im.Width, im.Height
	ncomp := s.cs.Channels()
	useAlpha := d.SMaskInData != 0 && im.Opacity != nil
	nPlanes := ncomp
	if useAlpha {
		nPlanes++
	}
	if limits.ImageDecodedExceedsLimit(width, height, nPlanes) {
		return nil, errors.New("image decoded data exceeds size limit")
	}

	var alpha []float32
	var maxA float64
	if useAlpha {
		maxA = float64(int(1)<<im.OpacityBits - 1)
		alpha = make([]float32, width*height)
		for i, v := range im.Opacity {
			alpha[i] = float32(float64(v) / maxA)
		}
	}
	_, isIndexed := s.cs.(*color.SpaceIndexed)
	premultiplied := useAlpha && (d.SMaskInData == 2 || im.Premultiplied) &&
		s.direct && !isIndexed

	maxV := float64(int(1)<<s.bits - 1)
	pix := make([]float32, width*height*ncomp)
	vals := make([]float64, ncomp)
	for i := range width * height {
		out := pix[i*ncomp : (i+1)*ncomp]
		if s.values != nil {
			for c, v := range s.values[s.index[i]] {
				out[c] = float32(v)
			}
			continue
		}

		var a float64
		if premultiplied {
			a = float64(im.Opacity[i]) / maxA
		}
		for c := range ncomp {
			var v int32
			if s.planes != nil {
				v = s.planes[c][i]
			} else {
				v = s.entries[s.index[i]][c]
			}
			x := float64(v) / maxV
			if a > 0 {
				x = min(x/a, 1)
			}
			lo, hi := s.decode[2*c], s.decode[2*c+1]
			vals[c] = lo + x*(hi-lo)
		}
		if s.convert != nil {
			s.convert(vals)
		}
		for c, v := range vals {
			out[c] = float32(v)
		}
	}

	var keyMask *image.Alpha
	if len(d.MaskColors) == 2*ncomp {
		keyMask = jpxColorKey(s, width, height, d.MaskColors)
	}

	return &Data{
		Pix:     pix,
		Stride:  width * ncomp,
		Rect:    image.Rect(0, 0, width, height),
		CS:      s.cs,
		NComp:   ncomp,
		Alpha:   alpha,
		KeyMask: keyMask,
	}, nil
}

// decodeJPXMask decodes the JPEG 2000 data of a stencil mask or a soft
// mask.  The data must have a single colour channel whose samples are
// used directly, without a palette.
func decodeJPXMask(raw []byte) (*jpx.Image, error) {
	budget, err := jpx.NewBudget(raw)
	if err != nil {
		return nil, err
	}
	im, err := jpx.Decode(raw, 0, false, budget)
	if err != nil {
		return nil, err
	}
	if len(im.Color) != 1 || im.Palette != nil {
		return nil, errors.New("JPEG 2000 mask data needs a single colour channel")
	}
	return im, nil
}

// jpxExplicit interprets the samples of im in the colour space cs given
// by the image dictionary.  A palette in the JPEG 2000 data is applied
// when this gives the right number of channels, except for an Indexed
// colour space, whose lookup takes the palette indices.
func jpxExplicit(im *jpx.Image, cs color.Space, decode []float64) (*jpxSamples, error) {
	n := cs.Channels()
	if n == 0 {
		return nil, errors.New("pattern color spaces are not valid for images")
	}
	_, isIndexed := cs.(*color.SpaceIndexed)

	s := &jpxSamples{cs: cs}
	switch {
	case im.Palette != nil && !isIndexed && len(im.Palette.Entries[0]) == n:
		s.index = im.Color[0]
		s.entries = im.Palette.Entries
		s.bits = im.Palette.Bits
	case len(im.Color) == n:
		// the dictionary declares the planes to be colour samples
		s.planes = im.Color
		s.bits = im.BitsPerComponent
		s.direct = true
	default:
		return nil, errors.New("JPEG 2000 channels do not match ColorSpace")
	}

	if len(decode) >= 2*n {
		s.decode = decode[:2*n]
	} else {
		s.decode = DefaultDecode(cs, s.bits)
	}
	return s, nil
}

// jpxDerived determines the colour space of a JPEG 2000 image from its
// colour specification.  A palette of up to 256 entries becomes an
// Indexed colour space; larger palettes, which PDF cannot represent, are
// applied.
func jpxDerived(im *jpx.Image) (*jpxSamples, error) {
	spec := im.ColorSpace
	base := spec.Space
	if base == nil {
		return nil, errors.New("unsupported number of JPEG 2000 colour channels")
	}

	pal := im.Palette
	if pal == nil {
		return &jpxSamples{
			cs:      base,
			planes:  im.Color,
			bits:    im.BitsPerComponent,
			decode:  spec.Decode,
			convert: spec.Convert,
			direct:  true,
		}, nil
	}

	values := make([][]float64, len(pal.Entries))
	maxV := float64(int(1)<<pal.Bits - 1)
	for i, e := range pal.Entries {
		vals := make([]float64, len(e))
		for c, v := range e {
			lo, hi := spec.Decode[2*c], spec.Decode[2*c+1]
			vals[c] = lo + float64(v)/maxV*(hi-lo)
		}
		if spec.Convert != nil {
			spec.Convert(vals)
		}
		values[i] = vals
	}

	if len(pal.Entries) > 256 {
		return &jpxSamples{
			cs:      base,
			index:   im.Color[0],
			entries: pal.Entries,
			values:  values,
			bits:    pal.Bits,
		}, nil
	}

	if lab, ok := base.(*color.SpaceLab); ok && spec.Convert != nil {
		// The lookup table of an Indexed space has 8 bits per component,
		// which over the wide a* and b* ranges of the extended encodings
		// would be far too coarse.  A base space fitted to the palette
		// keeps the steps small.
		var err error
		base, err = fitLab(lab, values)
		if err != nil {
			return nil, err
		}
	}
	colors := make([]color.Color, len(values))
	for i, vals := range values {
		colors[i] = color.FromValues(base, vals, nil)
	}
	cs, err := color.Indexed(colors)
	if err != nil {
		return nil, err
	}
	return &jpxSamples{
		cs:     cs,
		planes: im.Color,
		bits:   im.BitsPerComponent,
		decode: DefaultDecode(cs, im.BitsPerComponent),
	}, nil
}

// fitLab returns a Lab colour space with the white point of lab whose a*
// and b* ranges are the smallest whole-number ranges enclosing the given
// colours of lab.
func fitLab(lab *color.SpaceLab, colors [][]float64) (*color.SpaceLab, error) {
	ranges := []float64{math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)}
	for _, v := range colors {
		for i := range 2 {
			ranges[2*i] = min(ranges[2*i], math.Floor(v[i+1]))
			ranges[2*i+1] = max(ranges[2*i+1], math.Ceil(v[i+1]))
		}
	}
	for i := 0; i < 4; i += 2 {
		if ranges[i] == ranges[i+1] {
			ranges[i+1]++
		}
	}
	return color.Lab(lab.WhitePoint[:], lab.BlackPoint[:], ranges)
}
