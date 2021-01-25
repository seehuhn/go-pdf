// Copyright 2020 Jochen Voss <voss@seehuhn.de>
//
// Some code here, e.g. the pngUpReader, is taken from
// https://pkg.go.dev/rsc.io/pdf .  Use of this source code is governed by a
// BSD-style license, which is reproduced here:
//
//     Copyright (c) 2009 The Go Authors. All rights reserved.
//
//     Redistribution and use in source and binary forms, with or without
//     modification, are permitted provided that the following conditions are
//     met:
//
//        * Redistributions of source code must retain the above copyright
//     notice, this list of conditions and the following disclaimer.
//        * Redistributions in binary form must reproduce the above
//     copyright notice, this list of conditions and the following disclaimer
//     in the documentation and/or other materials provided with the
//     distribution.
//        * Neither the name of Google Inc. nor the names of its
//     contributors may be used to endorse or promote products derived from
//     this software without specific prior written permission.
//
//     THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
//     "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
//     LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
//     A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
//     OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
//     SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
//     LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
//     DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
//     THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
//     (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
//     OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package pdf

import (
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"strconv"
)

func extractFilterInfo(dict Dict) ([]*FilterInfo, error) {
	parms := dict["DecodeParms"]
	var filters []*FilterInfo
	switch f := dict["Filter"].(type) {
	case nil:
		// pass
	case Array:
		pa, _ := parms.(Array)
		for i, fi := range f {
			name, err := asName(fi)
			if err != nil {
				return nil, err
			}
			var pDict Dict
			if len(pa) > i {
				x, err := asDict(pa[i])
				if err != nil {
					return nil, err
				}
				pDict = x
			}
			filter := &FilterInfo{
				Name:  name,
				Parms: pDict,
			}
			filters = append(filters, filter)
		}
	case Name:
		pDict, err := asDict(parms)
		if err != nil {
			return nil, err
		}
		filters = append(filters, &FilterInfo{
			Name:  f,
			Parms: pDict,
		})
	default:
		return nil, errors.New("invalid /Filter field")
	}
	return filters, nil
}

type flateFilter struct {
	Predictor        int
	Colors           int
	BitsPerComponent int
	Columns          int
	EarlyChange      bool
}

func ffFromDict(parms Dict) *flateFilter {
	res := &flateFilter{
		Predictor:        1,
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          1,
		EarlyChange:      true,
	}
	if parms == nil {
		return res
	}
	if val, ok := parms["Predictor"].(Integer); ok && val >= 1 && val <= 15 {
		res.Predictor = int(val)
	}
	if val, ok := parms["Colors"].(Integer); ok && val >= 1 {
		res.Colors = int(val)
	}
	if val, ok := parms["BitsPerComponent"].(Integer); ok &&
		(val == 1 || val == 2 || val == 4 || val == 8 || val == 16) {
		res.BitsPerComponent = int(val)
	}
	if val, ok := parms["Columns"].(Integer); ok && val >= 0 && res.Predictor > 1 {
		res.Columns = int(val)
	}
	if val, ok := parms["EarlyChange"].(Integer); ok {
		res.EarlyChange = (val != 0)
	}
	return res
}

func (ff *flateFilter) ToDict() Dict {
	res := Dict{}
	needed := false
	if ff.Predictor != 1 {
		res["Predictor"] = Integer(ff.Predictor)
		needed = true
	}
	if ff.Predictor != 1 {
		res["Colors"] = Integer(ff.Colors)
		needed = true
	}
	if ff.Predictor != 8 {
		res["BitsPerComponent"] = Integer(ff.BitsPerComponent)
		needed = true
	}
	if ff.Predictor != 1 {
		res["Columns"] = Integer(ff.Columns)
		needed = true
	}
	if !ff.EarlyChange {
		res["EarlyChange"] = Integer(0)
		needed = true
	}
	if !needed {
		return nil
	}
	return res
}

func (ff *flateFilter) Encode(w io.WriteCloser) (io.WriteCloser, error) {
	zw := zlib.NewWriter(w)

	close := func() error {
		err := zw.Close()
		if err != nil {
			return err
		}
		return w.Close()
	}

	switch ff.Predictor {
	case 1:
		return &withClose{zw, close}, nil
	case 12:
		columns := ff.Columns
		return &pngUpWriter{
			w:     zw,
			prev:  make([]byte, columns),
			cur:   make([]byte, columns+1),
			close: close,
		}, nil
	default:
		return nil, errors.New("unsupported predictor " + strconv.Itoa(ff.Predictor))
	}
}

func (ff *flateFilter) Decode(r io.Reader) (io.Reader, error) {
	var res io.Reader
	var err error
	res, err = zlib.NewReader(r)
	if err != nil {
		return nil, err
	}
	switch ff.Predictor {
	case 1:
		// pass
	case 12:
		columns := ff.Columns
		res = &pngUpReader{
			r:    res,
			prev: make([]byte, 1+columns),
			tmp:  make([]byte, 1+columns),
			pend: []byte{},
		}
	default:
		return nil, errors.New("unsupported predictor " + strconv.Itoa(ff.Predictor))
	}
	return res, nil
}

type pngUpReader struct {
	r    io.Reader
	prev []byte
	tmp  []byte
	pend []byte
}

func (r *pngUpReader) Read(b []byte) (int, error) {
	n := 0
	for len(b) > 0 {
		if len(r.pend) > 0 {
			m := copy(b, r.pend)
			n += m
			b = b[m:]
			r.pend = r.pend[m:]
			continue
		}
		_, err := io.ReadFull(r.r, r.tmp)
		if err != nil {
			return n, err
		}
		if r.tmp[0] != 2 {
			return n, fmt.Errorf("malformed PNG-Up encoding")
		}
		for i, b := range r.tmp {
			r.prev[i] += b
		}
		r.pend = r.prev[1:]
	}
	return n, nil
}

type pngUpWriter struct {
	w     io.Writer
	prev  []byte // length col
	cur   []byte // length col+1
	pos   int
	close func() error
}

func (w *pngUpWriter) Write(p []byte) (int, error) {
	tmp := w.cur[1:]
	n := 0
	for len(p) > 0 {
		l := copy(tmp[w.pos:], p)
		p = p[l:]
		w.pos += l
		n += l
		if w.pos >= len(tmp) {
			w.cur[0] = 2
			for i := 0; i < w.pos; i++ {
				tmp[i], w.prev[i] = tmp[i]-w.prev[i], tmp[i]
			}
			_, err := w.w.Write(w.cur)
			if err != nil {
				return n, err
			}
			w.pos = 0
		}
	}
	return n, nil
}

func (w *pngUpWriter) Close() error {
	if w.close != nil {
		return w.close()
	}
	return nil
}

type withoutClose struct {
	io.Writer
}

func (w withoutClose) Close() error {
	return nil
}

type withClose struct {
	io.Writer
	close func() error
}

func (w *withClose) Close() error {
	return w.close()
}

// FilterInfo describes one PDF stream filter.
type FilterInfo struct {
	Name  Name
	Parms Dict
}

func (fi *FilterInfo) getFilter() (filter, error) {
	switch fi.Name {
	case "FlateDecode":
		return ffFromDict(fi.Parms), nil
	default:
		return nil, errors.New("unsupported filter type " + string(fi.Name))
	}
}

type filter interface {
	ToDict() Dict
	Encode(w io.WriteCloser) (io.WriteCloser, error)
	Decode(r io.Reader) (io.Reader, error)
}
