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

// Package jpx adapts the JPEG 2000 decoder to the needs of PDF images
// using the JPXDecode filter (PDF spec §7.4.9).
//
// The package turns a decoded JPEG 2000 image into unsigned samples at a
// common bit depth, separates colour and opacity channels, and selects
// the colour specification a PDF processor should use.  Each usable
// specification is described as a PDF colour space, a Decode array and,
// where the mapping is not linear, a conversion of the decoded values.
// Specifications PDF cannot use, such as unsupported ICC profiles, are
// skipped in favour of the next one.
package jpx

import (
	"bytes"
	"errors"
	"slices"

	"seehuhn.de/go/jpx"
	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/limits"
)

// maxDepth is the largest sample depth of a PDF image.  Deeper JPEG 2000
// samples are scaled down to this depth.
const maxDepth = 16

// Image holds the samples of a JPEG 2000 image in the form a PDF image
// needs.
type Image struct {
	// Width and Height give the image size in pixels.  For a reduced
	// decode, this is the reduced size.
	Width, Height int

	// BitsPerComponent is the common depth of the samples in Color,
	// between 1 and 16.  When Palette is non-nil, it is the depth of the
	// palette indices.
	BitsPerComponent int

	// Color holds the ordinary colour channels in colour order, each with
	// Width*Height unsigned samples in row-major order.  When Palette is
	// non-nil, Color holds a single channel of palette indices, each in
	// the range 0 to len(Palette.Entries)-1.
	Color [][]int32

	// Opacity is the opacity channel, or nil if the image has none.
	// The samples are unsigned.
	Opacity []int32

	// OpacityBits is the depth of the samples in Opacity.
	OpacityBits int

	// Premultiplied reports whether the colour samples have been
	// multiplied by the opacity.
	Premultiplied bool

	// ColorSpace describes the colour channels: the best usable colour
	// specification of the image, or the device colour space with the
	// number of colour channels if there is none or the specifications
	// were not used.  For a palette image, it describes the palette
	// entries.
	ColorSpace ColorSpace

	// Palette is the palette of the image, or nil.
	Palette *Palette
}

// ColorSpace describes how the colour samples of an [Image] map to a PDF
// colour space.
type ColorSpace struct {
	// Space is the PDF colour space of the samples.  It is nil only for
	// an image without usable colour specification whose number of
	// colour channels matches no device colour space.
	Space color.Space

	// Decode maps samples normalised to [0, 1] to the values passed to
	// Convert or, when Convert is nil, to the components of Space.  It
	// has two entries per colour channel.
	Decode []float64

	// Convert, if non-nil, rewrites decoded values in place as component
	// values of Space.  It is nil when the mapping is linear.
	Convert func(v []float64)
}

// Palette holds the colours of a palette image.
type Palette struct {
	// Bits is the depth of the palette entries, between 1 and 16.
	Bits int

	// Entries[i][c] is colour component c of palette entry i, an
	// unsigned value of depth Bits.
	Entries [][]int32
}

// enumerated colour spaces
const (
	enumCMYK  = 12
	enumLab   = 14
	enumSRGB  = 16
	enumGray  = 17
	enumSYCC  = 18
	enumESRGB = 20
	enumROMM  = 21
	enumESYCC = 24
)

// NewBudget returns the budget for decoding data as a PDF image.  The
// budget is sized by [limits.JPXBudget] from the image size, components,
// channels and tiles which the codestream header declares.  An image
// with more than [limits.MaxImagePixels] pixels is rejected.
func NewBudget(data []byte) (*membudget.Budget, error) {
	hb := membudget.New(limits.StreamBudget(int64(len(data))))
	h, err := jpx.ReadHeader(bytes.NewReader(data), hb)
	if err != nil {
		return nil, err
	}
	if limits.ImagePixelsExceedLimit(h.Width, h.Height) {
		return nil, errors.New("JPEG 2000 image pixel count exceeds limit")
	}
	depths := make([]int, len(h.Components))
	for i, c := range h.Components {
		depths[i] = c.Depth
	}
	n := limits.JPXBudget(int64(len(data)), h.Width, h.Height, depths, len(h.Channels), h.Tiles)
	return membudget.New(n), nil
}

