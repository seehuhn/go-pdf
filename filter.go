// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package pdf

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"sync"

	"seehuhn.de/go/pdf/internal/filter/ascii85"
	"seehuhn.de/go/pdf/internal/filter/asciihex"
	"seehuhn.de/go/pdf/internal/filter/ccittfax"
	"seehuhn.de/go/pdf/internal/filter/dct"
	"seehuhn.de/go/pdf/internal/filter/jbig2"
	"seehuhn.de/go/pdf/internal/filter/lzw"
	"seehuhn.de/go/pdf/internal/filter/predict"
	"seehuhn.de/go/pdf/internal/filter/runlength"
	"seehuhn.de/go/pdf/internal/streamlimits"
)

// Frequencies of filter types used in the PDF files on my system:
//     165622 FlateDecode
//      11334 CCITTFaxDecode
//       7595 DCTDecode
//       3440 LZWDecode
//       3431 ASCII85Decode
//        455 JBIG2Decode
//        166 ASCIIHexDecode
//         78 JPXDecode
//          5 RunLengthDecode

// Filter represents a PDF stream filter.
//
// Currently, the following filter types are implemented by this library:
// [FilterASCII85], [FilterASCIIHex], [FilterCCITTFax], [FilterDCT],
// [FilterFlate], [FilterJBIG2], [FilterLZW], [FilterRunLength].
//
// The [FilterDCT] and [FilterJBIG2] filters support decoding only.
// The [FilterJPX] type is present so that PDF files using this filter
// survive a read/write cycle, but encoding and decoding through the
// filter interface are not supported.
//
// The Crypt filter is represented by three variants implementing
// [CryptFilter]: [FilterCryptIdentity], [FilterCryptStandard], and
// [FilterCryptNamed].  Of these, only [FilterCryptIdentity] is
// implemented end-to-end; the other two round-trip the wire form but
// do not yet support encoding or decoding.
//
// In addition, [FilterCompress] can be used to select the best available
// general compression filter when writing PDF streams.  This is FilterFlate
// for PDF versions 1.2 and above, and FilterLZW for older versions.
type Filter interface {
	// Info returns the name and parameters of the filter,
	// as they should be written to the PDF file.
	Info(Version) (Name, Dict, error)

	// Encode returns a writer which encodes data written to it.
	// The returned writer must be closed after use.
	Encode(Version, io.WriteCloser) (io.WriteCloser, error)

	// Decode returns a reader which decodes data read from it.
	Decode(Version, io.Reader) (io.ReadCloser, error)
}

// MakeFilter constructs a [Filter] for the given filter name and parameters.
//
// Parameters are parsed permissively within reason: unknown or out-of-range
// values are silently replaced with PDF defaults so that malformed PDF input
// remains readable.  Cases where there is no safe fix-up — for example, a
// /Crypt filter whose /Name entry has the wrong PDF type — are reported as
// a [*MalformedFileError].
func MakeFilter(filter Name, param Dict) (Filter, error) {
	switch filter {
	case "ASCII85Decode":
		return FilterASCII85{}, nil
	case "ASCIIHexDecode":
		return FilterASCIIHex{}, nil
	case "RunLengthDecode":
		return FilterRunLength{}, nil
	case "FlateDecode":
		return parseFlate(param), nil
	case "LZWDecode":
		return parseLZW(param), nil
	case "CCITTFaxDecode":
		return parseCCITTFax(param), nil
	case "DCTDecode":
		return parseDCT(param), nil
	case "JBIG2Decode":
		return parseJBIG2(param), nil
	case "JPXDecode":
		return FilterJPX{}, nil
	case "Crypt":
		return parseCrypt(param)
	default:
		return &filterNotImplemented{Name: filter, Param: param}, nil
	}
}

// parseCrypt constructs the appropriate [CryptFilter] variant from a
// /Crypt filter's /DecodeParms dict.  Unlike the other parse* helpers,
// this one returns an error when the /Name entry is present with a
// non-Name PDF type: there is no safe default fix-up — picking Identity
// would silently mask a malformed file that may have intended an
// encrypted recipe, and picking a named CF would invent a name not
// found in the file.
func parseCrypt(param Dict) (CryptFilter, error) {
	var name Name
	if val, present := param["Name"]; present {
		n, ok := val.(Name)
		if !ok {
			return nil, &MalformedFileError{
				Err: fmt.Errorf("Crypt filter: /Name has wrong type %T", val),
			}
		}
		name = n
	}
	switch name {
	case "", "Identity":
		return FilterCryptIdentity{}, nil
	case "StdCF":
		return FilterCryptStandard{}, nil
	default:
		return FilterCryptNamed{Name: name}, nil
	}
}

type filterNotImplemented struct {
	Name  Name
	Param Dict
}

func (f *filterNotImplemented) Info(Version) (Name, Dict, error) {
	return f.Name, f.Param, nil
}

func (f *filterNotImplemented) Encode(Version, io.WriteCloser) (io.WriteCloser, error) {
	return nil, fmt.Errorf("filter %s not implemented", f.Name)
}

func (f *filterNotImplemented) Decode(Version, io.Reader) (io.ReadCloser, error) {
	return nil, Errorf("filter %s not implemented", f.Name)
}

// FilterASCII85 is the ASCII85Decode filter.
// This filter has no parameters.
type FilterASCII85 struct{}

// Info implements the [Filter] interface.
func (f FilterASCII85) Info(_ Version) (Name, Dict, error) {
	return "ASCII85Decode", nil, nil
}

