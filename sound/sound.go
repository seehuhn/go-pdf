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

package sound

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/optional"
)

// PDF 2.0 section: 13.3

// Sound represents a PDF sound object: a stream containing audio sample data
// to be played by a sound annotation or sound action.
//
// For maximum portability across PDF processors, the PDF specification
// recommends:
//
//   - SampleRate: 8000, 11025, or 22050 samples per second
//   - Channels: 1 or 2
//   - BitsPerSample: 8 or 16
//   - Encoding: [EncodingRaw], [EncodingSigned], or [EncodingMuLaw]
//
// When Encoding is [EncodingMuLaw], SampleRate should be 8000, Channels 1,
// and BitsPerSample 8.  When Encoding is [EncodingRaw] or [EncodingSigned],
// SampleRate should be 11025 or 22050.  These values are recommendations
// only and are not enforced.
type Sound struct {
	// SampleRate is the sampling rate, in samples per second per channel.
	// Must be greater than zero.
	SampleRate float64

	// Channels (optional) is the number of sound channels.
	// When unset, the PDF default of 1 is used.
	Channels optional.UInt

	// BitsPerSample (optional) is the number of bits per sample value
	// per channel.  When unset, the PDF default of 8 is used.
	BitsPerSample optional.UInt

	// Encoding specifies the sample encoding format.  It must be one of
	// the named [Encoding] constants or the zero value (treated as
	// [EncodingRaw] on write).
	Encoding Encoding

	// CompressionFormat (optional) names a sound compression format
	// applied to the sample data.  This is independent of the stream
	// filter chain.  The PDF specification defines no standard values.
	CompressionFormat pdf.Name

	// CompressionParams (optional) holds parameters specific to the
	// sound compression format.  Stored verbatim.
	CompressionParams pdf.Object

	// Data is the source of the sample bytes.  Use [InlineSource] for
	// samples stored inside the PDF file or [ExternalFileSource] for
	// samples stored in an external file.
	Data Source
}