// Decode reads a JPEG 2000 image (JP2/JPX file or bare codestream) and
// returns its samples in the form a PDF image needs.  All memory
// proportional to the image size, and the decoding work, are charged to
// budget, which must not be nil.
//
// If reduce is positive, that many resolution levels are discarded, so
// that the result has roughly 1/2^reduce of the width and height of the
// full image.  The value is clamped to the number of levels the
// codestream offers.  Width and Height of the result give the reduced
// size.
//
// If useColorSpec is true, the colour specifications of the file
// determine ColorSpace.  Otherwise the specifications are ignored,
// ColorSpace gives the device colour space with the number of colour
// channels, and the samples are returned as stored, as needed for an
// image dictionary with a ColorSpace entry.
//
// Truncated or partially corrupt data produce the image decoded so far.
func Decode(data []byte, reduce int, useColorSpec bool, budget *membudget.Budget) (*Image, error) {
	if reduce > 0 {
		// jpx.Decode rejects a reduction beyond the available levels
		h, err := jpx.ReadHeader(bytes.NewReader(data), budget)
		if err != nil {
			return nil, err
		}
		reduce = min(reduce, h.Resolutions-1)
	}
	img, err := jpx.Decode(bytes.NewReader(data), &jpx.Options{
		Budget: budget,
		Reduce: max(reduce, 0),
	})
	if err != nil {
		return nil, err
	}
	img, err = jpx.Upsample(img, budget)
	if err != nil {
		return nil, err
	}

	colour, opacity := classify(img.Channels)
	if len(colour) == 0 {
		return nil, errors.New("JPEG 2000 image has no colour channels")
	}
	if img.Palette != nil && !keepPalette(img.Palette, colour, opacity) {
		img, err = jpx.ApplyPalette(img, budget)
		if err != nil {
			return nil, err
		}
		colour, opacity = classify(img.Channels)
	}

	// after Upsample all planes have the (possibly reduced) image size
	size := img.Planes[colour[0].Component]
	res := &Image{
		Width:  size.Width,
		Height: size.Height,
	}

	depths := make([]int, len(colour))
	for i, ch := range colour {
		depths[i] = channelDepth(&img.Header, ch)
	}
	if useColorSpec {
		res.ColorSpace = chooseColorSpec(img.ColorSpecs, depths)
	} else {
		res.ColorSpace = deviceSpace(len(colour))
	}

	if img.Palette != nil {
		// The index samples are clamped in place, which is safe since
		// img is owned here.  As in [jpx.ApplyPalette], a truncated
		// sample gives the index through its retained bits.
		pal := img.Palette
		comp := img.Components[colour[0].Component]
		idx := img.Planes[colour[0].Component].Pix
		last := int64(len(pal.Entries) - 1)
		for i, v := range idx {
			idx[i] = int32(min(max(int64(v)<<comp.Truncation, 0), last))
		}
		res.BitsPerComponent = min(comp.Depth, maxDepth)
		res.Color = [][]int32{idx}

		res.Palette, err = convertPalette(pal, colour, budget)
		if err != nil {
			return nil, err
		}
	} else {
		depth := 0
		for _, ch := range colour {
			depth = max(depth, img.Components[ch.Component].Depth)
		}
		depth = min(depth, maxDepth)
		planes, err := unsigned(img, colour, depth, budget)
		if err != nil {
			return nil, err
		}
		res.BitsPerComponent = depth
		res.Color = make([][]int32, len(planes.Planes))
		for i, p := range planes.Planes {
			res.Color[i] = p.Pix
		}
	}

	if opacity != nil {
		depth := min(img.Components[opacity.Component].Depth, maxDepth)
		planes, err := unsigned(img, []jpx.Channel{*opacity}, depth, budget)
		if err != nil {
			return nil, err
		}
		res.Opacity = planes.Planes[0].Pix
		res.OpacityBits = depth
		res.Premultiplied = opacity.Kind == jpx.ChannelPremultipliedOpacity
	}

	return res, nil
}