// Encode implements the [Filter] interface.
func (f FilterASCII85) Encode(_ Version, w io.WriteCloser) (io.WriteCloser, error) {
	return ascii85.Encode(w, 79), nil
}

// Decode implements the [Filter] interface.
func (f FilterASCII85) Decode(_ Version, r io.Reader) (io.ReadCloser, error) {
	return asMalformedFilter(ascii85.Decode(r), nil)
}

// FilterASCIIHex is the ASCIIHexDecode filter.
// This filter has no parameters.
type FilterASCIIHex struct{}

// Info implements the [Filter] interface.
func (f FilterASCIIHex) Info(_ Version) (Name, Dict, error) {
	return "ASCIIHexDecode", nil, nil
}

// Encode implements the [Filter] interface.
func (f FilterASCIIHex) Encode(_ Version, w io.WriteCloser) (io.WriteCloser, error) {
	return asciihex.Encode(w, 79), nil
}

// Decode implements the [Filter] interface.
func (f FilterASCIIHex) Decode(_ Version, r io.Reader) (io.ReadCloser, error) {
	return asMalformedFilter(asciihex.Decode(r), nil)
}

// FilterRunLength is the RunLengthDecode filter.
// This filter has no parameters.
type FilterRunLength struct{}

// Info implements the [Filter] interface.
func (f FilterRunLength) Info(_ Version) (Name, Dict, error) {
	return "RunLengthDecode", nil, nil
}

// Encode implements the [Filter] interface.
func (f FilterRunLength) Encode(_ Version, w io.WriteCloser) (io.WriteCloser, error) {
	return runlength.Encode(w), nil
}

// Decode implements the [Filter] interface.
func (f FilterRunLength) Decode(_ Version, r io.Reader) (io.ReadCloser, error) {
	return asMalformedFilter(runlength.Decode(r), nil)
}

// FlatePredictor selects the predictor algorithm used by the
// [FilterFlate], [FilterLZW], and [FilterCompress] filters.
//
// The values are taken from PDF spec Table 10.  The Go zero value
// (an unset predictor) is treated as [FlatePredictorNone] when written.
type FlatePredictor int

// Predictor values as defined by PDF spec Table 10.
const (
	FlatePredictorNone       FlatePredictor = 1
	FlatePredictorTIFF       FlatePredictor = 2
	FlatePredictorPNGNone    FlatePredictor = 10
	FlatePredictorPNGSub     FlatePredictor = 11
	FlatePredictorPNGUp      FlatePredictor = 12
	FlatePredictorPNGAverage FlatePredictor = 13
	FlatePredictorPNGPaeth   FlatePredictor = 14
	FlatePredictorPNGOptimum FlatePredictor = 15
)

// isValid reports whether the predictor is one of the values listed
// in PDF spec Table 10 (or the unset zero value).
func (p FlatePredictor) isValid() bool {
	switch p {
	case 0,
		FlatePredictorNone, FlatePredictorTIFF,
		FlatePredictorPNGNone, FlatePredictorPNGSub,
		FlatePredictorPNGUp, FlatePredictorPNGAverage,
		FlatePredictorPNGPaeth, FlatePredictorPNGOptimum:
		return true
	}
	return false
}

// FilterFlate is the FlateDecode filter.
//
// This filter requires PDF version 1.2 or higher.
type FilterFlate struct {
	// Predictor selects the predictor algorithm.
	// On write, 0 can be used as a shorthand for [FlatePredictorNone].
	Predictor FlatePredictor

	// Colors is the number of interleaved colour components per sample.
	// Only meaningful when Predictor selects a predictor other than
	// [FlatePredictorNone]; setting Colors otherwise is a write-time error.
	//
	// On write, 0 can be used as a shorthand for 1.
	Colors int

	// BitsPerComponent is the number of bits used to represent each
	// colour component.  Only meaningful when Predictor selects a
	// predictor other than [FlatePredictorNone]; setting BitsPerComponent
	// otherwise is a write-time error.
	//
	// On write, 0 can be used as a shorthand for 8.
	BitsPerComponent int

	// Columns is the number of samples in each row.  Only meaningful
	// when Predictor selects a predictor other than [FlatePredictorNone];
	// setting Columns otherwise is a write-time error.
	//
	// On write, 0 can be used as a shorthand for 1.
	Columns int
}

// Info implements the [Filter] interface.
func (f FilterFlate) Info(v Version) (Name, Dict, error) {
	if err := f.validate(v); err != nil {
		return "", nil, err
	}
	return "FlateDecode", f.toDict(), nil
}

// Encode implements the [Filter] interface.
func (f FilterFlate) Encode(v Version, w io.WriteCloser) (io.WriteCloser, error) {
	if err := f.validate(v); err != nil {
		return nil, err
	}
	return encodeFlateLZW(w, f.Predictor, f.Colors, f.BitsPerComponent, f.Columns, false, false)
}

// Decode implements the [Filter] interface.
func (f FilterFlate) Decode(_ Version, r io.Reader) (io.ReadCloser, error) {
	return asMalformedFilter(decodeFlateLZW(r, f.Predictor, f.Colors, f.BitsPerComponent, f.Columns, false, false))
}

func (f FilterFlate) validate(v Version) error {
	if v < V1_2 {
		return &VersionError{Operation: "FlateDecode filter", Earliest: V1_2}
	}
	return validateFlateLZW(v, f.Predictor, f.Colors, f.BitsPerComponent, f.Columns)
}

