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

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/measure"
	"seehuhn.de/go/pdf/metadata"
	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/optional"
	"seehuhn.de/go/pdf/webcapture"
)

// PDF 2.0 sections: 8.9.5 8.9.6

type Mask struct {
	// Width is the width of the image mask in pixels.
	Width int

	// Height is the height of the image mask in pixels.
	Height int

	// Inverted indicates the meaning of individual bits in the image data:
	//   - false: 0=opaque and 1=transparent
	//   - true: 1=opaque and 0=transparent
	Inverted bool

	// WriteData is a function that writes the mask data to the provided writer.
	// The data should be written as a continuous bit stream, with each row
	// starting at a new byte boundary. 0 = opaque, 1 = transparent.
	WriteData func(io.Writer) error

	// Interpolate enables edge smoothing for the mask to reduce jagged
	// appearance in low-resolution stencil masks.
	Interpolate bool

	// Alternates (optional) is an array of alternate image dictionaries for this mask.
	Alternates []*Mask

	// OptionalContent (optional) allows to control the visibility of the mask.
	OptionalContent oc.Conditional

	// StructParent (required if the image mask is a structural content item)
	// is the integer key of the image mask's entry in the structural parent tree.
	StructParent optional.Int

	// Metadata (optional) is a metadata stream containing metadata for the image.
	Metadata *metadata.Stream

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

	// Name is deprecated and should be left empty.
	// Only used in PDF 1.0 where it was the name used to reference the image
	// from within content streams.
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
		WriteData: func(w io.Writer) error {
			return writeImageMaskData(w, img)
		},
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

// ExtractMask extracts an image mask from a PDF stream.
func ExtractMask(x *pdf.Extractor, obj pdf.Object) (*Mask, error) {
	stream, err := x.GetStream(obj)
	if err != nil {
		return nil, err
	} else if stream == nil {
		return nil, pdf.Error("missing image mask stream")
	}
	dict := stream.Dict

	// Check Type and Subtype
	typeName, _ := x.GetDictTyped(obj, "XObject")
	if typeName == nil {
		// Type is optional, but if present must be XObject
		if t, err := pdf.Optional(x.GetName(dict["Type"])); err != nil {
			return nil, err
		} else if t != "" && t != "XObject" {
			return nil, pdf.Errorf("invalid Type %q for image mask XObject", t)
		}
	}

	subtypeName, err := pdf.Optional(x.GetName(dict["Subtype"]))
	if err != nil {
		return nil, err
	}
	if subtypeName != "Image" {
		return nil, pdf.Errorf("invalid Subtype %q for image mask XObject", subtypeName)
	}

	// Check ImageMask flag
	isImageMask, err := x.GetBoolean(dict["ImageMask"])
	if err != nil || !bool(isImageMask) {
		return nil, pdf.Error("ImageMask flag not set for image mask")
	}

	// Extract required fields
	width, err := x.GetInteger(dict["Width"])
	if err != nil {
		return nil, fmt.Errorf("missing or invalid Width: %w", err)
	}
	if width <= 0 {
		return nil, pdf.Errorf("invalid image mask width %d", width)
	}

	height, err := x.GetInteger(dict["Height"])
	if err != nil {
		return nil, fmt.Errorf("missing or invalid Height: %w", err)
	}
	if height <= 0 {
		return nil, pdf.Errorf("invalid image mask height %d", height)
	}

	mask := &Mask{
		Width:  int(width),
		Height: int(height),
	}

	// BitsPerComponent is optional for image masks, but if present must be 1
	if bpc, err := pdf.Optional(x.GetInteger(dict["BitsPerComponent"])); err != nil {
		return nil, err
	} else if bpc > 0 && bpc != 1 {
		return nil, pdf.Errorf("invalid BitsPerComponent %d for image mask (must be 1)", bpc)
	}

	// ColorSpace must not be present for image masks
	if _, hasColorSpace := dict["ColorSpace"]; hasColorSpace {
		return nil, pdf.Error("ColorSpace not allowed for image masks")
	}

	// Mask entry must not be present for image masks
	if _, hasMask := dict["Mask"]; hasMask {
		return nil, pdf.Error("Mask not allowed for image masks")
	}

	// Extract Decode array (for image masks, must be [0 1] or [1 0])
	if decodeArray, err := pdf.Optional(x.GetArray(dict["Decode"])); err != nil {
		return nil, err
	} else if len(decodeArray) == 2 {
		d0, _ := x.GetNumber(decodeArray[0])
		d1, _ := x.GetNumber(decodeArray[1])
		if d0 == 1 && d1 == 0 {
			mask.Inverted = true
		}
	}

	if interp, err := x.GetBoolean(dict["Interpolate"]); err == nil {
		mask.Interpolate = bool(interp)
	}

	if alts, err := pdf.Optional(x.GetArray(dict["Alternates"])); err != nil {
		return nil, err
	} else if alts != nil {
		mask.Alternates = make([]*Mask, len(alts))
		for i, altObj := range alts {
			altMask, err := pdf.ExtractorGetOptional(x, altObj, ExtractMask)
			if err != nil {
				return nil, fmt.Errorf("invalid Alternates[%d]: %w", i, err)
			}
			mask.Alternates[i] = altMask
		}
	}

	if name, err := pdf.Optional(x.GetName(dict["Name"])); err != nil {
		return nil, err
	} else if pdf.GetVersion(x.R) < pdf.V2_0 { // Name is deprecated in PDF 2.0
		mask.Name = name
	}

	// Extract Metadata
	if metaObj, ok := dict["Metadata"]; ok {
		meta, err := metadata.Extract(x.R, metaObj)
		if err != nil {
			return nil, fmt.Errorf("invalid Metadata: %w", err)
		}
		mask.Metadata = meta
	}

	if oc, err := pdf.ExtractorGetOptional(x, dict["OC"], oc.ExtractConditional); err != nil {
		return nil, err
	} else {
		mask.OptionalContent = oc
	}

	// Extract Measure
	if measureObj, ok := dict["Measure"]; ok {
		m, err := measure.Extract(x, measureObj)
		if err != nil {
			return nil, fmt.Errorf("invalid Measure: %w", err)
		}
		mask.Measure = m
	}

	// Extract PtData
	if ptData, err := pdf.Optional(measure.ExtractPtData(x, dict["PtData"])); err != nil {
		return nil, err
	} else {
		mask.PtData = ptData
	}

	// Extract AssociatedFiles (AF)
	if afArray, err := pdf.Optional(x.GetArray(dict["AF"])); err != nil {
		return nil, err
	} else if afArray != nil {
		mask.AssociatedFiles = make([]*file.Specification, 0, len(afArray))
		for _, afObj := range afArray {
			if spec, err := pdf.ExtractorGetOptional(x, afObj, file.ExtractSpecification); err != nil {
				return nil, err
			} else if spec != nil {
				mask.AssociatedFiles = append(mask.AssociatedFiles, spec)
			}
		}
	}

	// Extract WebCaptureID (ID)
	if webID, err := pdf.ExtractorGetOptional(x, dict["ID"], webcapture.ExtractIdentifier); err != nil {
		return nil, err
	} else if webID != nil {
		mask.WebCaptureID = webID
	}

	// Extract StructParent
	if keyObj := dict["StructParent"]; keyObj != nil {
		if key, err := pdf.Optional(x.GetInteger(dict["StructParent"])); err != nil {
			return nil, err
		} else {
			mask.StructParent.Set(key)
		}
	}

	// Create WriteData function as a closure
	mask.WriteData = func(w io.Writer) error {
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

	return mask, nil
}

// Embed adds the mask to the PDF file and returns the embedded object.
func (m *Mask) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	if err := m.check(rm.Out()); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Type":      pdf.Name("XObject"),
		"Subtype":   pdf.Name("Image"),
		"Width":     pdf.Integer(m.Width),
		"Height":    pdf.Integer(m.Height),
		"ImageMask": pdf.Boolean(true),
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
		dict["StructParent"] = key
	}

	ref := rm.Alloc()
	filters := []pdf.Filter{
		pdf.FilterCCITTFax{
			"Columns": pdf.Integer(m.Width),
			"K":       pdf.Integer(-1),
		},
	}
	w, err := rm.Out().OpenStream(ref, dict, filters...)
	if err != nil {
		return nil, fmt.Errorf("cannot open image mask stream: %w", err)
	}

	err = m.WriteData(w)
	if err != nil {
		return nil, err
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}
	return ref, nil
}

func (m *Mask) check(out *pdf.Writer) error {
	if m.Width <= 0 {
		return fmt.Errorf("invalid image mask width %d", m.Width)
	}
	if m.Height <= 0 {
		return fmt.Errorf("invalid image mask height %d", m.Height)
	}
	if m.WriteData == nil {
		return errors.New("WriteData function cannot be nil")
	}

	if m.Alternates != nil {
		if err := pdf.CheckVersion(out, "image alternates", pdf.V1_3); err != nil {
			return err
		}

		for _, alt := range m.Alternates {
			if len(alt.Alternates) > 0 {
				return errors.New("alternates of alternates not allowed")
			}
		}
	}
	if m.Name != "" {
		v := pdf.GetVersion(out)
		if v >= pdf.V2_0 {
			return errors.New("unexpected /Name field")
		}
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

// IsImageMask returns true, indicating this is a stencil mask.
// This implements the [graphics.ImageMask] interface.
func (m *Mask) IsImageMask() bool {
	return true
}
