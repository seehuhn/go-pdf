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
	"errors"
	"fmt"
	"image"
	gocol "image/color"
	"io"
	"math"
	"slices"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/limits"
	"seehuhn.de/go/pdf/measure"

	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/opaque"
	"seehuhn.de/go/pdf/optional"
	"seehuhn.de/go/pdf/webcapture"
)

// PDF 2.0 sections: 8.9.5

// Dict represents a PDF image XObject dictionary.
//
// To extract an image from a PDF file, use
// [seehuhn.de/go/pdf/graphics/extract.Image].
type Dict struct {
	// Width is the width of the image in pixels.
	Width int

	// Height is the height of the image in pixels.
	Height int

	// ColorSpace is the color space in which image samples are specified.
	// It can be any type of color space except Pattern.
	//
	// May be nil only for JPXDecode images, where the colour space
	// description lives in the JPEG 2000 codestream.
	ColorSpace color.Space

	// BitsPerComponent is the number of bits used to represent each
	// colour component.  The value must be 1, 2, 4, 8, or (from PDF 1.5)
	// 16.
	//
	// May be 0 only for JPXDecode images, where the bit depth lives in
	// the JPEG 2000 codestream and may differ per channel.
	BitsPerComponent int

	// Decode (optional) is an array of numbers describing how to map pixel
	// component values into the range of values appropriate for the image's
	// color space. The slice must have twice the number of color components in
	// the ColorSpace.
	Decode []float64

	// Data holds the image pixel component values and controls how they are
	// encoded in the PDF stream.  Each pixel consists of one value per colour
	// channel, each BitsPerComponent bits wide, laid out row by row.  Use
	// [FlateSource] for raw pixel data, [DCTSource] for JPEG encoding, or
	// [seehuhn.de/go/pdf/graphics/image/jbig2.Image] for JBIG2.
	Data graphics.ImageData

	// MaskImage (optional) determines which parts of the image are to be
	// painted.
	//
	// Only one of MaskImage or MaskColors may be specified.
	MaskImage *Mask

	// MaskColors (optional) defines color ranges for transparency masking.
	// Contains pairs [min1, max1, min2, max2, ...] for each color component.
	// Each value must be in the range 0 to (2^BitsPerComponent - 1) and
	// represents raw color values before any Decode array processing. Pixels
	// with all components in their respective ranges become transparent.
	//
	// For JPXDecode images without an explicit ColorSpace, the channel
	// count is determined by the JPEG 2000 codestream and only a
	// length-only check is performed at read and write time:
	// 1 ≤ len(MaskColors)/2 ≤ limits.MaxImageChannels.
	//
	// Only one of MaskImage or MaskColors may be specified.
	MaskColors []uint16

	// SMask (optional; PDF 1.4) is a subsidiary image XObject defining a
	// soft-mask image for transparency effects.
	SMask *SoftMask

	// SMaskInData (optional for JPXDecode; PDF 1.5) specifies how soft-mask
	// information encoded with image samples should be used:
	// 0 = ignore encoded soft-mask info (default)
	// 1 = image data includes encoded soft-mask values
	// 2 = image data includes premultiplied opacity channel
	SMaskInData int

	// Interpolate indicates whether image interpolation should be performed by
	// a PDF processor.
	Interpolate bool

	// Alternates (optional) is an array of alternate image dictionaries for this image.
	Alternates []*Alternate

	// OptionalContent (optional) allows to control the visibility of the image.
	OptionalContent oc.Conditional

	// Intent (optional) is the name of a color rendering intent to be used in
	// rendering the image.
	Intent graphics.RenderingIntent

	// StructParent (required if the image is a structural content item)
	// is the integer key of the image's entry in the structural parent tree.
	StructParent optional.UInt

	// Metadata (optional) is a metadata stream containing metadata for the image.
	Metadata *pdf.MetadataStream

	// AssociatedFiles (optional; PDF 2.0) is an array of files associated with
	// the image. The relationship that the associated files have to the
	// XObject is supplied by the Specification.AFRelationship field.
	//
	// This corresponds to the AF entry in the image dictionary.
	AssociatedFiles []*file.Specification

	// Measure (optional; PDF 2.0) specifies the scale and units that apply to
	// the image.
	Measure measure.Measure

	// PtData (optional; PDF 2.0) contains extended geospatial point data.
	PtData *measure.PtData

	// WebCaptureID (optional) is the digital identifier of the image's parent
	// Web Capture content set.
	//
	// This corresponds to the /ID entry in the image mask dictionary.
	WebCaptureID *webcapture.Identifier

	// TODO(voss): OPI

	// Name is the PDF resource-dictionary key under which this image is
	// referenced in content streams.  If non-empty, the builder uses this
	// value as the /XObject subdictionary key; the spec requires the two
	// to match (PDF 2.0 Table 93).  Required in PDF 1.0; optional in PDF
	// 1.1–1.7; deprecated (forbidden by this library's writer) in PDF 2.0.
	Name pdf.Name
}

