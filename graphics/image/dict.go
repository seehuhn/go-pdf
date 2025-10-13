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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/measure"
	"seehuhn.de/go/pdf/metadata"
	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/structure"
	"seehuhn.de/go/pdf/webcapture"
)

// PDF 2.0 sections: 8.9.5

type Dict struct {
	// Width is the width of the image in pixels.
	Width int

	// Height is the height of the image in pixels.
	Height int

	// ColorSpace is the color space in which image samples are specified.
	// It can be any type of color space except Pattern.
	ColorSpace color.Space

	// BitsPerComponent is the number of bits used to represent each color component.
	// The value must be 1, 2, 4, 8, or (from PDF 1.5) 16.
	BitsPerComponent int

	// Decode (optional) is an array of numbers describing how to map image
	// samples into the range of values appropriate for the image's color
	// space. The slice must have twice the number of color components in the
	// ColorSpace.
	Decode []float64

	// WriteData is a function that writes the image data to the provided
	// writer. The data should be written row by row, with each row containing
	// Width * ColorSpace.Channels() samples, each sample using
	// BitsPerComponent bits.
	WriteData func(io.Writer) error

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
	Alternates []*Dict

	// OptionalContent (optional) allows to control the visibility of the image.
	OptionalContent oc.Conditional

	// Intent (optional) is the name of a color rendering intent to be used in
	// rendering the image.
	Intent graphics.RenderingIntent

	// StructParent (required if the image is a structural content item)
	// is the integer key of the image's entry in the structural parent tree.
	StructParent structure.Key

	// Metadata (optional) is a metadata stream containing metadata for the image.
	Metadata *metadata.Stream

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

	// Name is deprecated and should be left empty.
	// Only used in PDF 1.0 where it was the name used to reference the image
	// mask from within content streams.
	Name pdf.Name
}

