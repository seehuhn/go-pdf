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
	"io"
	"math"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/limits"
	"seehuhn.de/go/pdf/measure"

	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/optional"
	"seehuhn.de/go/pdf/webcapture"
)

// PDF 2.0 sections: 8.9.5 8.9.6

// Mask represents a stencil mask image XObject, used to mask out areas when
// painting an image.
//
// To extract an image mask from a PDF file, use
// [seehuhn.de/go/pdf/graphics/extract.ImageMask].
type Mask struct {
	// Width is the width of the image mask in pixels.
	Width int

	// Height is the height of the image mask in pixels.
	Height int

	// Inverted indicates the meaning of individual bits in the image data:
	//   - false: 0=opaque and 1=transparent
	//   - true: 1=opaque and 0=transparent
	Inverted bool

	// Source writes the body of the mask stream when the Mask is embedded.
	// For raw 1-bit data use [CCITTFaxSource]; for JPEG 2000 data with a
	// single 1-bit colour channel use [JPXSource].  Other pre-encoded
	// formats such as JBIG2 provide their own Source implementations.
	Source graphics.ImageData

	// Interpolate enables edge smoothing for the mask to reduce jagged
	// appearance in low-resolution stencil masks.
	Interpolate bool

	// Alternates (optional) is an array of alternate image dictionaries for this mask.
	Alternates []*Alternate

	// OptionalContent (optional) allows to control the visibility of the mask.
	OptionalContent oc.Conditional

	// StructParent (required if the image mask is a structural content item)
	// is the integer key of the image mask's entry in the structural parent tree.
	StructParent optional.UInt

	// Metadata (optional) is a metadata stream containing metadata for the image.
	Metadata *pdf.MetadataStream

	// AssociatedFiles (optional; PDF 2.0) is an array of files associated with
	// the mask. The relationship that the associated files have to the
	// XObject is supplied by the Specification.AFRelationship field.
	//
	// This corresponds to the AF entry in the image mask dictionary.
	AssociatedFiles []*file.Specification

	// Measure (optional; PDF 2.0) specifies the scale and units which apply to
	// the mask.
	Measure measure.Measure

	// PtData (optional; PDF 2.0) contains extended geospatial point data.
	PtData *measure.PtData

	// WebCaptureID (optional) is the digital identifier of the image's parent
	// Web Capture content set.
	//
	// This corresponds to the /ID entry in the image mask dictionary.
	WebCaptureID *webcapture.Identifier

	// Name is the PDF resource-dictionary key under which this image mask
	// is referenced in content streams.  If non-empty, the builder uses
	// this value as the /XObject subdictionary key; the spec requires the
	// two to match (PDF 2.0 Table 93).  Required in PDF 1.0; optional in
	// PDF 1.1–1.7; deprecated (forbidden by this library's writer) in PDF
	// 2.0.
	Name pdf.Name
}

var _ graphics.Image = (*Mask)(nil)

// FromImageMask creates an ImageMask from an image.Image.
// Only the alpha channel is used, with alpha values rounded to full opacity or full transparency.
func FromImageMask(img image.Image) *Mask {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	return &Mask{
		Width:  width,
		Height: height,
		Source: NewCCITTFaxSource(width, func(w io.Writer) error {
			return writeImageMaskData(w, img)
		}),
	}
}

// writeImageMaskData writes mask data from an image.Image to the provided writer.
func writeImageMaskData(w io.Writer, img image.Image) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Mask data is encoded as a continuous bit stream, with the high-order bit
	// of each byte first. Each row starts at a new byte boundary.
	// 0 = opaque, 1 = transparent
	rowBytes := (width + 7) / 8
	buf := make([]byte, rowBytes)
	for y := range height {
		for i := range buf {
			buf[i] = 0
		}

		for x := range width {
			_, _, _, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			if a < 0x8000 {
				buf[x/8] |= 1 << (7 - x%8)
			}
		}

		_, err := w.Write(buf)
		if err != nil {
			return err
		}
	}
	return nil
}