// ExtractDict extracts an image dictionary from a PDF stream.
func ExtractDict(c pdf.Cursor, obj pdf.Object, _ bool) (*Dict, error) {
	stream, err := c.Stream(obj)
	if err != nil {
		return nil, err
	} else if stream == nil {
		return nil, pdf.Error("missing image stream")
	}
	dict := stream.Dict

	// Check Type and Subtype
	typeName, _ := c.DictTyped(obj, "XObject")
	if typeName == nil {
		// Type is optional, but if present must be XObject
		if t, err := pdf.Optional(c.Name(dict["Type"])); err != nil {
			return nil, err
		} else if t != "" && t != "XObject" {
			return nil, pdf.Errorf("invalid Type %q for image XObject", t)
		}
	}

	subtypeName, err := pdf.Optional(c.Name(dict["Subtype"]))
	if err != nil {
		return nil, err
	}
	if subtypeName != "Image" {
		return nil, pdf.Errorf("invalid Subtype %q for image XObject", subtypeName)
	}

	if isImageMask, err := c.Boolean(dict["ImageMask"]); err == nil && isImageMask {
		return nil, pdf.Error("use ExtractImageMask for image masks")
	}

	filters, err := pdf.Optional(c.Filters(dict))
	if err != nil {
		return nil, err
	}
	hasJPX := false
	for _, f := range filters {
		name, _, err := f.Info(c.Version())
		if err != nil {
			return nil, err
		}
		if name == "JPXDecode" {
			hasJPX = true
			break
		}
	}

	// Extract required fields
	width, err := c.Integer(dict["Width"])
	if err != nil {
		return nil, fmt.Errorf("missing or invalid Width: %w", err)
	}
	if width <= 0 {
		return nil, pdf.Errorf("invalid image width %d", width)
	}
	if width > limits.MaxImageWidth {
		return nil, pdf.Errorf("image width %d exceeds limit", width)
	}

	height, err := c.Integer(dict["Height"])
	if err != nil {
		return nil, fmt.Errorf("missing or invalid Height: %w", err)
	}
	if height <= 0 {
		return nil, pdf.Errorf("invalid image height %d", height)
	}
	if height > limits.MaxImageHeight {
		return nil, pdf.Errorf("image height %d exceeds limit", height)
	}
	if limits.ImagePixelsExceedLimit(int(width), int(height)) {
		return nil, pdf.Error("image pixel count exceeds limit")
	}

	img := &Dict{
		Width:  int(width),
		Height: int(height),
	}

	// Extract ColorSpace (required for images, Pattern not allowed)
	if csObj, ok := dict["ColorSpace"]; ok {
		cs, err := pdf.Decode(c, csObj, color.ExtractSpace)
		if err != nil {
			return nil, fmt.Errorf("invalid ColorSpace: %w", err)
		}
		if cs.Family() == color.FamilyPattern {
			return nil, pdf.Error("Pattern colour space not permitted for images")
		}
		img.ColorSpace = cs
	} else if !hasJPX {
		return nil, pdf.Error("missing ColorSpace for non-JPXDecode image")
	}

	// Extract BitsPerComponent.  For JPXDecode images the entry is
	// optional and shall be ignored if present (spec §8.9.5); drop any
	// value to keep round-trip parity with check() which requires
	// img.BitsPerComponent == 0 for JPX.
	if hasJPX {
		img.BitsPerComponent = 0
	} else if bpc, err := pdf.Optional(c.Integer(dict["BitsPerComponent"])); err != nil {
		return nil, err
	} else if bpc != 0 {
		switch bpc {
		case 1, 2, 4, 8, 16:
			img.BitsPerComponent = int(bpc)
		default:
			return nil, pdf.Errorf("invalid BitsPerComponent %d", bpc)
		}
	} else {
		return nil, pdf.Error("missing BitsPerComponent")
	}

	// Refine the pixel-count cap above with a byte-count check when
	// ColorSpace and BitsPerComponent are known, since up-front-allocating
	// filters bypass the downstream LimitReader.  JPXDecode images do not
	// reach this branch (ColorSpace is optional and may be nil) and are
	// covered by the pixel-count cap alone; their decoder is responsible
	// for honouring channel and bit-depth bounds from the JP2 codestream.
	if img.ColorSpace != nil && img.BitsPerComponent > 0 {
		channels := img.ColorSpace.Channels()
		if limits.ImageBytesExceedLimit(img.Width, img.Height,
			channels, img.BitsPerComponent) {
			return nil, pdf.Error("image data exceeds size limit")
		}
		// the encoded-bytes cap admits up to 64× expansion into the
		// per-channel float64 buffer at bpc=1; cap the decoded form
		// separately to bound decoder memory use
		if limits.ImageDecodedFloat64ExceedsLimit(img.Width, img.Height, channels) {
			return nil, pdf.Error("image decoded data exceeds size limit")
		}
	}

	// Extract optional fields
	if intent, err := pdf.Optional(c.Name(dict["Intent"])); err != nil {
		return nil, err
	} else if intent != "" {
		img.Intent = graphics.RenderingIntent(intent)
	}

	// Mask can either be an image mask or a color key mask array.
	maskObj, err := c.Resolve(dict["Mask"])
	if err != nil {
		return nil, err
	}
	switch maskObj := maskObj.(type) {
	case *pdf.Stream: // image mask stream
		if maskImg, err := pdf.DecodeOptional(c, maskObj, ExtractMask); err != nil {
			return nil, err
		} else {
			img.MaskImage = maskImg
		}
	case pdf.Array: // color key mask array
		var numChannels int
		if img.ColorSpace != nil {
			numChannels = img.ColorSpace.Channels()
			if len(maskObj) != 2*numChannels {
				break
			}
		} else {
			// JPX channel count lives in the codestream; bound the
			// pair count instead of matching it exactly.
			if len(maskObj) < 2 ||
				len(maskObj) > 2*limits.MaxImageChannels ||
				len(maskObj)%2 != 0 {
				break
			}
			numChannels = len(maskObj) / 2
		}

		maskColors := make([]uint16, len(maskObj))
		valid := true
		for i, val := range maskObj {
			num, err := c.Integer(val)
			if err != nil || num < 0 || num > 0xFFFF {
				valid = false
				break
			}
			if img.BitsPerComponent > 0 && img.BitsPerComponent < 16 {
				maskColors[i] = uint16(num) % (1 << img.BitsPerComponent)
			} else {
				maskColors[i] = uint16(num)
			}
		}
		for i := 0; valid && i < numChannels; i++ {
			if maskColors[2*i] > maskColors[2*i+1] {
				valid = false
			}
		}
		if valid {
			img.MaskColors = maskColors
		}
	}

	// Extract Decode array, substituting the default if not present.
	// For JPXDecode images without an explicit ColorSpace, the spec
	// (§7.4.9, §8.9.5) says the Decode array shall be ignored unless
	// ImageMask is true.  Skip extraction entirely in that case and
	// leave img.Decode nil.
	if img.ColorSpace != nil {
		// honor a Decode array with at least 2*ncomp entries, ignoring any
		// surplus; a shorter array falls back to the default
		n := 2 * img.ColorSpace.Channels()
		if decodeArray, err := pdf.Optional(c.Array(dict["Decode"])); err != nil {
			return nil, err
		} else if len(decodeArray) >= n {
			decode := make([]float64, n)
			valid := true
			for i := range decode {
				num, err := c.Number(decodeArray[i])
				if pdf.IsMalformed(err) {
					valid = false
					break
				} else if err != nil {
					return nil, err
				}
				decode[i] = num
			}
			if valid {
				img.Decode = decode
			}
		}
		if img.Decode == nil {
			img.Decode = DefaultDecode(img.ColorSpace, img.BitsPerComponent)
		}
	}

	// Extract Interpolate
	if interp, err := c.Boolean(dict["Interpolate"]); err == nil {
		img.Interpolate = bool(interp)
	}

	// extract alternates (Table 89); drop the whole list if it exceeds
	// MaxAlternates rather than silently truncate
	if alts, err := pdf.Optional(c.Array(dict["Alternates"])); err != nil {
		return nil, err
	} else if len(alts) <= limits.MaxAlternates {
		for i, altObj := range alts {
			alt, err := pdf.DecodeOptional(c, altObj, ExtractAlternate)
			if err != nil {
				return nil, fmt.Errorf("invalid Alternates[%d]: %w", i, err)
			}
			if alt != nil {
				img.Alternates = append(img.Alternates, alt)
			}
		}
	}

	// Extract SMask (soft-mask image)
	if smaskObj, ok := dict["SMask"]; ok {
		smask, err := pdf.DecodeOptional(c, smaskObj, ExtractSoftMask)
		if err != nil {
			return nil, fmt.Errorf("invalid SMask: %w", err)
		}
		img.SMask = smask
	}

	// Extract SMaskInData (valid values are 0, 1, 2; invalid values default to 0).
	// Per spec §8.9.5.1 the entry is only meaningful for JPXDecode
	// images; drop it for any other filter so that round-trip via
	// [Dict.Embed] does not fail its consistency check.
	if smd, err := pdf.Optional(c.Integer(dict["SMaskInData"])); err != nil {
		return nil, err
	} else if hasJPX && (smd == 1 || smd == 2) {
		img.SMaskInData = int(smd)
		// Per spec: if SMaskInData is non-zero, SMask should be ignored
		if img.SMask != nil {
			img.SMask = nil
		}
	}

	// Extract Name (deprecated in PDF 2.0)
	if name, err := pdf.Optional(c.Name(dict["Name"])); err != nil {
		return nil, err
	} else {
		img.Name = name
	}

	// Extract Metadata
	if metaObj, ok := dict["Metadata"]; ok {
		meta, err := pdf.Decode(c, metaObj, pdf.ExtractMetadataStream)
		if err != nil {
			return nil, fmt.Errorf("invalid Metadata: %w", err)
		}
		img.Metadata = meta
	}

	// OC (optional)
	if oc, err := pdf.DecodeOptional(c, dict["OC"], oc.ExtractConditional); err != nil {
		return nil, err
	} else {
		img.OptionalContent = oc
	}

	// Extract Measure
	if measureObj, ok := dict["Measure"]; ok {
		m, err := pdf.Decode(c, measureObj, measure.Extract)
		if err != nil {
			return nil, fmt.Errorf("invalid Measure: %w", err)
		}
		img.Measure = m
	}

	// Extract PtData
	if ptData, err := pdf.DecodeOptional(c, dict["PtData"], measure.ExtractPtData); err != nil {
		return nil, err
	} else {
		img.PtData = ptData
	}

	// Extract StructParent
	if keyObj := dict["StructParent"]; keyObj != nil {
		if key, err := pdf.Optional(c.Integer(dict["StructParent"])); err != nil {
			return nil, err
		} else if key >= 0 && uint64(key) <= math.MaxUint {
			img.StructParent.Set(uint(key))
		}
	}

	// Extract AssociatedFiles (AF); drop the whole list if it exceeds
	// MaxAssociatedFiles rather than silently truncate
	if afArray, err := pdf.Optional(c.Array(dict["AF"])); err != nil {
		return nil, err
	} else if afArray != nil && len(afArray) <= limits.MaxAssociatedFiles {
		img.AssociatedFiles = make([]*file.Specification, 0, len(afArray))
		for _, afObj := range afArray {
			if spec, err := pdf.DecodeOptional(c, afObj, file.ExtractSpecification); err != nil {
				return nil, err
			} else if spec != nil {
				img.AssociatedFiles = append(img.AssociatedFiles, spec)
			}
		}
	}

	// Extract WebCaptureID (ID)
	if webID, err := pdf.DecodeOptional(c, dict["ID"], webcapture.ExtractIdentifier); err != nil {
		return nil, err
	} else if webID != nil {
		img.WebCaptureID = webID
	}

	// lazily read from the original stream, preserving the encoding
	img.Data = &streamData{
		inner:    opaque.ExtractStream(c.Extractor(), stream),
		isJPX:    hasJPX,
		maxBytes: ImageDataLimit(img.Width, img.Height, img.BitsPerComponent, img.ColorSpace),
	}

	return img, nil
}