// toDict returns the DecodeParms dictionary, or nil if no entries are needed.
func (f FilterFlate) toDict() Dict {
	res := Dict{}
	usingPredictor := f.Predictor != 0 && f.Predictor != FlatePredictorNone
	if usingPredictor {
		res["Predictor"] = Integer(f.Predictor)
		if f.Colors != 0 && f.Colors != 1 {
			res["Colors"] = Integer(f.Colors)
		}
		if f.BitsPerComponent != 0 && f.BitsPerComponent != 8 {
			res["BitsPerComponent"] = Integer(f.BitsPerComponent)
		}
		if f.Columns != 0 && f.Columns != 1 {
			res["Columns"] = Integer(f.Columns)
		}
	}
	if len(res) == 0 {
		return nil
	}
	return res
}

// FilterLZW is the LZWDecode filter.
//
// This is only useful to read legacy PDF files.  For new files, use
// [FilterFlate] instead.
type FilterLZW struct {
	// Predictor selects the predictor algorithm.
	// On write, 0 can be used as a shorthand for [FlatePredictorNone].
	Predictor FlatePredictor

	// Colors is the number of interleaved colour components per sample.
	// Only meaningful when Predictor selects a predictor other than
	// [FlatePredictorNone]; setting Colors otherwise is a write-time error.
	//
	// On write, 0 can be used as a shorthand for 1.
	Colors int

	// BitsPerComponent is the number of bits used to represent each
	// colour component.  Only meaningful when Predictor selects a
	// predictor other than [FlatePredictorNone]; setting BitsPerComponent
	// otherwise is a write-time error.
	//
	// On write, 0 can be used as a shorthand for 8.
	BitsPerComponent int

	// Columns is the number of samples in each row.  Only meaningful
	// when Predictor selects a predictor other than [FlatePredictorNone];
	// setting Columns otherwise is a write-time error.
	//
	// On write, 0 can be used as a shorthand for 1.
	Columns int

	// OffByOne selects which variant of the LZW algorithm is used.
	// The Go zero value (false) selects the corrected algorithm
	// (PDF EarlyChange=0); set to true to use the legacy off-by-one
	// variant (PDF EarlyChange=1, the PDF default).
	OffByOne bool
}

// Info implements the [Filter] interface.
func (f FilterLZW) Info(v Version) (Name, Dict, error) {
	if err := f.validate(v); err != nil {
		return "", nil, err
	}
	dict := FilterFlate{
		Predictor:        f.Predictor,
		Colors:           f.Colors,
		BitsPerComponent: f.BitsPerComponent,
		Columns:          f.Columns,
	}.toDict()
	if !f.OffByOne {
		if dict == nil {
			dict = Dict{}
		}
		dict["EarlyChange"] = Integer(0)
	}
	return "LZWDecode", dict, nil
}

// Encode implements the [Filter] interface.
func (f FilterLZW) Encode(v Version, w io.WriteCloser) (io.WriteCloser, error) {
	if err := f.validate(v); err != nil {
		return nil, err
	}
	return encodeFlateLZW(w, f.Predictor, f.Colors, f.BitsPerComponent, f.Columns, true, f.OffByOne)
}

// Decode implements the [Filter] interface.
func (f FilterLZW) Decode(_ Version, r io.Reader) (io.ReadCloser, error) {
	return asMalformedFilter(decodeFlateLZW(r, f.Predictor, f.Colors, f.BitsPerComponent, f.Columns, true, f.OffByOne))
}

func (f FilterLZW) validate(v Version) error {
	return validateFlateLZW(v, f.Predictor, f.Colors, f.BitsPerComponent, f.Columns)
}

// FilterCompress is a special filter, which is used to select the
// best available compression filter when writing PDF streams.  This is
// [FilterFlate] for PDF versions 1.2 and above, and [FilterLZW] for older
// versions.
type FilterCompress struct {
	// Predictor selects the predictor algorithm.
	// On write, 0 can be used as a shorthand for [FlatePredictorNone].
	Predictor FlatePredictor

	// Colors is the number of interleaved colour components per sample.
	// Only meaningful when Predictor selects a predictor other than
	// [FlatePredictorNone]; setting Colors otherwise is a write-time error.
	//
	// On write, 0 can be used as a shorthand for 1.
	Colors int

	// BitsPerComponent is the number of bits used to represent each
	// colour component.  Only meaningful when Predictor selects a
	// predictor other than [FlatePredictorNone]; setting BitsPerComponent
	// otherwise is a write-time error.
	//
	// On write, 0 can be used as a shorthand for 8.
	BitsPerComponent int

	// Columns is the number of samples in each row.  Only meaningful
	// when Predictor selects a predictor other than [FlatePredictorNone];
	// setting Columns otherwise is a write-time error.
	//
	// On write, 0 can be used as a shorthand for 1.
	Columns int
}

// Info implements the [Filter] interface.
func (f FilterCompress) Info(v Version) (Name, Dict, error) {
	if v >= V1_2 {
		return f.toFlate().Info(v)
	}
	return f.toLZW().Info(v)
}

// Encode implements the [Filter] interface.
func (f FilterCompress) Encode(v Version, w io.WriteCloser) (io.WriteCloser, error) {
	if v >= V1_2 {
		return f.toFlate().Encode(v, w)
	}
	return f.toLZW().Encode(v, w)
}

// Decode implements the [Filter] interface.
func (f FilterCompress) Decode(v Version, r io.Reader) (io.ReadCloser, error) {
	if v >= V1_2 {
		return f.toFlate().Decode(v, r)
	}
	return f.toLZW().Decode(v, r)
}

func (f FilterCompress) toFlate() FilterFlate {
	return FilterFlate{
		Predictor:        f.Predictor,
		Colors:           f.Colors,
		BitsPerComponent: f.BitsPerComponent,
		Columns:          f.Columns,
	}
}