// Source supplies the sample data for a [Sound].  The two user-facing
// implementations are [InlineSource] and [ExternalFileSource].  A third,
// internal implementation is returned by [Extract] for sounds read from
// a PDF file; it preserves the original stream encoding on re-write.
type Source interface {
	// Reader returns a reader that yields the decoded sample bytes.
	// For [ExternalFileSource], an error is returned because the
	// external file cannot be opened through this API.
	Reader() (io.ReadCloser, error)

	// WriteStream opens the sound data stream on ref, writes its body,
	// and adds any source-specific entries to dict (such as the F
	// entry for an external file specification) before opening the
	// stream.
	//
	// User code does not normally call WriteStream directly; it is
	// invoked by [(*Sound).Embed].
	WriteStream(e *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error
}

// InlineSource holds sound sample data stored inside the PDF file.
type InlineSource struct {
	// WriteData writes the sample bytes (post-decode) to w.
	// It must not be nil.
	WriteData func(w io.Writer) error

	// Filter is the stream filter chain applied to the sample bytes
	// before they are written to the PDF file.
	Filter []pdf.Filter
}

// Reader implements [Source] by running [WriteData] into an in-memory
// buffer.
func (s *InlineSource) Reader() (io.ReadCloser, error) {
	if s.WriteData == nil {
		return nil, errors.New("sound: InlineSource.WriteData is nil")
	}
	var buf bytes.Buffer
	if err := s.WriteData(&buf); err != nil {
		return nil, err
	}
	return io.NopCloser(&buf), nil
}

// WriteStream implements [Source].
func (s *InlineSource) WriteStream(e *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	if s.WriteData == nil {
		return errors.New("sound: InlineSource.WriteData is nil")
	}
	stm, err := e.Out().OpenStream(ref, dict, s.Filter...)
	if err != nil {
		return err
	}
	if err := s.WriteData(stm); err != nil {
		stm.Close()
		return err
	}
	return stm.Close()
}

// ExternalFileSource refers to sound sample data stored in an external
// file.  The file format must be self-describing (e.g. AIFF, RIFF/.wav,
// or .au); the PDF processor does not interpret format-specific
// parameters from the [Sound] dictionary in this case.
type ExternalFileSource struct {
	// File describes the external sound file.  It must not be nil.
	File *file.Specification
}

// Reader implements [Source].  External files cannot be opened through
// this API, so it always returns an error.
func (s *ExternalFileSource) Reader() (io.ReadCloser, error) {
	return nil, errors.New("sound: cannot read sample data from ExternalFileSource")
}

// WriteStream implements [Source].
func (s *ExternalFileSource) WriteStream(e *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	if s.File == nil {
		return errors.New("sound: ExternalFileSource.File is nil")
	}
	f, err := e.Embed(s.File)
	if err != nil {
		return err
	}
	dict["F"] = f
	stm, err := e.Out().OpenStream(ref, dict)
	if err != nil {
		return err
	}
	return stm.Close()
}

// streamSource is the [Source] implementation returned by [Extract] for
// sounds read from a PDF file.  Reader decodes the source stream
// through its filter chain; WriteStream copies the raw, still-encoded
// bytes verbatim to the destination, preserving the original /Filter
// and /DecodeParms entries.
type streamSource struct {
	getter pdf.Getter
	stream *pdf.Stream
}

// Reader implements [Source] by decoding the source stream through its
// filter chain.
func (s *streamSource) Reader() (io.ReadCloser, error) {
	return pdf.DecodeStream(s.getter, s.stream, 0)
}

// WriteStream implements [Source] by copying the raw, still-encoded
// stream bytes verbatim to the destination, along with the original
// /Filter and /DecodeParms entries.
func (s *streamSource) WriteStream(e *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	dict = maps.Clone(dict)

	if filter := s.stream.Dict["Filter"]; filter != nil {
		resolved, err := pdf.Resolve(s.getter, filter)
		if err != nil {
			return err
		}
		dict["Filter"] = resolved
	}
	if dp := s.stream.Dict["DecodeParms"]; dp != nil {
		resolved, err := pdf.Resolve(s.getter, dp)
		if err != nil {
			return err
		}
		dict["DecodeParms"] = resolved
	}

	raw, err := pdf.RawStreamReader(s.getter, s.stream)
	if err != nil {
		return err
	}
	defer raw.Close()

	w, err := e.Out().OpenStream(ref, dict)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, raw); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// Extract reads a sound object from the PDF file.  The isDirect parameter
// is accepted for signature uniformity but ignored, because sound objects
// are always indirect stream objects.
func Extract(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (*Sound, error) {
	stream, err := x.GetStream(path, obj)
	if err != nil {
		return nil, err
	} else if stream == nil {
		return nil, pdf.Error("missing sound stream")
	}
	if err := pdf.CheckDictType(x.R, stream.Dict, "Sound"); err != nil {
		return nil, err
	}

	res := &Sound{
		Channels:      optional.NewUInt(1),
		BitsPerSample: optional.NewUInt(8),
		Encoding:      EncodingRaw,
	}

	// SampleRate (R) — required.
	if r, err := pdf.Optional(x.GetNumber(path, stream.Dict["R"])); err != nil {
		return nil, err
	} else if r > 0 && !math.IsInf(r, 0) && !math.IsNaN(r) {
		res.SampleRate = r
	} else {
		return nil, pdf.Error("sound: missing or invalid R entry")
	}

	// Channels (C) — optional, default 1.
	if c, err := pdf.Optional(x.GetInteger(path, stream.Dict["C"])); err != nil {
		return nil, err
	} else if c > 0 && c <= math.MaxUint32 {
		res.Channels = optional.NewUInt(uint(c))
	}

	// BitsPerSample (B) — optional, default 8.
	if b, err := pdf.Optional(x.GetInteger(path, stream.Dict["B"])); err != nil {
		return nil, err
	} else if b > 0 && b <= math.MaxUint32 {
		res.BitsPerSample = optional.NewUInt(uint(b))
	}

	// Encoding (E) — optional, default Raw; unknown names ignored.
	if enc, err := pdf.Optional(x.GetName(path, stream.Dict["E"])); err != nil {
		return nil, err
	} else if enc != "" {
		switch Encoding(enc) {
		case EncodingRaw, EncodingSigned, EncodingMuLaw, EncodingALaw:
			res.Encoding = Encoding(enc)
		}
	}

	// CompressionFormat (CO) — optional, opaque.
	if co, err := pdf.Optional(x.GetName(path, stream.Dict["CO"])); err != nil {
		return nil, err
	} else if co != "" {
		res.CompressionFormat = co
	}

	// CompressionParams (CP) — optional, opaque (stored verbatim).
	if cp := stream.Dict["CP"]; cp != nil {
		res.CompressionParams = cp
	}

	// ExternalFile (F) — optional.  When present, the sample data lives
	// in an external file; the inline stream body is ignored.
	externalFile, err := pdf.ExtractorGetOptional(x, path, stream.Dict["F"], file.ExtractSpecification)
	if err != nil {
		return nil, err
	}
	if externalFile != nil {
		res.Data = &ExternalFileSource{File: externalFile}
	} else {
		res.Data = &streamSource{getter: x.R, stream: stream}
	}

	return res, nil
}

// Embed writes the sound object to the PDF file as an indirect stream
// object and returns a reference to it.
func (s *Sound) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "sound object", pdf.V1_2); err != nil {
		return nil, err
	}

	if s.SampleRate <= 0 {
		return nil, errors.New("sound: SampleRate must be greater than zero")
	}
	if c, ok := s.Channels.Get(); ok && c == 0 {
		return nil, errors.New("sound: Channels must be greater than zero")
	}
	if b, ok := s.BitsPerSample.Get(); ok && b == 0 {
		return nil, errors.New("sound: BitsPerSample must be greater than zero")
	}
	if s.Data == nil {
		return nil, errors.New("sound: Data is required")
	}
	switch s.Encoding {
	case "", EncodingRaw, EncodingSigned, EncodingMuLaw, EncodingALaw:
		// valid
	default:
		return nil, fmt.Errorf("sound: invalid Encoding %q", string(s.Encoding))
	}

	dict := pdf.Dict{
		"R": pdf.Number(s.SampleRate),
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Sound")
	}
	if c, ok := s.Channels.Get(); ok && c != 1 {
		dict["C"] = pdf.Integer(c)
	}
	if b, ok := s.BitsPerSample.Get(); ok && b != 8 {
		dict["B"] = pdf.Integer(b)
	}
	if s.Encoding != "" && s.Encoding != EncodingRaw {
		dict["E"] = pdf.Name(s.Encoding)
	}
	if s.CompressionFormat != "" {
		dict["CO"] = s.CompressionFormat
	}
	if s.CompressionParams != nil {
		dict["CP"] = s.CompressionParams
	}

	ref := e.Alloc()
	if err := s.Data.WriteStream(e, ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// Encoding identifies the sample encoding of a sound object.
type Encoding pdf.Name

// Sample encoding formats defined by the PDF specification.
const (
	// EncodingRaw is unspecified or unsigned values in the range 0 to 2^B-1.
	EncodingRaw Encoding = "Raw"

	// EncodingSigned is two's-complement signed values.
	EncodingSigned Encoding = "Signed"

	// EncodingMuLaw is mu-law-encoded samples.
	EncodingMuLaw Encoding = "muLaw"

	// EncodingALaw is A-law-encoded samples.
	EncodingALaw Encoding = "ALaw"
)