// ExtractDict extracts an image dictionary from a PDF stream.
func ExtractDict(x *pdf.Extractor, obj pdf.Object) (*Dict, error) {
	stream, err := x.GetStream(obj)
	if err != nil {
		return nil, err
	} else if stream == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing image stream"),
		}
	}
	dict := stream.Dict

	// Check Type and Subtype
	typeName, _ := x.GetDictTyped(obj, "XObject")
	if typeName == nil {
		// Type is optional, but if present must be XObject
		if t, err := pdf.Optional(x.GetName(dict["Type"])); err != nil {
			return nil, err
		} else if t != "" && t != "XObject" {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("invalid Type %q for image XObject", t),
			}
		}
	}

	subtypeName, err := pdf.Optional(x.GetName(dict["Subtype"]))
	if err != nil {
		return nil, err
	}
	if subtypeName != "Image" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("invalid Subtype %q for image XObject", subtypeName),
		}
	}

	// Check if this is an image mask (should use ExtractImageMask instead)
	if isImageMask, err := x.GetBoolean(dict["ImageMask"]); err == nil && bool(isImageMask) {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("use ExtractImageMask for image masks"),
		}
	}

	// Extract required fields
	width, err := x.GetInteger(dict["Width"])
	if err != nil {
		return nil, fmt.Errorf("missing or invalid Width: %w", err)
	}
	if width <= 0 {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("invalid image width %d", width),
		}
	}

	height, err := x.GetInteger(dict["Height"])
	if err != nil {
		return nil, fmt.Errorf("missing or invalid Height: %w", err)
	}
	if height <= 0 {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("invalid image height %d", height),
		}
	}

	img := &Dict{
		Width:  int(width),
		Height: int(height),
	}

	// Extract ColorSpace (required for images)
	if csObj, ok := dict["ColorSpace"]; ok {
		cs, err := color.ExtractSpace(x, csObj)
		if err != nil {
			return nil, fmt.Errorf("invalid ColorSpace: %w", err)
		}
		img.ColorSpace = cs
	} else {
		// ColorSpace is optional only for JPXDecode filter
		filters, _ := x.GetArray(dict["Filter"])
		hasJPX := false
		for _, f := range filters {
			if name, _ := x.GetName(f); name == "JPXDecode" {
				hasJPX = true
				break
			}
		}
		if !hasJPX {
			return nil, &pdf.MalformedFileError{
				Err: errors.New("missing ColorSpace for non-JPXDecode image"),
			}
		}
	}

	// Extract BitsPerComponent (required except for JPXDecode)
	if bpc, err := pdf.Optional(x.GetInteger(dict["BitsPerComponent"])); err != nil {
		return nil, err
	} else if bpc > 0 {
		img.BitsPerComponent = int(bpc)
	} else if img.ColorSpace != nil {
		// BitsPerComponent is required when ColorSpace is present
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing BitsPerComponent"),
		}
	}

	// Extract optional fields
	if intent, err := pdf.Optional(x.GetName(dict["Intent"])); err != nil {
		return nil, err
	} else if intent != "" {
		img.Intent = graphics.RenderingIntent(intent)
	}

	// Mask can either be an image mask or a color key mask array.
	maskObj, err := pdf.Resolve(x.R, dict["Mask"])
	if err != nil {
		return nil, err
	}
	switch maskObj := maskObj.(type) {
	case *pdf.Stream: // image mask stream
		if maskImg, err := pdf.ExtractorGetOptional(x, maskObj, ExtractMask); err != nil {
			return nil, err
		} else {
			img.MaskImage = maskImg
		}
	case pdf.Array: // color key mask array
		img.MaskColors = make([]uint16, len(maskObj))
		for i, val := range maskObj {
			if num, err := x.GetInteger(val); err != nil {
				return nil, fmt.Errorf("invalid MaskColors[%d]: %w", i, err)
			} else {
				img.MaskColors[i] = uint16(num)
			}
		}
	}

	// Extract Decode array
	if decodeArray, err := pdf.Optional(x.GetArray(dict["Decode"])); err != nil {
		return nil, err
	} else if decodeArray != nil {
		img.Decode = make([]float64, len(decodeArray))
		for i, val := range decodeArray {
			if num, err := x.GetNumber(val); err != nil {
				return nil, fmt.Errorf("invalid Decode[%d]: %w", i, err)
			} else {
				img.Decode[i] = num
			}
		}
	}

	// Extract Interpolate
	if interp, err := x.GetBoolean(dict["Interpolate"]); err == nil {
		img.Interpolate = bool(interp)
	}

	// Extract Alternates
	if alts, err := pdf.Optional(x.GetArray(dict["Alternates"])); err != nil {
		return nil, err
	} else if alts != nil {
		img.Alternates = make([]*Dict, len(alts))
		for i, altObj := range alts {
			altDict, err := pdf.ExtractorGetOptional(x, altObj, ExtractDict)
			if err != nil {
				return nil, fmt.Errorf("invalid Alternates[%d]: %w", i, err)
			}
			img.Alternates[i] = altDict
		}
	}

	// Extract SMask (soft-mask image)
	if smaskObj, ok := dict["SMask"]; ok {
		smask, err := pdf.ExtractorGetOptional(x, smaskObj, func(x *pdf.Extractor, obj pdf.Object) (*SoftMask, error) {
			return ExtractSoftMask(x, obj)
		})
		if err != nil {
			return nil, fmt.Errorf("invalid SMask: %w", err)
		}
		img.SMask = smask
	}

	// Extract SMaskInData
	if smd, err := pdf.Optional(x.GetInteger(dict["SMaskInData"])); err != nil {
		return nil, err
	} else if smd > 0 {
		img.SMaskInData = int(smd)
		// Per spec: if SMaskInData is non-zero, SMask should be ignored
		if img.SMask != nil {
			img.SMask = nil
		}
	}

	// Extract Name (deprecated in PDF 2.0)
	if name, err := pdf.Optional(x.GetName(dict["Name"])); err != nil {
		return nil, err
	} else {
		img.Name = name
	}

	// Extract Metadata
	if metaObj, ok := dict["Metadata"]; ok {
		meta, err := metadata.Extract(x.R, metaObj)
		if err != nil {
			return nil, fmt.Errorf("invalid Metadata: %w", err)
		}
		img.Metadata = meta
	}

	// OC (optional)
	if oc, err := pdf.ExtractorGetOptional(x, dict["OC"], oc.ExtractConditional); err != nil {
		return nil, err
	} else {
		img.OptionalContent = oc
	}

	// Extract Measure
	if measureObj, ok := dict["Measure"]; ok {
		m, err := measure.Extract(x.R, measureObj)
		if err != nil {
			return nil, fmt.Errorf("invalid Measure: %w", err)
		}
		img.Measure = m
	}

	// Extract PtData
	if ptData, err := pdf.Optional(measure.ExtractPtData(x.R, dict["PtData"])); err != nil {
		return nil, err
	} else {
		img.PtData = ptData
	}

	// Extract StructParent
	if keyObj := dict["StructParent"]; keyObj != nil {
		if key, err := pdf.Optional(x.GetInteger(dict["StructParent"])); err != nil {
			return nil, err
		} else {
			img.StructParent.Set(key)
		}
	}

	// Extract AssociatedFiles (AF)
	if afArray, err := pdf.Optional(x.GetArray(dict["AF"])); err != nil {
		return nil, err
	} else if afArray != nil {
		img.AssociatedFiles = make([]*file.Specification, 0, len(afArray))
		for _, afObj := range afArray {
			if spec, err := pdf.ExtractorGetOptional(x, afObj, file.ExtractSpecification); err != nil {
				return nil, err
			} else if spec != nil {
				img.AssociatedFiles = append(img.AssociatedFiles, spec)
			}
		}
	}

	// Extract WebCaptureID (ID)
	if webID, err := pdf.ExtractorGetOptional(x, dict["ID"], webcapture.ExtractIdentifier); err != nil {
		return nil, err
	} else if webID != nil {
		img.WebCaptureID = webID
	}

	// Create WriteData function as a closure
	img.WriteData = func(w io.Writer) error {
		stm, err := pdf.DecodeStream(x.R, stream, 0)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, stm)
		if err != nil {
			stm.Close()
			return err
		}
		return stm.Close()
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
		WriteData: func(w io.Writer) error {
			return writeImageData(w, img, colorSpace, bitsPerComponent)
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

	csEmbedded, err := rm.Embed(d.ColorSpace)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(d.Width),
		"Height":           pdf.Integer(d.Height),
		"ColorSpace":       csEmbedded,
		"BitsPerComponent": pdf.Integer(d.BitsPerComponent),
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
	if d.Decode != nil {
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
	compress := pdf.FilterCompress{
		"Predictor":        pdf.Integer(15), // TODO(voss): check that this is a good choice
		"Colors":           pdf.Integer(d.ColorSpace.Channels()),
		"BitsPerComponent": pdf.Integer(d.BitsPerComponent),
		"Columns":          pdf.Integer(d.Width),
	}
	w, err := rm.Out().OpenStream(ref, dict, compress)
	if err != nil {
		return nil, fmt.Errorf("cannot open image stream: %w", err)
	}

	err = d.WriteData(w)
	if err != nil {
		return nil, err
	}

	err = w.Close()
	if err != nil {
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
	if d.WriteData == nil {
		return errors.New("WriteData function cannot be nil")
	}

	if fam := d.ColorSpace.Family(); fam == color.FamilyPattern {
		return fmt.Errorf("invalid image color space %q", fam)
	}

	switch d.BitsPerComponent {
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

	numChannels := d.ColorSpace.Channels()
	if d.MaskImage != nil || d.MaskColors != nil {
		if err := pdf.CheckVersion(out, "image masks", pdf.V1_3); err != nil {
			return err
		}
		if d.MaskImage != nil && d.MaskColors != nil {
			return errors.New("only one of MaskImage or MaskColors may be specified")
		}
		if d.MaskColors != nil && len(d.MaskColors) != 2*numChannels {
			return fmt.Errorf("wrong MaskColors length: expected %d, got %d",
				2*numChannels, len(d.MaskColors))
		}
		if d.MaskColors != nil {
			maxVal := uint16(1<<d.BitsPerComponent - 1)
			for i, v := range d.MaskColors {
				if v > maxVal {
					return fmt.Errorf("MaskColors[%d] value %d exceeds maximum %d", i, v, maxVal)
				}
			}
		}
	}
	if d.Decode != nil && len(d.Decode) != 2*numChannels {
		return fmt.Errorf("wrong Decode length: expected %d, got %d",
			2*numChannels, len(d.Decode))
	}

	if d.Alternates != nil {
		if err := pdf.CheckVersion(out, "image alternates", pdf.V1_3); err != nil {
			return err
		}
		for _, alt := range d.Alternates {
			if len(alt.Alternates) > 0 {
				return errors.New("alternates of alternates not allowed")
			}
		}
	}

	if d.Name != "" {
		v := pdf.GetVersion(out)
		if v >= pdf.V2_0 {
			return errors.New("unexpected /Name field")
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
		if d.SMaskInData < 0 || d.SMaskInData > 2 {
			return fmt.Errorf("invalid SMaskInData value %d (must be 0, 1, or 2)", d.SMaskInData)
		}
		if err := pdf.CheckVersion(out, "SMaskInData", pdf.V1_5); err != nil {
			return err
		}
		// Note: SMaskInData is only meaningful for JPXDecode filter
		// Full validation would require filter information
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
			// Validate Matte length matches color space channels
			if len(d.SMask.Matte) != d.ColorSpace.Channels() {
				return fmt.Errorf("Matte array length %d doesn't match color space channels %d",
					len(d.SMask.Matte), d.ColorSpace.Channels())
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
func (d *Dict) Bounds() graphics.Rectangle {
	return graphics.Rectangle{
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