func (f FilterCompress) toLZW() FilterLZW {
	return FilterLZW{
		Predictor:        f.Predictor,
		Colors:           f.Colors,
		BitsPerComponent: f.BitsPerComponent,
		Columns:          f.Columns,
		OffByOne:         true, // PDF default
	}
}

// FilterCCITTFax is the CCITTFaxDecode filter.
type FilterCCITTFax struct {
	// K identifies the encoding scheme used:
	//   K < 0: Group 4 (pure two-dimensional)
	//   K = 0: Group 3, one-dimensional (PDF default)
	//   K > 0: Group 3, mixed one- and two-dimensional
	K int

	// EndOfLine indicates whether end-of-line bit patterns are present
	// in the encoded data.
	EndOfLine bool

	// EncodedByteAlign indicates that each encoded scan line is padded
	// with zero bits to begin on a byte boundary.
	EncodedByteAlign bool

	// Columns is the width of the image in pixels.
	//
	// On write, 0 can be used as a shorthand for 1728.
	Columns int

	// Rows is the height of the image in scan lines.
	// A value of 0 means the height is not predetermined.
	Rows int

	// IgnoreEndOfBlock causes the filter to ignore end-of-block bit
	// patterns in the encoded data.  The Go zero value (false)
	// corresponds to the PDF default (EndOfBlock=true).
	IgnoreEndOfBlock bool

	// BlackIs1, if true, interprets bits with value 1 as black pixels
	// (the reverse of the normal PDF convention).
	BlackIs1 bool

	// DamagedRowsBeforeError is the number of damaged rows that may be
	// tolerated before an error occurs.
	DamagedRowsBeforeError int
}

// Info implements the [Filter] interface.
func (f FilterCCITTFax) Info(v Version) (Name, Dict, error) {
	if err := f.validate(v); err != nil {
		return "", nil, err
	}

	res := Dict{}
	if f.K != 0 {
		res["K"] = Integer(f.K)
	}
	if f.EndOfLine {
		res["EndOfLine"] = Boolean(true)
	}
	if f.EncodedByteAlign {
		res["EncodedByteAlign"] = Boolean(true)
	}
	if f.Columns != 0 && f.Columns != 1728 {
		res["Columns"] = Integer(f.Columns)
	}
	if f.Rows > 0 {
		res["Rows"] = Integer(f.Rows)
	}
	if f.IgnoreEndOfBlock {
		res["EndOfBlock"] = Boolean(false)
	}
	if f.BlackIs1 {
		res["BlackIs1"] = Boolean(true)
	}
	if f.DamagedRowsBeforeError > 0 {
		res["DamagedRowsBeforeError"] = Integer(f.DamagedRowsBeforeError)
	}

	if len(res) == 0 {
		return "CCITTFaxDecode", nil, nil
	}
	return "CCITTFaxDecode", res, nil
}

// Encode implements the [Filter] interface.
func (f FilterCCITTFax) Encode(v Version, w io.WriteCloser) (io.WriteCloser, error) {
	if err := f.validate(v); err != nil {
		return nil, err
	}
	ww, err := ccittfax.NewWriter(w, f.toParams())
	if err != nil {
		return nil, err
	}
	return &withClose{
		Writer: ww,
		close: func() error {
			err := ww.Close()
			if err != nil {
				return err
			}
			return w.Close()
		},
	}, nil
}

// Decode implements the [Filter] interface.
func (f FilterCCITTFax) Decode(_ Version, r io.Reader) (io.ReadCloser, error) {
	reader, err := ccittfax.NewReader(r, f.toParams())
	if err != nil {
		return asMalformedFilter(nil, err)
	}
	return asMalformedFilter(io.NopCloser(reader), nil)
}

func (f FilterCCITTFax) validate(_ Version) error {
	// Cap dimensions at 1<<20, matching the internal ccittfax.maxColumns
	// constant and the Flate predictor's column bound.  This catches
	// pathological inputs (memory exhaustion in the encoder) before
	// setXRef rather than partway through writing.
	const maxDim = 1 << 20
	if f.Columns < 0 || f.Columns > maxDim {
		return fmt.Errorf("invalid number of columns %d", f.Columns)
	}
	if f.Rows < 0 || f.Rows > maxDim {
		return fmt.Errorf("invalid number of rows %d", f.Rows)
	}
	if f.DamagedRowsBeforeError < 0 || f.DamagedRowsBeforeError > maxDim {
		return fmt.Errorf("invalid number of damaged rows %d", f.DamagedRowsBeforeError)
	}
	return nil
}

func (f FilterCCITTFax) toParams() *ccittfax.Params {
	cols := f.Columns
	if cols == 0 {
		cols = 1728
	}
	return &ccittfax.Params{
		Columns:                cols,
		K:                      f.K,
		MaxRows:                f.Rows,
		EndOfLine:              f.EndOfLine,
		EncodedByteAlign:       f.EncodedByteAlign,
		BlackIs1:               f.BlackIs1,
		IgnoreEndOfBlock:       f.IgnoreEndOfBlock,
		DamagedRowsBeforeError: f.DamagedRowsBeforeError,
	}
}

// DCTColorTransform selects the colour-space transformation applied
// by the [FilterDCT] filter.
//
// The Go zero value [DCTColorTransformAuto] omits the ColorTransform
// entry from the parameter dictionary, leaving the choice to the
// JPEG markers (and, in their absence, the PDF default rules).
type DCTColorTransform int

