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
	"fmt"
	"io"
)

func applyFilter(r io.Reader, name Object, param Object) io.Reader {
	n, ok := name.(Name)
	if !ok {
		return &errorReader{
			fmt.Errorf("invalid filter description %s", format(name))}
	}
	switch string(n) {
	case "FlateDecode":
		params := map[string]int{
			"Predictor":        1,
			"Colors":           1,
			"BitsPerComponent": 8,
			"Columns":          1,
			"EarlyChange":      1,
		}
		if pDict, ok := param.(Dict); ok {
			for key := range params {
				if val, ok := pDict[Name(key)].(Integer); ok {
					params[key] = int(val)
				}
			}
		}
		var zr io.Reader
		var err error
		zr, err = zlib.NewReader(r)
		if err != nil {
			return &errorReader{err}
		}
		switch params["Predictor"] {
		case 1:
			// pass
		case 12:
			columns := params["Columns"]
			zr = &pngUpReader{
				r:    zr,
				hist: make([]byte, 1+columns),
				tmp:  make([]byte, 1+columns),
				pend: []byte{},
			}
		default:
			zr = &errorReader{fmt.Errorf("unsupported predictor %d",
				params["Predictor"])}
		}
		return zr
	default:
		return &errorReader{fmt.Errorf("unsupported filter %q", n)}
	}
}

type pngUpReader struct {
	r    io.Reader
	hist []byte
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
			r.hist[i] += b
		}
		r.pend = r.hist[1:]
	}
	return n, nil
}

type errorReader struct {
	err error
}

func (e *errorReader) Read([]byte) (int, error) {
	return 0, e.err
}
