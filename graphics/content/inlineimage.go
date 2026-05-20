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

package content

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/streamlimits"
)

// inlineFilterAbbreviations maps abbreviated inline image filter names
// (PDF 2.0, Table 90) to their full names.
var inlineFilterAbbreviations = map[pdf.Name]pdf.Name{
	"AHx": "ASCIIHexDecode",
	"A85": "ASCII85Decode",
	"LZW": "LZWDecode",
	"Fl":  "FlateDecode",
	"RL":  "RunLengthDecode",
	"CCF": "CCITTFaxDecode",
	"DCT": "DCTDecode",
}

// inlineFilterForbidden lists filters that PDF 2.0 §8.9.7 prohibits in inline
// images.  Names are the full forms; abbreviated keys are mapped to their
// full forms before the lookup.
var inlineFilterForbidden = map[pdf.Name]bool{
	"JBIG2Decode": true,
	"JPXDecode":   true,
	"Crypt":       true,
}

// ValidateInlineImageFilter reports an error if the Filter (or F) entry of an
// inline image dict names a filter that PDF 2.0 §8.9.7 prohibits in inline
// images: JBIG2Decode, JPXDecode, or Crypt.  Other entries of the dict are
// not inspected.
func ValidateInlineImageFilter(dict pdf.Dict) error {
	filter, ok := dict["F"]
	if !ok {
		filter = dict["Filter"]
	}
	switch f := filter.(type) {
	case pdf.Name:
		if inlineFilterForbidden[f] {
			return fmt.Errorf("filter %s not permitted in inline image", f)
		}
	case pdf.Array:
		for _, elem := range f {
			name, ok := elem.(pdf.Name)
			if ok && inlineFilterForbidden[name] {
				return fmt.Errorf("filter %s not permitted in inline image", name)
			}
		}
	}
	return nil
}

// DecodeInlineImage decompresses the image data from an inline image operator.
// The operator must have name [OpInlineImage] with two arguments:
// a [pdf.Dict] containing image parameters and a [pdf.String] holding the
// raw image data.  res is the parsed resource dictionary of the surrounding
// content stream; it is used to resolve named colour spaces.
func DecodeInlineImage(op Operator, res *Resources) ([]byte, error) {
	if op.Name != OpInlineImage {
		return nil, fmt.Errorf("expected %s operator, got %s", OpInlineImage, op.Name)
	}
	if len(op.Args) < 2 {
		return nil, fmt.Errorf("inline image: expected 2 arguments, got %d", len(op.Args))
	}

	dict, ok := op.Args[0].(pdf.Dict)
	if !ok {
		return nil, fmt.Errorf("inline image: expected Dict, got %T", op.Args[0])
	}
	rawData, ok := op.Args[1].(pdf.String)
	if !ok {
		return nil, fmt.Errorf("inline image: expected String, got %T", op.Args[1])
	}

	if err := checkInlineImageDimensions(dict, res); err != nil {
		return nil, err
	}
	if err := ValidateInlineImageFilter(dict); err != nil {
		return nil, &pdf.MalformedFileError{Err: err}
	}
	sizeLimit := inlineImageSizeLimit(dict, res)

	data := []byte(rawData)

	// extract filter name(s)
	filterObj := dict["F"]
	if filterObj == nil {
		filterObj = dict["Filter"]
	}
	if filterObj == nil {
		return data, nil
	}

	// extract decode parameters
	parmsObj := dict["DP"]
	if parmsObj == nil {
		parmsObj = dict["DecodeParms"]
	}

	type filterSpec struct {
		name  pdf.Name
		parms pdf.Dict
	}

	var filters []filterSpec
	switch f := filterObj.(type) {
	case pdf.Name:
		if full, ok := inlineFilterAbbreviations[f]; ok {
			f = full
		}
		var pDict pdf.Dict
		if d, ok := parmsObj.(pdf.Dict); ok {
			pDict = d
		}
		filters = append(filters, filterSpec{f, pDict})
	case pdf.Array:
		parmsArr, _ := parmsObj.(pdf.Array)
		for i, elem := range f {
			name, ok := elem.(pdf.Name)
			if !ok {
				return nil, fmt.Errorf("inline image: filter element %d: expected Name, got %T", i, elem)
			}
			if full, ok := inlineFilterAbbreviations[name]; ok {
				name = full
			}
			var pDict pdf.Dict
			if i < len(parmsArr) {
				if d, ok := parmsArr[i].(pdf.Dict); ok {
					pDict = d
				}
			}
			filters = append(filters, filterSpec{name, pDict})
		}
	default:
		return nil, fmt.Errorf("inline image: unexpected filter type %T", filterObj)
	}

	// per-decode working-memory budget, sized to the raw inline data
	budget := membudget.New(streamlimits.StreamBudget(int64(len(data))))

	// chain filters
	var r io.Reader = bytes.NewReader(data)
	var closers []io.Closer
	for _, fs := range filters {
		f, err := pdf.MakeFilter(fs.name, fs.parms)
		if err != nil {
			return nil, err
		}
		rc, err := f.Decode(pdf.V2_0, r, budget)
		if err != nil {
			return nil, err
		}
		closers = append(closers, rc)
		r = rc
	}

	result, err := io.ReadAll(io.LimitReader(r, sizeLimit+1))
	for i := len(closers) - 1; i >= 0; i-- {
		if cerr := closers[i].Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		return nil, fmt.Errorf("inline image: reading decompressed data: %w", err)
	}
	if int64(len(result)) > sizeLimit {
		return nil, &pdf.MalformedFileError{Err: errors.New("inline image data exceeds size limit")}
	}
	return result, nil
}