// DCTColorTransform values.  The numeric constant values are an
// internal Go encoding; the wire mapping happens in
// [FilterDCT.Info] and [MakeFilter].
const (
	// DCTColorTransformAuto leaves the transform choice to the JPEG
	// markers and PDF default rules (no ColorTransform entry written).
	DCTColorTransformAuto DCTColorTransform = iota

	// DCTColorTransformNone applies no colour transform (PDF value 0).
	DCTColorTransformNone

	// DCTColorTransformYCbCr converts between RGB/YCbCr (3 components)
	// or CMYK/YCbCrK (4 components) (PDF value 1).
	DCTColorTransformYCbCr
)

// FilterDCT is the DCTDecode filter, used for JPEG-compressed data.
// This filter supports decoding only.
type FilterDCT struct {
	// ColorTransform specifies the colour-space transformation applied
	// during decoding.
	ColorTransform DCTColorTransform
}

// Info implements the [Filter] interface.
func (f FilterDCT) Info(_ Version) (Name, Dict, error) {
	switch f.ColorTransform {
	case DCTColorTransformAuto:
		return "DCTDecode", nil, nil
	case DCTColorTransformNone:
		return "DCTDecode", Dict{"ColorTransform": Integer(0)}, nil
	case DCTColorTransformYCbCr:
		return "DCTDecode", Dict{"ColorTransform": Integer(1)}, nil
	default:
		return "", nil, fmt.Errorf("invalid DCTColorTransform value %d", f.ColorTransform)
	}
}

// Encode implements the [Filter] interface.
// DCTDecode encoding is not supported via the filter interface.
func (f FilterDCT) Encode(_ Version, _ io.WriteCloser) (io.WriteCloser, error) {
	return nil, errors.New("DCTDecode encoding not supported via filter interface")
}

// Decode implements the [Filter] interface.
func (f FilterDCT) Decode(_ Version, r io.Reader) (io.ReadCloser, error) {
	var ct *int
	switch f.ColorTransform {
	case DCTColorTransformNone:
		v := 0
		ct = &v
	case DCTColorTransformYCbCr:
		v := 1
		ct = &v
	}
	return asMalformedFilter(dct.Decode(r, ct))
}

// FilterJBIG2 is the JBIG2Decode filter for bi-level image compression.
// JBIG2Decode encoding is not supported via the filter interface;
// use the graphics/image/jbig2 package for encoding.
type FilterJBIG2 struct {
	// Globals holds the resolved contents of the JBIG2Globals stream,
	// or nil if the encoded data does not reference global segments.
	// The field is populated automatically when reading; for writing
	// it has no effect (use GlobalsRef instead).
	Globals []byte

	// GlobalsRef is the /JBIG2Globals entry from the stream's
	// /DecodeParms, or nil if absent.  Per spec the value is a
	// [Reference] to a globals stream; the field is typed as [Object]
	// so that malformed inputs round-trip rather than being lost.
	//
	// On read, GlobalsRef points into the source file's object graph;
	// it is preserved so that round-trip writers can either use it
	// as-is (in-place re-write) or remap it (cross-file write).
	// [Copier] handles the cross-file remap implicitly by deep-copying
	// the stream dictionary, so callers using Copier need not touch
	// this field; callers writing through OpenStream from scratch must
	// allocate a destination-side globals stream and set GlobalsRef
	// to its [Reference].
	GlobalsRef Object
}

// Info implements the [Filter] interface.
func (f *FilterJBIG2) Info(v Version) (Name, Dict, error) {
	if err := checkVersionV(v, "JBIG2Decode filter", V1_4); err != nil {
		return "", nil, err
	}
	if f.GlobalsRef != nil {
		return "JBIG2Decode", Dict{"JBIG2Globals": f.GlobalsRef}, nil
	}
	return "JBIG2Decode", nil, nil
}

// Encode implements the [Filter] interface.
// JBIG2Decode encoding is not supported via the filter interface.
func (f *FilterJBIG2) Encode(_ Version, _ io.WriteCloser) (io.WriteCloser, error) {
	return nil, errors.New("JBIG2Decode encoding not supported via filter interface")
}