// classify returns the colour channels in colour order and the opacity
// channel, if any.
//
// Colour channels are ordered by their association, which gives their
// position in the colour space, when the associations number the
// channels 1, ..., n; otherwise they are taken in channel order.
//
// PDF allows a single opacity channel which applies to all colour
// channels.  The first such channel is used; failing that, a sole
// opacity channel with some other association is used.  Further opacity
// channels and channels of unknown type are ignored.
func classify(channels []jpx.Channel) (colour []jpx.Channel, opacity *jpx.Channel) {
	var others []jpx.Channel
	for _, ch := range channels {
		switch ch.Kind {
		case jpx.ChannelColor:
			colour = append(colour, ch)
		case jpx.ChannelOpacity, jpx.ChannelPremultipliedOpacity:
			if ch.Assoc == 0 && opacity == nil {
				opacity = &ch
			} else {
				others = append(others, ch)
			}
		}
	}
	if opacity == nil && len(others) == 1 {
		opacity = &others[0]
	}

	byAssoc := slices.Clone(colour)
	slices.SortStableFunc(byAssoc, func(a, b jpx.Channel) int { return a.Assoc - b.Assoc })
	for i, ch := range byAssoc {
		if ch.Assoc != i+1 {
			return colour, opacity
		}
	}
	return byAssoc, opacity
}

// keepPalette reports whether the palette can be passed on to the caller
// unapplied: all colour channels must read palette columns of the same
// component, and the opacity channel must use another component directly.
func keepPalette(pal *jpx.Palette, colour []jpx.Channel, opacity *jpx.Channel) bool {
	comp := colour[0].Component
	for _, ch := range colour {
		if ch.PaletteColumn < 0 || ch.Component != comp {
			return false
		}
	}
	if opacity != nil && (opacity.PaletteColumn >= 0 || opacity.Component == comp) {
		return false
	}
	return len(pal.Entries) > 0
}

// KeepsPalette reports whether [Decode] returns the image described by h
// as palette indices with a [Palette], rather than applying the palette to
// the samples.  This is the case when all colour channels read columns of
// the palette from the same component, and no opacity channel does.
func KeepsPalette(h *jpx.Header) bool {
	if h.Palette == nil {
		return false
	}
	colour, opacity := classify(h.Channels)
	if len(colour) == 0 {
		return false
	}
	return keepPalette(h.Palette, colour, opacity)
}

// convertPalette returns the palette columns used by the colour channels,
// in colour order, as unsigned values at a common depth.
func convertPalette(pal *jpx.Palette, colour []jpx.Channel, budget *membudget.Budget) (*Palette, error) {
	n := len(pal.Entries)
	if err := budget.ChargeN(n*len(colour), 4); err != nil {
		return nil, err
	}
	tmp := &jpx.Image{
		Width:  n,
		Height: 1,
		Planes: make([]jpx.Plane, len(colour)),
	}
	tmp.Components = make([]jpx.Component, len(colour))
	depth := 0
	for c, ch := range colour {
		col := pal.Columns[ch.PaletteColumn]
		depth = max(depth, col.Depth)
		tmp.Components[c] = jpx.Component{
			Depth: col.Depth, Signed: col.Signed, Truncation: col.Truncation,
			DX: 1, DY: 1, Width: n, Height: 1,
		}
		pix := make([]int32, n)
		for i, e := range pal.Entries {
			pix[i] = e[ch.PaletteColumn]
		}
		tmp.Planes[c] = jpx.Plane{Width: n, Height: 1, Pix: pix}
	}
	depth = min(depth, maxDepth)
	out, err := jpx.ToUnsigned(tmp, depth, budget)
	if err != nil {
		return nil, err
	}

	if err := budget.ChargeN(n, 24+4*len(colour)); err != nil {
		return nil, err
	}
	res := &Palette{Bits: depth, Entries: make([][]int32, n)}
	for i := range res.Entries {
		e := make([]int32, len(colour))
		for c := range e {
			e[c] = out.Planes[c].Pix[i]
		}
		res.Entries[i] = e
	}
	return res, nil
}

// unsigned returns the planes read by the given channels, converted to
// unsigned samples of the given depth.
func unsigned(img *jpx.Image, channels []jpx.Channel, depth int, budget *membudget.Budget) (*jpx.Image, error) {
	sub := &jpx.Image{
		Header: img.Header,
		Reduce: img.Reduce,
		Planes: make([]jpx.Plane, len(channels)),
	}
	sub.Components = make([]jpx.Component, len(channels))
	for i, ch := range channels {
		sub.Components[i] = img.Components[ch.Component]
		sub.Planes[i] = img.Planes[ch.Component]
	}
	return jpx.ToUnsigned(sub, depth, budget)
}

