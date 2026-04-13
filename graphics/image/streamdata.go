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

package image

import (
	"fmt"
	"io"
	"maps"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// streamData is the ImageData implementation for images read from a PDF
// file.  It holds a reference to the source stream and lazily decodes
// or copies it on demand.
//
// Pixels() fully decodes the stream (all filters) and returns raw pixel
// bytes.
//
// WriteStream() copies the stream with its original encoding
// (Filter / DecodeParms) to the destination PDF, preserving JPEG,
// CCITT, JBIG2, Flate or any other encoding without a
// decode/re-encode round-trip.
type streamData struct {
	getter pdf.Getter
	stream *pdf.Stream
}

// NewStreamData returns an [graphics.ImageData] that lazily reads from
// the given PDF stream.  On [graphics.ImageData.WriteStream], the
// stream is copied with its original encoding.  On
// [graphics.ImageData.Pixels], the stream is fully decoded.
func NewStreamData(getter pdf.Getter, stream *pdf.Stream) graphics.ImageData {
	return &streamData{getter: getter, stream: stream}
}

// Pixels returns the fully decoded pixel data from the stream.
func (s *streamData) Pixels() ([]byte, error) {
	r, err := pdf.DecodeStream(s.getter, s.stream, 0)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// WriteStream copies the stream to the destination PDF, preserving the
// original encoding.
func (s *streamData) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	dict = maps.Clone(dict)

	// copy Filter and DecodeParms from the source stream dict
	if filter := s.stream.Dict["Filter"]; filter != nil {
		resolved, err := pdf.Resolve(s.getter, filter)
		if err != nil {
			return err
		}
		dict["Filter"] = resolved
	}
	if dp := s.stream.Dict["DecodeParms"]; dp != nil {
		copied, err := s.copyDecodeParms(rm, dp)
		if err != nil {
			return err
		}
		dict["DecodeParms"] = copied
	}

	// read raw stream bytes (decrypted but not filter-decoded)
	raw, err := pdf.RawStreamReader(s.getter, s.stream)
	if err != nil {
		return fmt.Errorf("cannot read image stream: %w", err)
	}
	defer raw.Close()

	// write to destination with no additional filters
	w, err := rm.Out().OpenStream(ref, dict)
	if err != nil {
		return fmt.Errorf("cannot open image stream: %w", err)
	}
	if _, err := io.Copy(w, raw); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// copyDecodeParms deep-copies a DecodeParms value, re-embedding any
// stream references (e.g. JBIG2Globals) in the destination PDF.
func (s *streamData) copyDecodeParms(rm *pdf.EmbedHelper, obj pdf.Object) (pdf.Object, error) {
	resolved, err := pdf.Resolve(s.getter, obj)
	if err != nil {
		return nil, err
	}

	switch v := resolved.(type) {
	case pdf.Dict:
		out := make(pdf.Dict, len(v))
		for key, val := range v {
			out[key], err = s.copyDecodeParmsValue(rm, val)
			if err != nil {
				return nil, err
			}
		}
		return out, nil
	case pdf.Array:
		out := make(pdf.Array, len(v))
		for i, elem := range v {
			out[i], err = s.copyDecodeParms(rm, elem)
			if err != nil {
				return nil, err
			}
		}
		return out, nil
	default:
		return resolved, nil
	}
}

// copyDecodeParmsValue copies a single value from DecodeParms.  If the
// value is a reference to a stream (e.g. JBIG2Globals), the stream is
// copied to the destination PDF with its original encoding.
func (s *streamData) copyDecodeParmsValue(rm *pdf.EmbedHelper, obj pdf.Object) (pdf.Object, error) {
	resolved, err := pdf.Resolve(s.getter, obj)
	if err != nil {
		return nil, err
	}

	// if the value is a stream, copy it to the destination
	if stm, ok := resolved.(*pdf.Stream); ok {
		child := &streamData{getter: s.getter, stream: stm}
		childRef := rm.Alloc()
		childDict := make(pdf.Dict, len(stm.Dict))
		for k, v := range stm.Dict {
			// skip stream-specific keys that WriteStream manages
			switch k {
			case "Filter", "DecodeParms", "Length":
				continue
			}
			childDict[k] = v
		}
		if err := child.WriteStream(rm, childRef, childDict); err != nil {
			return nil, err
		}
		return childRef, nil
	}

	return resolved, nil
}

var _ graphics.ImageData = (*streamData)(nil)