// Decode implements the [Filter] interface.
func (f *FilterJBIG2) Decode(_ Version, r io.Reader) (io.ReadCloser, error) {
	pageData, err := io.ReadAll(io.LimitReader(r, streamlimits.MaxJBIG2PageBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(pageData)) > streamlimits.MaxJBIG2PageBytes {
		return nil, &MalformedFileError{Err: errors.New("JBIG2 page data exceeds size limit")}
	}

	bm, err := jbig2.Decode(f.Globals, pageData)
	if err != nil {
		return asMalformedFilter(nil, err)
	}

	// JBIG2 uses 1=black, but the normal PDF convention for decoded
	// bi-level image data is 0=black (see the BlackIs1 definition in
	// §7.4.6).  Invert all bytes to match.
	for i := range bm.Pix {
		bm.Pix[i] ^= 0xFF
	}

	return io.NopCloser(bytes.NewReader(bm.Pix)), nil
}

// FilterJPX is the JPXDecode filter for JPEG 2000-compressed data.
// JPXDecode is not implemented; this type exists so PDF files using
// the filter survive a read/write cycle.
type FilterJPX struct{}

// Info implements the [Filter] interface.
func (f FilterJPX) Info(v Version) (Name, Dict, error) {
	if err := checkVersionV(v, "JPXDecode filter", V1_5); err != nil {
		return "", nil, err
	}
	return "JPXDecode", nil, nil
}

// Encode implements the [Filter] interface.
// JPXDecode encoding is not supported.
func (f FilterJPX) Encode(_ Version, _ io.WriteCloser) (io.WriteCloser, error) {
	return nil, errors.New("JPXDecode encoding not supported")
}

// Decode implements the [Filter] interface.
// JPXDecode decoding is not supported.
func (f FilterJPX) Decode(_ Version, _ io.Reader) (io.ReadCloser, error) {
	return asMalformedFilter(nil, errors.New("JPXDecode decoding not supported"))
}

// CryptFilter is the common interface implemented by the three Crypt
// filter variants: [FilterCryptIdentity], [FilterCryptStandard], and
// [FilterCryptNamed].
//
// Per PDF spec §7.4.10, a Crypt filter must be the first entry in a
// stream's /Filter array.  The library enforces this on read (via
// [GetFilters]) and on write (via [Writer.OpenStream]).
type CryptFilter interface {
	Filter
	isCryptFilter()
}

// FilterCryptIdentity declares that a stream's bytes are stored
// plaintext on disk, even when the document otherwise uses an /Encrypt
// dictionary.  Producers use this to mark a specific stream (for
// example, a Metadata stream) as exempt from document-level encryption.
type FilterCryptIdentity struct{}

func (FilterCryptIdentity) isCryptFilter() {}

// Info implements the [Filter] interface.
func (FilterCryptIdentity) Info(v Version) (Name, Dict, error) {
	if err := checkVersionV(v, "Crypt filter", V1_5); err != nil {
		return "", nil, err
	}
	return "Crypt", nil, nil
}

// Encode implements the [Filter] interface.
// For [FilterCryptIdentity] this is a pass-through: the stream's bytes
// are written unchanged.  The document-level encryption is bypassed
// for this stream by [Writer.OpenStream].
func (FilterCryptIdentity) Encode(_ Version, w io.WriteCloser) (io.WriteCloser, error) {
	return w, nil
}

// Decode implements the [Filter] interface.
// For [FilterCryptIdentity] this is a pass-through: the stream's bytes
// are read unchanged.  The document-level decryption is bypassed for
// this stream by [DecodeStream].
func (FilterCryptIdentity) Decode(_ Version, r io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(r), nil
}

// FilterCryptStandard declares that a stream is encrypted using the
// document's /StdCF crypt filter, with the file key used as-is (no
// per-object Algorithm 1 key derivation, per PDF spec §7.4.10).
//
// Encoding and decoding via the filter interface are not yet supported;
// the type exists so PDF files using /Crypt with /Name /StdCF survive a
// read/write cycle.
type FilterCryptStandard struct{}

func (FilterCryptStandard) isCryptFilter() {}

// Info implements the [Filter] interface.
func (FilterCryptStandard) Info(v Version) (Name, Dict, error) {
	if err := checkVersionV(v, "Crypt filter", V1_5); err != nil {
		return "", nil, err
	}
	return "Crypt", Dict{"Name": Name("StdCF")}, nil
}

// Encode implements the [Filter] interface.
func (FilterCryptStandard) Encode(_ Version, _ io.WriteCloser) (io.WriteCloser, error) {
	return nil, errors.New("FilterCryptStandard encoding not yet supported")
}

// Decode implements the [Filter] interface.
func (FilterCryptStandard) Decode(_ Version, _ io.Reader) (io.ReadCloser, error) {
	return asMalformedFilter(nil, errors.New("FilterCryptStandard decoding not yet supported"))
}

// FilterCryptNamed declares that a stream is encrypted using a named
// crypt filter from the document's /CF dictionary, with the file key
// used as-is (no per-object Algorithm 1 key derivation, per PDF spec
// §7.4.10).
//
// Name must not be empty, "Identity" (use [FilterCryptIdentity]
// instead), or "StdCF" (use [FilterCryptStandard] instead).
//
// Encoding and decoding via the filter interface are not yet supported;
// the type exists so PDF files using a named /Crypt filter survive a
// read/write cycle.
type FilterCryptNamed struct {
	Name Name
}

func (FilterCryptNamed) isCryptFilter() {}

// Info implements the [Filter] interface.
func (f FilterCryptNamed) Info(v Version) (Name, Dict, error) {
	if err := checkVersionV(v, "Crypt filter", V1_5); err != nil {
		return "", nil, err
	}
	switch f.Name {
	case "":
		return "", nil, errors.New("FilterCryptNamed: Name must not be empty")
	case "Identity":
		return "", nil, errors.New("FilterCryptNamed: use FilterCryptIdentity for /Identity")
	case "StdCF":
		return "", nil, errors.New("FilterCryptNamed: use FilterCryptStandard for /StdCF")
	}
	return "Crypt", Dict{"Name": f.Name}, nil
}

// Encode implements the [Filter] interface.
func (FilterCryptNamed) Encode(_ Version, _ io.WriteCloser) (io.WriteCloser, error) {
	return nil, errors.New("FilterCryptNamed encoding not yet supported")
}

// Decode implements the [Filter] interface.
func (FilterCryptNamed) Decode(_ Version, _ io.Reader) (io.ReadCloser, error) {
	return asMalformedFilter(nil, errors.New("FilterCryptNamed decoding not yet supported"))
}

// checkVersionV is the equivalent of [CheckVersion] for use
// when only a [Version] (and no [Writer]) is available.
func checkVersionV(v Version, operation string, minVersion Version) error {
	if v >= minVersion {
		return nil
	}
	return &VersionError{Operation: operation, Earliest: minVersion}
}

// parseFlate parses a FlateDecode parameter dictionary into a FilterFlate.
// The parsing is permissive: malformed or absent values are silently
// replaced with PDF defaults so that a read→write→read cycle is stable.
func parseFlate(d Dict) FilterFlate {
	res := FilterFlate{
		Predictor: FlatePredictorNone,
	}
	if val, ok := d["Predictor"].(Integer); ok {
		p := FlatePredictor(val)
		if p.isValid() && p != 0 {
			res.Predictor = p
		}
	}
	usingPredictor := res.Predictor != FlatePredictorNone
	if usingPredictor {
		res.Colors = 1
		if val, ok := d["Colors"].(Integer); ok && val >= 1 && val <= Integer(maxInt) {
			res.Colors = int(val)
		}
		res.BitsPerComponent = 8
		if val, ok := d["BitsPerComponent"].(Integer); ok {
			switch val {
			case 1, 2, 4, 8, 16:
				res.BitsPerComponent = int(val)
			}
		}
		res.Columns = 1
		if val, ok := d["Columns"].(Integer); ok && val >= 1 && val <= 1<<20 {
			res.Columns = int(val)
		}
	}
	return res
}

// parseLZW parses an LZWDecode parameter dictionary into a FilterLZW.
func parseLZW(d Dict) FilterLZW {
	f := parseFlate(d)
	res := FilterLZW{
		Predictor:        f.Predictor,
		Colors:           f.Colors,
		BitsPerComponent: f.BitsPerComponent,
		Columns:          f.Columns,
		OffByOne:         true, // PDF default
	}
	if val, ok := d["EarlyChange"].(Integer); ok && val == 0 {
		res.OffByOne = false
	}
	return res
}

// parseCCITTFax parses a CCITTFaxDecode parameter dictionary into a FilterCCITTFax.
// Out-of-range values are silently demoted to defaults so a malformed input
// round-trips through validate() without an asymmetric write-side rejection.
func parseCCITTFax(d Dict) FilterCCITTFax {
	const maxDim = 1 << 20
	res := FilterCCITTFax{
		Columns: 1728,
	}
	if val, ok := d["K"].(Integer); ok {
		switch {
		case val < 0:
			res.K = -1
		case val > Integer(maxInt):
			res.K = maxInt
		default:
			res.K = int(val)
		}
	}
	if val, ok := d["EndOfLine"].(Boolean); ok {
		res.EndOfLine = bool(val)
	}
	if val, ok := d["EncodedByteAlign"].(Boolean); ok {
		res.EncodedByteAlign = bool(val)
	}
	if val, ok := d["Columns"].(Integer); ok && val > 0 && val <= maxDim {
		res.Columns = int(val)
	}
	if val, ok := d["Rows"].(Integer); ok && val > 0 && val <= maxDim {
		res.Rows = int(val)
	}
	if val, ok := d["EndOfBlock"].(Boolean); ok && !bool(val) {
		res.IgnoreEndOfBlock = true
	}
	if val, ok := d["BlackIs1"].(Boolean); ok {
		res.BlackIs1 = bool(val)
	}
	if val, ok := d["DamagedRowsBeforeError"].(Integer); ok && val > 0 && val <= maxDim {
		res.DamagedRowsBeforeError = int(val)
	}
	return res
}

// parseDCT parses a DCTDecode parameter dictionary into a FilterDCT.
func parseDCT(d Dict) FilterDCT {
	res := FilterDCT{}
	if val, ok := d["ColorTransform"].(Integer); ok {
		switch val {
		case 0:
			res.ColorTransform = DCTColorTransformNone
		case 1:
			res.ColorTransform = DCTColorTransformYCbCr
		}
		// any other value -> Auto (the Go zero value)
	}
	return res
}

// parseJBIG2 parses a JBIG2Decode parameter dictionary into a FilterJBIG2.
// The JBIG2Globals stream contents are resolved later by resolveJBIG2Globals.
func parseJBIG2(d Dict) *FilterJBIG2 {
	return &FilterJBIG2{
		GlobalsRef: d["JBIG2Globals"],
	}
}

// validateFlateLZW checks the predictor parameters shared by Flate and LZW.
func validateFlateLZW(v Version, p FlatePredictor, colors, bpc, columns int) error {
	if !p.isValid() {
		return fmt.Errorf("unsupported predictor %d", p)
	}
	usingPredictor := p != 0 && p != FlatePredictorNone
	if !usingPredictor && colors != 0 {
		return fmt.Errorf("Colors=%d requires a predictor with colour components", colors)
	}
	if !usingPredictor && bpc != 0 {
		return fmt.Errorf("BitsPerComponent=%d requires a predictor", bpc)
	}
	if !usingPredictor && columns != 0 {
		return fmt.Errorf("Columns=%d requires a predictor", columns)
	}
	if usingPredictor {
		if colors != 0 {
			if colors < 1 || (v < V1_3 && colors > 4) {
				return fmt.Errorf("invalid number of colour channels %d", colors)
			}
		}
		if bpc != 0 {
			switch bpc {
			case 1, 2, 4, 8:
				// always valid
			case 16:
				if err := checkVersionV(v, "FlateDecode/LZWDecode BitsPerComponent=16", V1_5); err != nil {
					return err
				}
			default:
				return fmt.Errorf("invalid number of bits per component %d", bpc)
			}
		}
		if columns != 0 && (columns < 1 || columns > 1<<20) {
			return fmt.Errorf("invalid number of columns %d", columns)
		}
	}
	return nil
}

// encodeFlateLZW returns a writer that compresses data using Flate (or LZW
// when isLZW is true), optionally followed by a predictor.
func encodeFlateLZW(w io.WriteCloser, p FlatePredictor, colors, bpc, columns int, isLZW, lzwOffByOne bool) (io.WriteCloser, error) {
	var zw io.WriteCloser
	var err error
	if isLZW {
		zw, err = lzw.NewWriter(w, lzwOffByOne)
		if err != nil {
			return nil, err
		}
	} else {
		tmp := zlibWriterPool.Get().(*zlib.Writer)
		tmp.Reset(w)
		zw = tmp
	}

	originalZw := zw
	close := func() error {
		err := originalZw.Close()
		if err != nil {
			return err
		}
		if !isLZW {
			zlibWriterPool.Put(originalZw)
		}
		return w.Close()
	}
	zw = &withClose{zw, close}

	return predict.NewWriter(zw, predictParams(p, colors, bpc, columns))
}

// decodeFlateLZW returns a reader that decompresses Flate/LZW data,
// optionally followed by a predictor decoding step.
func decodeFlateLZW(r io.Reader, p FlatePredictor, colors, bpc, columns int, isLZW, lzwOffByOne bool) (io.ReadCloser, error) {
	var inner io.ReadCloser
	var err error
	if isLZW {
		inner = lzw.NewReader(r, lzwOffByOne)
	} else {
		inner, err = zlibNewReader(r)
		if err != nil {
			return nil, err
		}
	}
	return predict.NewReader(inner, predictParams(p, colors, bpc, columns))
}

func predictParams(p FlatePredictor, colors, bpc, columns int) *predict.Params {
	if colors == 0 {
		colors = 1
	}
	if bpc == 0 {
		bpc = 8
	}
	if columns == 0 {
		columns = 1
	}
	pred := int(p)
	if pred == 0 {
		pred = 1
	}
	return &predict.Params{
		Colors:           colors,
		BitsPerComponent: bpc,
		Columns:          columns,
		Predictor:        pred,
	}
}

const maxInt = int(^uint(0) >> 1)

// withDummyClose turns and io.Writer into an io.WriteCloser.
type withDummyClose struct {
	io.Writer
}

func (w withDummyClose) Close() error {
	return nil
}

type withClose struct {
	io.Writer
	close func() error
}

func (w *withClose) Close() error {
	return w.close()
}

func appendFilter(streamDict Dict, name Name, parms Dict) {
	switch filter := streamDict["Filter"].(type) {
	case Name:
		streamDict["Filter"] = Array{filter, name}
		p0, _ := streamDict["DecodeParms"].(Dict)
		if len(p0)+len(parms) > 0 {
			streamDict["DecodeParms"] = Array{p0, parms}
		}

	case Array:
		streamDict["Filter"] = append(filter, name)
		pp, _ := streamDict["DecodeParms"].(Array)
		needsParms := len(parms) > 0
		for i := 0; i < len(pp) && !needsParms; i++ {
			pi, _ := pp[i].(Dict)
			needsParms = len(pi) > 0
		}
		if needsParms {
			for len(pp) < len(filter) {
				pp = append(pp, nil)
			}
			pp := pp[:len(filter)]
			streamDict["DecodeParms"] = append(pp, parms)
		}

	default:
		streamDict["Filter"] = name
		if len(parms) > 0 {
			streamDict["DecodeParms"] = parms
		}
	}
}

// asMalformedFilter reclassifies the result of a Filter.Decode call.
// Any non-[MalformedFileError] error returned by the filter's
// construction, or by a subsequent Read from the returned reader, is
// wrapped as [*MalformedFileError] so that permissive readers (via
// [IsMalformed] and [Optional]) recognise it as recoverable content
// corruption. Errors that are already malformed pass through unchanged.
//
// Source-read errors that flow up through a filter are also wrapped here,
// but they remain recoverable via [errors.As]; [DecodeStream]'s
// sourceAwareReader (phase 3) restores them at the top of the stack.
func asMalformedFilter(rc io.ReadCloser, err error) (io.ReadCloser, error) {
	if err != nil {
		if !IsMalformed(err) {
			err = &MalformedFileError{Err: err}
		}
		return nil, err
	}
	return &filterContentReader{rc}, nil
}

type filterContentReader struct {
	io.ReadCloser
}

func (c *filterContentReader) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	if err != nil && !errors.Is(err, io.EOF) && !IsMalformed(err) {
		err = &MalformedFileError{Err: err}
	}
	return n, err
}

