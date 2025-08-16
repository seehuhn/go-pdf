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
	"compress/zlib"
	"fmt"
	"io"
	"sync"

	"seehuhn.de/go/pdf/internal/filter/ascii85"
	"seehuhn.de/go/pdf/internal/filter/asciihex"
	"seehuhn.de/go/pdf/internal/filter/ccittfax"
	"seehuhn.de/go/pdf/internal/filter/lzw"
	"seehuhn.de/go/pdf/internal/filter/predict"
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
// [FilterASCII85], [FilterASCIIHex], [FilterFlate], [FilterLZW].  In addition,
// [FilterCompress] can be used to select the best available compression filter
// when writing PDF streams.  This is FilterFlate for PDF versions 1.2 and
// above, and FilterLZW for older versions.
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

func makeFilter(filter Name, param Dict) Filter {
	switch filter {
	case "ASCII85Decode":
		return FilterASCII85{}
	case "ASCIIHexDecode":
		return FilterASCIIHex{}
	case "FlateDecode":
		return FilterFlate(param)
	case "LZWDecode":
		return FilterLZW(param)
	case "CCITTFaxDecode":
		return FilterCCITTFax(param)
	default:
		return &filterNotImplemented{Name: filter, Param: param}
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
	return nil, fmt.Errorf("filter %s not implemented", f.Name)
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
	return ascii85.Decode(r), nil
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
	return asciihex.Decode(r), nil
}

// FilterCompress is a special filter name, which is used to select the
// best available compression filter when writing PDF streams.  This is
// [FilterFlate] for PDF versions 1.2 and above, and [FilterLZW] for older
// versions.
type FilterCompress Dict

// Info implements the [Filter] interface.
func (f FilterCompress) Info(v Version) (Name, Dict, error) {
	if v >= V1_2 {
		return FilterFlate(f).Info(v)
	}
	return FilterLZW(f).Info(v)
}

// Encode implements the [Filter] interface.
func (f FilterCompress) Encode(v Version, w io.WriteCloser) (io.WriteCloser, error) {
	if v >= V1_2 {
		return FilterFlate(f).Encode(v, w)
	}
	return FilterLZW(f).Encode(v, w)
}

// Decode implements the [Filter] interface.
func (f FilterCompress) Decode(v Version, r io.Reader) (io.ReadCloser, error) {
	if v >= V1_2 {
		return FilterFlate(f).Decode(v, r)
	}
	return FilterLZW(f).Decode(v, r)
}

// FilterFlate is the FlateDecode filter.
//
// The filter is represented by a dictionary of filter parameters. The following
// parameters are supported:
//
//   - "Predictor": A code that selects the predictor algorithm, if any.
//     If the value is greater than 1, the data are differenced before being
//     encoded. (Default: 1)
//
//   - "Colors": The number of interleaved color components per sample.
//     (Default: 1)
//
//   - "BitsPerComponent": The number of bits used to represent each color.
//     (Default: 8)
//
//   - "Columns": The number of samples in each row. (Default: 1)
//
// The parameters are explained in detail in section 7.4.4 of PDF 32000-1:2008.
//
// This filter requires PDF versions 1.2 or higher.
type FilterFlate Dict

// Info implements the [Filter] interface.
func (f FilterFlate) Info(v Version) (Name, Dict, error) {
	ff, err := f.parseParameters(v)
	if err != nil {
		return "", nil, err
	}

	res := Dict{}
	if ff.Predictor != 1 {
		switch ff.Predictor {
		case 1, 2, 10, 11, 12, 13, 14, 15:
			// pass
		default:
			return "", nil, fmt.Errorf("unsupported predictor %d", ff.Predictor)
		}
		res["Predictor"] = Integer(ff.Predictor)
	}
	if ff.Predictor > 1 && ff.Colors != 1 {
		if ff.Colors < 1 || v < V1_3 && ff.Colors > 4 {
			return "", nil, fmt.Errorf("invalid number of colour channels %d", ff.Colors)
		}
		res["Colors"] = Integer(ff.Colors)
	}
	if ff.Predictor > 1 && ff.BitsPerComponent != 8 {
		// Valid values are 1, 2, 4, 8, and (PDF 1.5) 16
		switch ff.BitsPerComponent {
		case 1, 2, 4, 8, 16:
			if v >= V1_5 || ff.BitsPerComponent <= 8 {
				break
			}
			fallthrough
		default:
			return "", nil, fmt.Errorf("invalid number of bits per component %d", ff.BitsPerComponent)
		}
		res["BitsPerComponent"] = Integer(ff.BitsPerComponent)
	}
	if ff.Predictor > 1 && ff.Columns != 1 {
		if ff.Columns < 1 || ff.Columns > 1<<20 {
			return "", nil, fmt.Errorf("invalid number of columns %d", ff.Columns)
		}
		res["Columns"] = Integer(ff.Columns)
	}

	return "FlateDecode", res, nil
}

// Encode implements the [Filter] interface.
func (f FilterFlate) Encode(v Version, w io.WriteCloser) (io.WriteCloser, error) {
	ff, err := f.parseParameters(v)
	if err != nil {
		return nil, err
	}
	return ff.Encode(w)
}

// Decode implements the [Filter] interface.
func (f FilterFlate) Decode(v Version, r io.Reader) (io.ReadCloser, error) {
	ff, err := f.parseParameters(v)
	if err != nil {
		return nil, err
	}
	return ff.Decode(r)
}

func (f FilterFlate) parseParameters(v Version) (*flateFilter, error) {
	if v < V1_2 {
		return nil, &VersionError{Operation: "FlateDecode filter", Earliest: V1_2}
	}
	res := &flateFilter{ // set defaults
		Predictor:        1,
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          1,
	}
	if val, ok := f["Predictor"].(Integer); ok {
		res.Predictor = int(val)
	}
	if val, ok := f["Colors"].(Integer); ok {
		res.Colors = int(val)
	}
	if val, ok := f["BitsPerComponent"].(Integer); ok {
		res.BitsPerComponent = int(val)
	}
	if val, ok := f["Columns"].(Integer); ok {
		res.Columns = int(val)
	}
	return res, nil
}

// FilterLZW is the LZWDecode filter.
//
// This is only useful to read legacy PDF files.  For new files, use
// [FilterFlate] instead.
//
// The filter is represented by a dictionary of filter parameters.
// The following parameters are supported:
//
//   - "Predictor": A code that selects the predictor algorithm, if any.
//     If the value is greater than 1, the data were differenced before being
//     encoded. (Default: 1)
//
//   - "Colors": The number of interleaved color components per sample.
//     (Default: 1)
//
//   - "BitsPerComponent": The number of bits used to represent each color.
//     (Default: 8)
//
//   - "Columns": The number of samples in each row. (Default: 1)
//
//   - "EarlyChange": An integer value specifying whether the data
//     is encoded using the correct LZW algorithm (value 0), or whether
//     code with an off-by-one error is used (value 1).  (Default: 1)
//
// The parameters are explained in detail in section 7.4.4 of PDF 32000-1:2008.
type FilterLZW Dict

// Info implements the [Filter] interface.
func (f FilterLZW) Info(v Version) (Name, Dict, error) {
	// TODO(voss): move the error handling into parseParameters

	ff, err := f.parseParameters(v)
	if err != nil {
		return "", nil, err
	}

	res := Dict{}
	if ff.Predictor != 1 {
		switch ff.Predictor {
		case 1, 2, 10, 11, 12, 13, 14, 15:
			// pass
		default:
			return "", nil, fmt.Errorf("unsupported predictor %d", ff.Predictor)
		}
		res["Predictor"] = Integer(ff.Predictor)
	}
	if ff.Predictor > 1 && ff.Colors != 1 {
		if ff.Colors < 1 || v < V1_3 && ff.Colors > 4 {
			return "", nil, fmt.Errorf("invalid number of colour channels %d", ff.Colors)
		}
		res["Colors"] = Integer(ff.Colors)
	}
	if ff.Predictor > 1 && ff.BitsPerComponent != 8 {
		// Valid values are 1, 2, 4, 8, and (PDF 1.5) 16
		switch ff.BitsPerComponent {
		case 1, 2, 4, 8, 16:
			if v >= V1_5 || ff.BitsPerComponent <= 8 {
				break
			}
			fallthrough
		default:
			return "", nil, fmt.Errorf("invalid number of bits per component %d", ff.BitsPerComponent)
		}
		res["BitsPerComponent"] = Integer(ff.BitsPerComponent)
	}
	if ff.Predictor > 1 && ff.Columns != 1 {
		if ff.Columns < 1 || ff.Columns > 1<<20 {
			return "", nil, fmt.Errorf("invalid number of columns %d", ff.Columns)
		}
		res["Columns"] = Integer(ff.Columns)
	}
	if !ff.EarlyChange {
		res["EarlyChange"] = Integer(0)
	}

	return "LZWDecode", res, nil
}

// Encode implements the [Filter] interface.
func (f FilterLZW) Encode(v Version, w io.WriteCloser) (io.WriteCloser, error) {
	ff, err := f.parseParameters(v)
	if err != nil {
		return nil, err
	}
	return ff.Encode(w)
}

// Decode implements the [Filter] interface.
func (f FilterLZW) Decode(v Version, r io.Reader) (io.ReadCloser, error) {
	ff, err := f.parseParameters(v)
	if err != nil {
		return nil, err
	}
	return ff.Decode(r)
}

func (f FilterLZW) parseParameters(_ Version) (*flateFilter, error) {
	res := &flateFilter{ // set defaults
		Predictor:        1,
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          1,
		EarlyChange:      true,
		IsLZW:            true,
	}
	if val, ok := f["Predictor"].(Integer); ok {
		res.Predictor = int(val)
	}
	if val, ok := f["Colors"].(Integer); ok {
		res.Colors = int(val)
	}
	if val, ok := f["BitsPerComponent"].(Integer); ok {
		res.BitsPerComponent = int(val)
	}
	if val, ok := f["Columns"].(Integer); ok {
		res.Columns = int(val)
	}
	if val, ok := f["EarlyChange"].(Integer); ok {
		res.EarlyChange = (val != 0)
	}
	return res, nil
}

type flateFilter struct {
	Predictor        int
	Colors           int
	BitsPerComponent int
	Columns          int
	EarlyChange      bool
	IsLZW            bool
}

func (ff *flateFilter) ToDict() Dict {
	res := Dict{}
	if ff.Predictor != 1 {
		res["Predictor"] = Integer(ff.Predictor)
	}
	if ff.Predictor > 1 && ff.Colors != 1 {
		res["Colors"] = Integer(ff.Colors)
	}
	if ff.Predictor > 1 && ff.BitsPerComponent != 8 {
		res["BitsPerComponent"] = Integer(ff.BitsPerComponent)
	}
	if ff.Predictor > 1 && ff.Columns != 1 {
		res["Columns"] = Integer(ff.Columns)
	}
	if ff.IsLZW && !ff.EarlyChange {
		res["EarlyChange"] = Integer(0)
	}
	if len(res) == 0 {
		return nil
	}
	return res
}

// Decode implements the [filter] interface.
func (ff *flateFilter) Decode(r io.Reader) (io.ReadCloser, error) {
	var res io.ReadCloser
	var err error
	if ff.IsLZW {
		res = lzw.NewReader(r, ff.EarlyChange)
	} else {
		res, err = zlibNewReader(r)
	}
	if err != nil {
		return nil, err
	}

	param := &predict.Params{
		Colors:           ff.Colors,
		BitsPerComponent: ff.BitsPerComponent,
		Columns:          ff.Columns,
		Predictor:        ff.Predictor,
	}
	return predict.NewReader(res, param)
}

// Encode implements the [filter] interface.
func (ff *flateFilter) Encode(w io.WriteCloser) (io.WriteCloser, error) {
	var zw io.WriteCloser
	var err error
	if ff.IsLZW {
		zw, err = lzw.NewWriter(w, ff.EarlyChange)
		if err != nil {
			return nil, err
		}
	} else {
		// zw, err = zlib.NewWriterLevel(w, zlib.BestCompression)
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
		if !ff.IsLZW {
			zlibWriterPool.Put(originalZw)
		}
		return w.Close()
	}
	zw = &withClose{zw, close}

	params := &predict.Params{
		Colors:           ff.Colors,
		BitsPerComponent: ff.BitsPerComponent,
		Columns:          ff.Columns,
		Predictor:        ff.Predictor,
	}
	return predict.NewWriter(zw, params)
}

type FilterCCITTFax Dict

// Info returns the name and parameters of the filter,
// as they should be written to the PDF file.
func (f FilterCCITTFax) Info(_ Version) (Name, Dict, error) {
	ff, err := f.parseParameters()
	if err != nil {
		return "", nil, err
	}

	res := Dict{}
	if ff.K != 0 {
		res["K"] = Integer(ff.K)
	}
	if ff.eol {
		res["EndOfLine"] = Boolean(ff.eol)
	}
	if ff.byteAlign {
		res["EncodedByteAlign"] = Boolean(ff.byteAlign)
	}
	if ff.columns != 1728 {
		res["Columns"] = Integer(ff.columns)
	}
	if ff.rows > 0 {
		res["Rows"] = Integer(ff.rows)
	}
	if !ff.eob { // default is true
		res["EndOfBlock"] = Boolean(false)
	}
	if ff.blackIs1 {
		res["BlackIs1"] = Boolean(ff.blackIs1)
	}
	if ff.damagedRows > 0 {
		res["DamagedRowsBeforeError"] = Integer(ff.damagedRows)
	}

	return "CCITTFaxDecode", res, nil
}

// Encode returns a writer which encodes data written to it.
// The returned writer must be closed after use.
func (f FilterCCITTFax) Encode(_ Version, w io.WriteCloser) (io.WriteCloser, error) {
	ff, err := f.parseParameters()
	if err != nil {
		return nil, err
	}

	params := &ccittfax.Params{
		Columns:                ff.columns,
		K:                      ff.K,
		MaxRows:                ff.rows,
		EndOfLine:              ff.eol,
		EncodedByteAlign:       ff.byteAlign,
		BlackIs1:               ff.blackIs1,
		IgnoreEndOfBlock:       !ff.eob,
		DamagedRowsBeforeError: ff.damagedRows,
	}
	ww, err := ccittfax.NewWriter(w, params)
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

// Decode returns a reader which decodes data read from it.
func (f FilterCCITTFax) Decode(_ Version, r io.Reader) (io.ReadCloser, error) {
	ff, err := f.parseParameters()
	if err != nil {
		return nil, err
	}

	params := &ccittfax.Params{
		Columns:                ff.columns,
		K:                      ff.K,
		MaxRows:                ff.rows,
		EndOfLine:              ff.eol,
		EncodedByteAlign:       ff.byteAlign,
		BlackIs1:               ff.blackIs1,
		IgnoreEndOfBlock:       !ff.eob,
		DamagedRowsBeforeError: ff.damagedRows,
	}
	reader, err := ccittfax.NewReader(r, params)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(reader), nil
}

func (f FilterCCITTFax) parseParameters() (*ccittFilter, error) {
	res := &ccittFilter{ // set defaults
		K:       0,
		columns: 1728,
		eob:     true,
	}
	if val, ok := f["K"].(Integer); ok {
		if val < 0 {
			val = -1
		} else if val > Integer(maxInt) {
			val = 999
		}
		res.K = int(val)
	}
	if val, ok := f["EndOfLine"].(Boolean); ok {
		res.eol = bool(val)
	}
	if val, ok := f["EncodedByteAlign"].(Boolean); ok {
		res.byteAlign = bool(val)
	}
	if val, ok := f["Columns"].(Integer); ok {
		if val < 1 || val > Integer(maxInt) {
			return nil, fmt.Errorf("invalid number of columns %d", val)
		}
		res.columns = int(val)
	}
	if val, ok := f["Rows"].(Integer); ok {
		if val < 0 || val > Integer(maxInt) {
			return nil, fmt.Errorf("invalid number of rows %d", val)
		}
		res.rows = int(val)
	}
	if val, ok := f["EndOfBlock"].(Boolean); ok {
		res.eob = bool(val)
	}
	if val, ok := f["BlackIs1"].(Boolean); ok {
		res.blackIs1 = bool(val)
	}
	if val, ok := f["DamagedRowsBeforeError"].(Integer); ok {
		if val < 0 || val > Integer(maxInt) {
			return nil, fmt.Errorf("invalid number of damaged rows %d", val)
		}
		res.damagedRows = int(val)
	}

	return res, nil
}

type ccittFilter struct {
	K           int
	eol         bool
	byteAlign   bool
	columns     int
	rows        int
	eob         bool
	blackIs1    bool
	damagedRows int
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

func (r pooledZlibReader) Close() error {
	if err := r.ReadCloser.Close(); err != nil {
		return err
	}
	zlibReaderPool.Put(r.ReadCloser)
	return nil
}

var (
	zlibReaderPool = &sync.Pool{}

	zlibWriterPool = sync.Pool{
		New: func() interface{} {
			zw, _ := zlib.NewWriterLevel(nil, zlib.BestCompression)
			return zw
		},
	}
)