var _ graphics.Image = (*Dict)(nil)

// FromImage creates a Dict from an image.Image.
// The ColorSpace and BitsPerComponent must be set appropriately for the image.
func FromImage(img image.Image, colorSpace color.Space, bitsPerComponent int) *Dict {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	return &Dict{
		Width:            width,
		Height:           height,
		ColorSpace:       colorSpace,
		BitsPerComponent: bitsPerComponent,
		Data: &FlateSource{
			Predictor:        15,
			Width:            width,
			Colors:           colorSpace.Channels(),
			BitsPerComponent: bitsPerComponent,
			WriteData: func(w io.Writer) error {
				return writeImageData(w, img, colorSpace, bitsPerComponent)
			},
		},
	}
}

// writeImageData writes image data from an image.Image to the provided writer.
func writeImageData(w io.Writer, img image.Image, colorSpace color.Space, bitsPerComponent int) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	numChannels := colorSpace.Channels()

	buf := NewPixelRow(width*numChannels, bitsPerComponent)
	for y := range height {
		buf.Reset()

		for x := range width {
			pixCol := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			shift := 16 - bitsPerComponent
			switch colorSpace.Family() {
			case color.FamilyDeviceGray, color.FamilyCalGray:
				g16 := gocol.Gray16Model.Convert(pixCol).(gocol.Gray16).Y
				buf.AppendBits(g16 >> shift)

			case color.FamilyDeviceRGB, color.FamilyCalRGB:
				c16 := gocol.NRGBA64Model.Convert(pixCol).(gocol.NRGBA64)
				buf.AppendBits(c16.R >> shift)
				buf.AppendBits(c16.G >> shift)
				buf.AppendBits(c16.B >> shift)

			case color.FamilyDeviceCMYK:
				c16 := gocol.NRGBA64Model.Convert(pixCol).(gocol.NRGBA64)
				c, m, y, k := rgbToCMYK(c16.R, c16.G, c16.B)
				buf.AppendBits(c >> shift)
				buf.AppendBits(m >> shift)
				buf.AppendBits(y >> shift)
				buf.AppendBits(k >> shift)

			// TODO(voss): implement the remaining color spaces
			case color.FamilyLab:
				return errors.New("color space Lab not implemented")
			case color.FamilyICCBased:
				return errors.New("color space ICCBased not implemented")
			case color.FamilyIndexed:
				return errors.New("color space Indexed not implemented")
			case color.FamilySeparation:
				return errors.New("color space Separation not implemented")
			case color.FamilyDeviceN:
				return errors.New("color space DeviceN not implemented")
			}
		}

		_, err := w.Write(buf.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

func rgbToCMYK(r, g, b uint16) (c, m, y, k uint16) {
	maxVal := max(r, g, b)

	if maxVal == 0 {
		return 0, 0, 0, 0xffff
	}

	k = 0xffff - maxVal

	c = uint16((uint32(maxVal-r) * 0xffff) / uint32(maxVal))
	m = uint16((uint32(maxVal-g) * 0xffff) / uint32(maxVal))
	y = uint16((uint32(maxVal-b) * 0xffff) / uint32(maxVal))

	return c, m, y, k
}

// FromImageWithMask creates a Dict with an associated ImageMask from two image.Image objects.
func FromImageWithMask(img image.Image, mask image.Image, colorSpace color.Space, bitsPerComponent int) *Dict {
	dict := FromImage(img, colorSpace, bitsPerComponent)
	dict.MaskImage = FromImageMask(mask)
	return dict
}

// Embed adds the image to the PDF file and returns the embedded object.
func (d *Dict) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := d.check(rm.Out()); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Image"),
		"Width":   pdf.Integer(d.Width),
		"Height":  pdf.Integer(d.Height),
	}
	if d.ColorSpace != nil {
		csEmbedded, err := rm.Embed(d.ColorSpace)
		if err != nil {
			return nil, err
		}
		dict["ColorSpace"] = csEmbedded
	}
	if d.BitsPerComponent != 0 {
		dict["BitsPerComponent"] = pdf.Integer(d.BitsPerComponent)
	}
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("XObject")
	}
	if d.Intent != "" {
		dict["Intent"] = pdf.Name(d.Intent)
	}
	if d.MaskImage != nil {
		ref, err := rm.Embed(d.MaskImage)
		if err != nil {
			return nil, err
		}
		dict["Mask"] = ref
	} else if d.MaskColors != nil {
		var mask pdf.Array
		for _, v := range d.MaskColors {
			mask = append(mask, pdf.Integer(v))
		}
		dict["Mask"] = mask
	}
	if d.Decode != nil && d.ColorSpace != nil &&
		!slices.Equal(d.Decode, DefaultDecode(d.ColorSpace, d.BitsPerComponent)) {
		var decode pdf.Array
		for _, v := range d.Decode {
			decode = append(decode, pdf.Number(v))
		}
		dict["Decode"] = decode
	}
	if d.Interpolate {
		dict["Interpolate"] = pdf.Boolean(true)
	}
	if len(d.Alternates) > 0 {
		var alts pdf.Array
		for _, alt := range d.Alternates {
			if alt == nil {
				return nil, errors.New("nil entry in Alternates")
			}
			ref, err := rm.Embed(alt)
			if err != nil {
				return nil, err
			}
			alts = append(alts, ref)
		}
		dict["Alternates"] = alts
	}

	// Handle SMask/SMaskInData (mutually exclusive)
	if d.SMask != nil && d.SMaskInData == 0 {
		if err := pdf.CheckVersion(rm.Out(), "soft mask images", pdf.V1_4); err != nil {
			return nil, err
		}
		ref, err := rm.Embed(d.SMask)
		if err != nil {
			return nil, err
		}
		dict["SMask"] = ref
	} else if d.SMaskInData > 0 {
		if err := pdf.CheckVersion(rm.Out(), "SMaskInData", pdf.V1_5); err != nil {
			return nil, err
		}
		dict["SMaskInData"] = pdf.Integer(d.SMaskInData)
	}

	if d.Name != "" {
		dict["Name"] = d.Name
	}
	if d.Metadata != nil {
		ref, err := rm.Embed(d.Metadata)
		if err != nil {
			return nil, err
		}
		dict["Metadata"] = ref
	}

	if d.OptionalContent != nil {
		if err := pdf.CheckVersion(rm.Out(), "Image dict OC entry", pdf.V1_5); err != nil {
			return nil, err
		}
		embedded, err := rm.Embed(d.OptionalContent)
		if err != nil {
			return nil, err
		}
		dict["OC"] = embedded
	}

	if d.Measure != nil {
		embedded, err := rm.Embed(d.Measure)
		if err != nil {
			return nil, err
		}
		dict["Measure"] = embedded
	}

	if d.PtData != nil {
		if err := pdf.CheckVersion(rm.Out(), "image dictionary PtData entry", pdf.V2_0); err != nil {
			return nil, err
		}
		embedded, err := rm.Embed(d.PtData)
		if err != nil {
			return nil, err
		}
		dict["PtData"] = embedded
	}

	if key, ok := d.StructParent.Get(); ok {
		if err := pdf.CheckVersion(rm.Out(), "image dictionary StructParent entry", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["StructParent"] = pdf.Integer(key)
	}

	if len(d.AssociatedFiles) > 0 {
		if err := pdf.CheckVersion(rm.Out(), "image dictionary AF entry", pdf.V2_0); err != nil {
			return nil, err
		}

		// Validate each file specification can be used as associated file
		version := pdf.GetVersion(rm.Out())
		for i, spec := range d.AssociatedFiles {
			if spec == nil {
				continue
			}
			if err := spec.CanBeAF(version); err != nil {
				return nil, fmt.Errorf("AssociatedFiles[%d]: %w", i, err)
			}
		}

		// Embed the file specifications
		var afArray pdf.Array
		for _, spec := range d.AssociatedFiles {
			if spec != nil {
				embedded, err := rm.Embed(spec)
				if err != nil {
					return nil, err
				}
				afArray = append(afArray, embedded)
			}
		}
		dict["AF"] = afArray
	}

	if d.WebCaptureID != nil {
		if err := pdf.CheckVersion(rm.Out(), "image dictionary ID entry", pdf.V1_3); err != nil {
			return nil, err
		}
		embedded, err := rm.Embed(d.WebCaptureID)
		if err != nil {
			return nil, err
		}
		dict["ID"] = embedded
	}

	ref := rm.Alloc()
	if err := d.Data.WriteStream(rm, ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

func (d *Dict) check(out *pdf.Writer) error {
	if d.Width <= 0 {
		return fmt.Errorf("invalid image width %d", d.Width)
	}
	if d.Height <= 0 {
		return fmt.Errorf("invalid image height %d", d.Height)
	}
	if d.Data == nil {
		return errors.New("source cannot be nil")
	}

	isJPX := d.Data.IsJPX()

	if d.ColorSpace != nil {
		if fam := d.ColorSpace.Family(); fam == color.FamilyPattern {
			return fmt.Errorf("invalid image color space %q", fam)
		}
	} else if !isJPX {
		return errors.New("missing ColorSpace for non-JPXDecode image")
	}

	// JPXDecode images carry their bit depth in the JP2 codestream;
	// spec §8.9.5 says the dict entry is ignored if present.
	if isJPX && d.BitsPerComponent != 0 {
		return errors.New("BitsPerComponent must be 0 for JPXDecode images")
	}
	switch d.BitsPerComponent {
	case 0:
		if !isJPX {
			return errors.New("missing BitsPerComponent")
		}
	case 1, 2, 4, 8:
		// pass
	case 16:
		if err := pdf.CheckVersion(out, "16 bits per image component", pdf.V1_5); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid BitsPerComponent %d", d.BitsPerComponent)
	}

	if d.Intent != "" {
		if err := pdf.CheckVersion(out, "rendering intents", pdf.V1_1); err != nil {
			return err
		}
	}

	var numChannels int
	if d.ColorSpace != nil {
		numChannels = d.ColorSpace.Channels()
	}
	if d.MaskImage != nil || d.MaskColors != nil {
		if err := pdf.CheckVersion(out, "image masks", pdf.V1_3); err != nil {
			return err
		}
		if d.MaskImage != nil && d.MaskColors != nil {
			return errors.New("only one of MaskImage or MaskColors may be specified")
		}
		if d.MaskColors != nil {
			if d.ColorSpace != nil {
				if len(d.MaskColors) != 2*numChannels {
					return fmt.Errorf("wrong MaskColors length: expected %d, got %d",
						2*numChannels, len(d.MaskColors))
				}
			} else {
				if len(d.MaskColors) < 2 ||
					len(d.MaskColors) > 2*limits.MaxImageChannels ||
					len(d.MaskColors)%2 != 0 {
					return fmt.Errorf("invalid MaskColors length %d", len(d.MaskColors))
				}
				numChannels = len(d.MaskColors) / 2
			}
			if d.BitsPerComponent > 0 {
				maxVal := uint16(1<<d.BitsPerComponent - 1)
				for i, v := range d.MaskColors {
					if v > maxVal {
						return fmt.Errorf("MaskColors[%d] value %d exceeds maximum %d", i, v, maxVal)
					}
				}
			}
			for i := range numChannels {
				if d.MaskColors[2*i] > d.MaskColors[2*i+1] {
					return fmt.Errorf("MaskColors[%d] > MaskColors[%d]", 2*i, 2*i+1)
				}
			}
		}
	}
	if d.Decode != nil {
		if d.ColorSpace == nil {
			return errors.New("Decode array not permitted when ColorSpace is absent")
		}
		if len(d.Decode) != 2*numChannels {
			return fmt.Errorf("wrong Decode length: expected %d, got %d",
				2*numChannels, len(d.Decode))
		}
	}

	if d.Alternates != nil {
		if err := pdf.CheckVersion(out, "image alternates", pdf.V1_3); err != nil {
			return err
		}
		defaultForPrintingCount := 0
		for _, alt := range d.Alternates {
			if hasNestedAlternates(alt.Image) {
				return errors.New("alternates of alternates not allowed")
			}
			if alt.DefaultForPrinting {
				defaultForPrintingCount++
			}
		}
		if defaultForPrintingCount > 1 {
			return errors.New("at most one alternate may have DefaultForPrinting set")
		}
	}

	switch v := pdf.GetVersion(out); {
	case v == pdf.V1_0 && d.Name == "":
		return &pdf.VersionError{
			Operation: "image XObject without /Name field",
			Earliest:  pdf.V1_1,
		}
	case v >= pdf.V2_0 && d.Name != "":
		return &pdf.VersionError{
			Operation: "image XObject /Name field",
			Latest:    pdf.V1_7,
		}
	}

	if d.Metadata != nil {
		if err := pdf.CheckVersion(out, "image metadata", pdf.V1_4); err != nil {
			return err
		}
	}

	if d.Measure != nil {
		if err := pdf.CheckVersion(out, "image dictionary Measure entry", pdf.V2_0); err != nil {
			return err
		}
	}

	// Validate SMask/SMaskInData
	if d.SMask != nil && d.SMaskInData != 0 {
		return errors.New("SMask and SMaskInData are mutually exclusive")
	}

	if d.SMaskInData != 0 {
		if !isJPX {
			return errors.New("SMaskInData only permitted on JPXDecode images")
		}
		if d.SMaskInData < 0 || d.SMaskInData > 2 {
			return fmt.Errorf("invalid SMaskInData value %d (must be 0, 1, or 2)", d.SMaskInData)
		}
		if err := pdf.CheckVersion(out, "SMaskInData", pdf.V1_5); err != nil {
			return err
		}
	}

	if d.SMask != nil {
		if err := pdf.CheckVersion(out, "soft mask images", pdf.V1_4); err != nil {
			return err
		}

		// Validate soft mask dimensions match if Matte is present
		if d.SMask.Matte != nil {
			if d.SMask.Width != d.Width || d.SMask.Height != d.Height {
				return errors.New("soft mask dimensions mismatch")
			}
			// Validate Matte length matches color space channels.  When
			// ColorSpace is absent (JPX), we only bound the length.
			if d.ColorSpace != nil {
				if len(d.SMask.Matte) != numChannels {
					return fmt.Errorf("matte array length %d does not match color space channels %d",
						len(d.SMask.Matte), numChannels)
				}
			} else if len(d.SMask.Matte) < 1 ||
				len(d.SMask.Matte) > limits.MaxImageChannels {
				return fmt.Errorf("invalid Matte length %d", len(d.SMask.Matte))
			}
		}
	}

	// Validate AssociatedFiles
	if len(d.AssociatedFiles) > 0 {
		if err := pdf.CheckVersion(out, "image dictionary AssociatedFiles entry", pdf.V2_0); err != nil {
			return err
		}

		version := pdf.GetVersion(out)
		for i, spec := range d.AssociatedFiles {
			if spec == nil {
				continue
			}
			if err := spec.CanBeAF(version); err != nil {
				return fmt.Errorf("AssociatedFiles[%d]: %w", i, err)
			}
		}
	}

	return nil
}

// Bounds returns the dimensions of the image.
func (d *Dict) Bounds() rect.IntRect {
	return rect.IntRect{
		XMin: 0,
		YMin: 0,
		XMax: d.Width,
		YMax: d.Height,
	}
}

// Subtype returns the PDF XObject subtype for images.
func (d *Dict) Subtype() pdf.Name {
	return pdf.Name("Image")
}

// ResourceName returns the preferred resource-dictionary key for this image.
// See [graphics.XObject.ResourceName].
func (d *Dict) ResourceName() pdf.Name {
	return d.Name
}