func zlibNewReader(r io.Reader) (io.ReadCloser, error) {
	obj := zlibReaderPool.Get()
	if obj != nil {
		zr := obj.(zlib.Resetter)
		if err := zr.Reset(r, nil); err != nil {
			return nil, err
		}
		return pooledZlibReader{obj.(io.ReadCloser)}, nil
	}

	zr, err := zlib.NewReader(r)
	if err != nil {
		return nil, err
	}
	return pooledZlibReader{zr}, nil
}

type pooledZlibReader struct {
	io.ReadCloser
}

// Read delegates to the wrapped zlib reader, but masks a final
// [zlib.ErrChecksum] as [io.EOF].  PDF readers in the wild routinely ignore
// the trailing Adler-32 check, and we follow suit here so that a corrupt
// checksum does not make an otherwise readable stream unusable.
func (r pooledZlibReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if err == zlib.ErrChecksum {
		err = io.EOF
	}
	return n, err
}

func (r pooledZlibReader) Close() error {
	err := r.ReadCloser.Close()
	if err == zlib.ErrChecksum {
		err = nil
	}
	if err != nil {
		return err
	}
	zlibReaderPool.Put(r.ReadCloser)
	return nil
}

var (
	zlibReaderPool = &sync.Pool{}

	zlibWriterPool = sync.Pool{
		New: func() any {
			zw, _ := zlib.NewWriterLevel(nil, zlib.BestCompression)
			return zw
		},
	}
)