// ColorChannels returns the number of colour channels of a JPEG 2000
// image and their bit depths, in colour order.  For a channel which reads
// a palette column, the depth is that of the column.
func ColorChannels(h *jpx.Header) (n int, depths []int) {
	colour, _ := classify(h.Channels)
	depths = make([]int, len(colour))
	for i, ch := range colour {
		depths[i] = channelDepth(h, ch)
	}
	return len(colour), depths
}

// channelDepth returns the bit depth of the samples a channel reads.
func channelDepth(h *jpx.Header, ch jpx.Channel) int {
	if ch.PaletteColumn >= 0 {
		return h.Palette.Columns[ch.PaletteColumn].Depth
	}
	return h.Components[ch.Component].Depth
}

// Usable reports whether a colour specification can describe colour
// channels with the given bit depths, as returned by [ColorChannels].
// [Decode] uses exactly the specifications for which Usable returns true.
func Usable(s jpx.ColorSpec, depths []int) bool {
	_, ok := resolve(s, depths)
	return ok
}

// chooseColorSpec selects the colour specification to use for colour
// channels with the given bit depths.  Without a usable specification,
// the device colour space with the number of colour channels is returned.
//
// Among the usable specifications, the one with the highest precedence
// wins, and among these the one with the best approximation.  An
// approximation value of 0 ("not specified") ranks behind the specified
// values 1 (accurate) to 4 (poor).  Remaining ties go to the
// specification which comes first in the file.
func chooseColorSpec(specs []jpx.ColorSpec, depths []int) ColorSpace {
	best := -1
	var bestCS ColorSpace
	for i, s := range specs {
		// only resolve specifications which would win
		if best >= 0 && !better(s, specs[best]) {
			continue
		}
		cs, ok := resolve(s, depths)
		if !ok {
			continue
		}
		best, bestCS = i, cs
	}
	if best < 0 {
		return deviceSpace(len(depths))
	}
	return bestCS
}

// deviceSpace returns the device colour space with n channels, or a
// ColorSpace with nil Space if there is none.
func deviceSpace(n int) ColorSpace {
	switch n {
	case 1:
		return linear(color.SpaceDeviceGray)
	case 3:
		return linear(color.SpaceDeviceRGB)
	case 4:
		return linear(color.SpaceDeviceCMYK)
	}
	return ColorSpace{}
}

// linear returns the description of samples which map linearly onto the
// component ranges [0, 1] of cs.
func linear(cs color.Space) ColorSpace {
	n := cs.Channels()
	decode := make([]float64, 2*n)
	for i := range n {
		decode[2*i+1] = 1
	}
	return ColorSpace{Space: cs, Decode: decode}
}

// resolve describes a colour specification for colour channels with the
// given bit depths.  The second return value is false if the
// specification is not supported or does not fit the channels.
func resolve(s jpx.ColorSpec, depths []int) (ColorSpace, bool) {
	n := len(depths)
	switch s.Method {
	case 1:
		switch s.Enumerated {
		case enumGray:
			return linear(color.SpaceDeviceGray), n == 1
		case enumSRGB:
			return linear(color.SpaceDeviceRGB), n == 3
		case enumCMYK:
			return linear(color.SpaceDeviceCMYK), n == 4
		case enumROMM:
			return linear(color.SpaceROMMRGB), n == 3
		case enumLab:
			if n != 3 {
				return ColorSpace{}, false
			}
			return resolveLab(s, depths)
		case enumSYCC:
			if n != 3 {
				return ColorSpace{}, false
			}
			return resolveSYCC(depths), true
		case enumESRGB:
			if n != 3 {
				return ColorSpace{}, false
			}
			return resolveESRGB(depths), true
		case enumESYCC:
			if n != 3 {
				return ColorSpace{}, false
			}
			return resolveESYCC(depths), true
		}
	case 2, 3:
		cs, err := color.ICCBased(s.ICC, nil)
		if err != nil || cs.Channels() != n {
			return ColorSpace{}, false
		}
		return ColorSpace{Space: cs, Decode: slices.Clone(cs.Ranges[:2*n])}, true
	}
	return ColorSpace{}, false
}

// better reports whether colour specification a is preferable to b.
func better(a, b jpx.ColorSpec) bool {
	if a.Precedence != b.Precedence {
		return a.Precedence > b.Precedence
	}
	return approxRank(a.Approx) < approxRank(b.Approx)
}

func approxRank(approx int) int {
	if approx == 0 {
		return 1 << 16
	}
	return approx
}