// ExtractMask extracts an image mask from a PDF stream.  ColorSpace and
// Mask entries, which are not allowed for image masks, are ignored.
func ExtractMask(c pdf.Cursor, obj pdf.Object, _ bool) (*Mask, error) {
	stream, err := c.Stream(obj)
	if err != nil {
		return nil, err
	} else if stream == nil {
		return nil, pdf.Error("missing image mask stream")
	}
	dict := stream.Dict

	// Check Type and Subtype
	typeName, _ := c.DictTyped(obj, "XObject")
	if typeName == nil {
		// Type is optional, but if present must be XObject
		if t, err := pdf.Optional(c.Name(dict["Type"])); err != nil {
			return nil, err
		} else if t != "" && t != "XObject" {
			return nil, pdf.Errorf("invalid Type %q for image mask XObject", t)
		}
	}

	subtypeName, err := pdf.Optional(c.Name(dict["Subtype"]))
	if err != nil {
		return nil, err
	}
	if subtypeName != "Image" {
		return nil, pdf.Errorf("invalid Subtype %q for image mask XObject", subtypeName)
	}

	// Check ImageMask flag
	isImageMask, _, err := c.OptionalBoolean(dict["ImageMask"])
	if err != nil {
		return nil, err
	}
	if !isImageMask {
		return nil, pdf.Error("ImageMask flag not set for image mask")
	}

	// Extract required fields
	width, err := c.Integer(dict["Width"])
	if err != nil {
		return nil, fmt.Errorf("missing or invalid Width: %w", err)
	}
	if width <= 0 {
		return nil, pdf.Errorf("invalid image mask width %d", width)
	}
	if width > limits.MaxImageWidth {
		return nil, pdf.Errorf("image mask width %d exceeds limit", width)
	}

	height, err := c.Integer(dict["Height"])
	if err != nil {
		return nil, fmt.Errorf("missing or invalid Height: %w", err)
	}
	if height <= 0 {
		return nil, pdf.Errorf("invalid image mask height %d", height)
	}
	if height > limits.MaxImageHeight {
		return nil, pdf.Errorf("image mask height %d exceeds limit", height)
	}
	if limits.ImagePixelsExceedLimit(int(width), int(height)) {
		return nil, pdf.Error("image mask pixel count exceeds limit")
	}

	mask := &Mask{
		Width:  int(width),
		Height: int(height),
	}

	filters, err := pdf.Optional(c.Filters(dict))
	if err != nil {
		return nil, err
	}
	jpxIndex := jpxFilterIndex(filters)

	// BitsPerComponent is optional for image masks, but if present must
	// be 1.  For JPXDecode masks the entry is ignored.
	if bpc, err := pdf.Optional(c.Integer(dict["BitsPerComponent"])); err != nil {
		return nil, err
	} else if jpxIndex < 0 && bpc > 0 && bpc != 1 {
		return nil, pdf.Errorf("invalid BitsPerComponent %d for image mask (must be 1)", bpc)
	}

	// Decode must be [0 1] or [1 0] for image masks
	if decodeArray, err := pdf.Optional(c.Array(dict["Decode"])); err != nil {
		return nil, err
	} else if len(decodeArray) == 2 {
		// a Decode array with a malformed element is dropped
		d0, err := c.Number(decodeArray[0])
		if pdf.IsReadError(err) {
			return nil, err
		}
		d1, err2 := c.Number(decodeArray[1])
		if pdf.IsReadError(err2) {
			return nil, err2
		}
		if err == nil && err2 == nil && d0 == 1 && d1 == 0 {
			mask.Inverted = true
		}
	}

	interp, _, err := c.OptionalBoolean(dict["Interpolate"])
	if err != nil {
		return nil, err
	}
	mask.Interpolate = interp

	// drop the whole Alternates list if it exceeds MaxAlternates rather
	// than silently truncate
	if alts, err := pdf.Optional(c.Array(dict["Alternates"])); err != nil {
		return nil, err
	} else if len(alts) <= limits.MaxAlternates {
		for i, altObj := range alts {
			alt, err := c.DecodeOptional(altObj, ExtractAlternate)
			if err != nil {
				return nil, fmt.Errorf("invalid Alternates[%d]: %w", i, err)
			}
			if alt != nil {
				mask.Alternates = append(mask.Alternates, alt)
			}
		}
	}

	if name, err := pdf.Optional(c.Name(dict["Name"])); err != nil {
		return nil, err
	} else if c.Version() < pdf.V2_0 { // Name is deprecated in PDF 2.0
		mask.Name = name
	}

	// Extract Metadata
	if meta, err := c.DecodeOptional(dict["Metadata"], pdf.ExtractMetadataStream); err != nil {
		return nil, err
	} else {
		mask.Metadata = meta
	}

	if oc, err := c.DecodeOptional(dict["OC"], oc.ExtractConditional); err != nil {
		return nil, err
	} else {
		mask.OptionalContent = oc
	}

	// Extract Measure
	if m, err := c.DecodeOptional(dict["Measure"], measure.Extract); err != nil {
		return nil, err
	} else {
		mask.Measure = m
	}

	// Extract PtData
	if ptData, err := c.DecodeOptional(dict["PtData"], measure.ExtractPtData); err != nil {
		return nil, err
	} else {
		mask.PtData = ptData
	}

	mask.AssociatedFiles, err = file.ExtractAssociatedFiles(c, dict["AF"])
	if err != nil {
		return nil, err
	}

	// Extract WebCaptureID (ID)
	if webID, err := c.DecodeOptional(dict["ID"], webcapture.ExtractIdentifier); err != nil {
		return nil, err
	} else if webID != nil {
		mask.WebCaptureID = webID
	}

	// Extract StructParent
	if key, err := c.Integer(dict["StructParent"]); err == nil {
		if key >= 0 && uint64(key) <= math.MaxUint {
			mask.StructParent.Set(uint(key))
		}
	} else if pdf.IsReadError(err) {
		return nil, err
	}

	mask.Source, err = newStreamData(c, stream, jpxIndex,
		imageMaskDataLimit(mask.Width, mask.Height))
	if err != nil {
		return nil, err
	}

	return mask, nil
}