// InlineImageColorSpace resolves the ColorSpace of an inline image dict.
// It handles the three forms allowed by PDF 2.0 §8.9.7: device colour space
// names (with abbreviations), the limited Indexed array form whose base is
// a device space, and resource references resolved through res.ColorSpace.
// It returns nil when CS is missing, malformed, or refers to a name that
// is not present in res.
func InlineImageColorSpace(dict pdf.Dict, res *Resources) color.Space {
	var val pdf.Object
	if v, ok := dict["CS"]; ok {
		val = v
	} else if v, ok := dict["ColorSpace"]; ok {
		val = v
	}
	switch v := val.(type) {
	case nil:
		return nil
	case pdf.Name:
		if cs := color.ParseInlineDeviceName(v); cs != nil {
			return cs
		}
		if res != nil {
			return res.ColorSpace[v]
		}
		return nil
	case pdf.Array:
		return color.ParseInlineIndexed(v)
	}
	return nil
}

// checkInlineImageDimensions rejects inline images whose W/H values exceed
// [streamlimits.MaxImageWidth] or [streamlimits.MaxImageHeight], whose
// pixel count exceeds [streamlimits.MaxImagePixels], or whose declared
// pixel data exceeds [streamlimits.MaxImageBytes].  These checks match
// the dimension and product caps enforced for image XObjects.
func checkInlineImageDimensions(dict pdf.Dict, res *Resources) error {
	width := getInlineImageInt(dict, "W", "Width")
	height := getInlineImageInt(dict, "H", "Height")
	if width > streamlimits.MaxImageWidth {
		return &pdf.MalformedFileError{Err: fmt.Errorf("inline image width %d exceeds limit", width)}
	}
	if height > streamlimits.MaxImageHeight {
		return &pdf.MalformedFileError{Err: fmt.Errorf("inline image height %d exceeds limit", height)}
	}
	if streamlimits.ImagePixelsExceedLimit(width, height) {
		return &pdf.MalformedFileError{Err: errors.New("inline image pixel count exceeds limit")}
	}

	// byte-count refinement when ColorSpace and BPC are known
	channels, bpc := 0, 0
	if isImageMask(dict) {
		channels, bpc = 1, 1
	} else {
		bpc = getInlineImageInt(dict, "BPC", "BitsPerComponent")
		if cs := InlineImageColorSpace(dict, res); cs != nil {
			channels = cs.Channels()
		}
	}
	if streamlimits.ImageBytesExceedLimit(width, height, channels, bpc) {
		return &pdf.MalformedFileError{Err: errors.New("inline image data exceeds size limit")}
	}
	return nil
}

// inlineImageSizeLimit computes the per-image decoded-size cap for an inline
// image dict.  Keys may use abbreviated (W/H/BPC/CS/IM) or full
// (Width/Height/BitsPerComponent/ColorSpace/ImageMask) names per Table 90.
func inlineImageSizeLimit(dict pdf.Dict, res *Resources) int64 {
	width := getInlineImageInt(dict, "W", "Width")
	height := getInlineImageInt(dict, "H", "Height")
	if width <= 0 || height <= 0 {
		return streamlimits.MaxImageBytes
	}

	// image masks are always 1 bpc, 1 channel
	if isImageMask(dict) {
		return streamlimits.ImageDataLimit(width, height, 1, 1)
	}

	bpc := getInlineImageInt(dict, "BPC", "BitsPerComponent")
	if bpc <= 0 {
		return streamlimits.MaxImageBytes
	}
	cs := InlineImageColorSpace(dict, res)
	if cs == nil {
		return streamlimits.MaxImageBytes
	}
	channels := cs.Channels()
	if channels <= 0 {
		return streamlimits.MaxImageBytes
	}
	return streamlimits.ImageDataLimit(width, height, channels, bpc)
}

// isImageMask reports whether the dict describes an image mask.
func isImageMask(dict pdf.Dict) bool {
	var val pdf.Object
	if v, ok := dict["IM"]; ok {
		val = v
	} else if v, ok := dict["ImageMask"]; ok {
		val = v
	}
	b, ok := val.(pdf.Boolean)
	return ok && bool(b)
}