// Embed adds the mask to the PDF file and returns the embedded object.
func (m *Mask) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := m.check(rm.Out()); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype":   pdf.Name("Image"),
		"Width":     pdf.Integer(m.Width),
		"Height":    pdf.Integer(m.Height),
		"ImageMask": pdf.Boolean(true),
	}
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("XObject")
	}
	if m.Interpolate {
		dict["Interpolate"] = pdf.Boolean(true)
	}
	if m.Inverted {
		dict["Decode"] = pdf.Array{pdf.Integer(1), pdf.Integer(0)}
	}
	// Default [0 1] is omitted as per PDF spec
	if len(m.Alternates) > 0 {
		var alts pdf.Array
		for _, alt := range m.Alternates {
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

	if m.Name != "" {
		dict["Name"] = m.Name
	}

	if m.Metadata != nil {
		ref, err := rm.Embed(m.Metadata)
		if err != nil {
			return nil, err
		}
		dict["Metadata"] = ref
	}

	if m.OptionalContent != nil {
		if err := pdf.CheckVersion(rm.Out(), "ImageMask OC entry", pdf.V1_5); err != nil {
			return nil, err
		}
		embedded, err := rm.Embed(m.OptionalContent)
		if err != nil {
			return nil, err
		}
		dict["OC"] = embedded
	}

	if m.Measure != nil {
		if err := pdf.CheckVersion(rm.Out(), "image mask Measure entry", pdf.V2_0); err != nil {
			return nil, err
		}
		embedded, err := rm.Embed(m.Measure)
		if err != nil {
			return nil, err
		}
		dict["Measure"] = embedded
	}

	// PtData (optional; PDF 2.0)
	if m.PtData != nil {
		if err := pdf.CheckVersion(rm.Out(), "image mask PtData entry", pdf.V2_0); err != nil {
			return nil, err
		}
		embedded, err := rm.Embed(m.PtData)
		if err != nil {
			return nil, err
		}
		dict["PtData"] = embedded
	}

	if len(m.AssociatedFiles) > 0 {
		if err := pdf.CheckVersion(rm.Out(), "image mask AF entry", pdf.V2_0); err != nil {
			return nil, err
		}

		// Validate each file specification can be used as associated file
		version := pdf.GetVersion(rm.Out())
		for i, spec := range m.AssociatedFiles {
			if spec == nil {
				continue
			}
			if err := spec.CanBeAF(version); err != nil {
				return nil, fmt.Errorf("AssociatedFiles[%d]: %w", i, err)
			}
		}

		// Embed the file specifications
		var afArray pdf.Array
		for _, spec := range m.AssociatedFiles {
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

	if m.WebCaptureID != nil {
		if err := pdf.CheckVersion(rm.Out(), "image mask ID entry", pdf.V1_3); err != nil {
			return nil, err
		}
		embedded, err := rm.Embed(m.WebCaptureID)
		if err != nil {
			return nil, err
		}
		dict["ID"] = embedded
	}

	if key, ok := m.StructParent.Get(); ok {
		if err := pdf.CheckVersion(rm.Out(), "image mask StructParent entry", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["StructParent"] = pdf.Integer(key)
	}

	ref := rm.Alloc()
	if err := m.Source.WriteStream(rm, ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// LoadAlpha decodes the 1-bit mask data and returns an alpha image.
// Opaque pixels get alpha 255, transparent pixels get alpha 0.
// The [Mask.Inverted] flag controls which bit value means opaque.
//
// For JPXDecode masks, the size of the result is taken from the JPEG 2000
// data, which must have a single colour channel with 1-bit samples.
func (m *Mask) LoadAlpha() (*image.Alpha, error) {
	raw, err := m.Source.Pixels()
	if err != nil {
		return nil, err
	}
	if m.Source.IsJPX() {
		return m.loadAlphaJPX(raw)
	}

	width, height := m.Width, m.Height
	raw = normalizeData(raw, expectedDataSize(width, 1, 1, height))

	alpha := image.NewAlpha(image.Rect(0, 0, width, height))
	for y := range height {
		rowBytes := (width + 7) / 8
		rowStart := y * rowBytes
		for x := range width {
			bit := (raw[rowStart+x/8] >> (7 - x%8)) & 1
			opaque := bit == 0
			if m.Inverted {
				opaque = bit == 1
			}
			if opaque {
				alpha.Pix[y*alpha.Stride+x] = 255
			}
		}
	}
	return alpha, nil
}

// loadAlphaJPX implements [Mask.LoadAlpha] for JPXDecode masks.
//
// Samples of other depths are rejected rather than reduced to one bit:
// the spec gives stencil masks 1-bit samples only, and any threshold
// would invent a meaning the file does not state.
func (m *Mask) loadAlphaJPX(raw []byte) (*image.Alpha, error) {
	im, err := decodeJPXMask(raw)
	if err != nil {
		return nil, err
	}
	if im.BitsPerComponent != 1 {
		return nil, fmt.Errorf("JPEG 2000 stencil mask has %d-bit samples", im.BitsPerComponent)
	}

	var opaqueBit int32
	if m.Inverted {
		opaqueBit = 1
	}
	alpha := image.NewAlpha(image.Rect(0, 0, im.Width, im.Height))
	for i, v := range im.Color[0] {
		if v == opaqueBit {
			alpha.Pix[i] = 255
		}
	}
	return alpha, nil
}

func (m *Mask) check(out *pdf.Writer) error {
	if m.Width <= 0 {
		return fmt.Errorf("invalid image mask width %d", m.Width)
	}
	if m.Height <= 0 {
		return fmt.Errorf("invalid image mask height %d", m.Height)
	}
	if m.Source == nil {
		return errors.New("source cannot be nil")
	}
	if s, ok := m.Source.(graphics.GeometryChecker); ok {
		bpc := 1
		if m.Source.IsJPX() {
			bpc = 0 // the depth is given by the JPEG 2000 data
		}
		if err := s.CheckGeometry(m.Width, m.Height, 1, bpc); err != nil {
			return err
		}
	}
	if s, ok := m.Source.(*JPXSource); ok {
		if err := s.checkMask(1); err != nil {
			return err
		}
	}

	if m.Alternates != nil {
		if err := pdf.CheckVersion(out, "image alternates", pdf.V1_3); err != nil {
			return err
		}
		defaultForPrintingCount := 0
		for _, alt := range m.Alternates {
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
	case v == pdf.V1_0 && m.Name == "":
		return errors.New("missing image mask /Name field")
	case v >= pdf.V2_0 && m.Name != "":
		return errors.New("unexpected /Name field")
	}
	if m.Metadata != nil {
		if err := pdf.CheckVersion(out, "image metadata", pdf.V1_4); err != nil {
			return err
		}
	}

	if m.Measure != nil {
		if err := pdf.CheckVersion(out, "image mask Measure entry", pdf.V2_0); err != nil {
			return err
		}
	}

	// Validate AssociatedFiles
	if len(m.AssociatedFiles) > 0 {
		if err := pdf.CheckVersion(out, "image mask AssociatedFiles entry", pdf.V2_0); err != nil {
			return err
		}

		version := pdf.GetVersion(out)
		for i, spec := range m.AssociatedFiles {
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

// Bounds returns the dimensions of the mask.
func (m *Mask) Bounds() rect.IntRect {
	return rect.IntRect{
		XMin: 0,
		YMin: 0,
		XMax: m.Width,
		YMax: m.Height,
	}
}

// Subtype returns the PDF XObject subtype for image masks.
func (m *Mask) Subtype() pdf.Name {
	return pdf.Name("Image")
}

// ResourceName returns the preferred resource-dictionary key for this image
// mask.  See [graphics.XObject.ResourceName].
func (m *Mask) ResourceName() pdf.Name {
	return m.Name
}

// IsImageMask returns true, indicating this is a stencil mask.
// This implements the [graphics.ImageMask] interface.
func (m *Mask) IsImageMask() bool {
	return true
}
