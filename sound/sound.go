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
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/opaque"
)

// PDF 2.0 sections: 13.3

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

	// Channels is the number of sound channels.  Must be set to a
	// positive value.  In the PDF representation a missing /C entry
	// stands for 1, but the Go field always carries an explicit count.
	Channels int

	// BitsPerSample is the number of bits per sample value per channel.
	// Must be set to a positive value.  In the PDF representation a
	// missing /B entry stands for 8, but the Go field always carries an
	// explicit count.
	BitsPerSample int

	// Encoding specifies the sample encoding format.  It must be one of
	// the named [Encoding] constants or the zero value (treated as
	// [EncodingRaw] on write).
	Encoding Encoding

	// CompressionFormat (optional) names a sound compression format
	// applied to the sample data.  This is independent of the stream
	// filter chain.  The PDF specification defines no standard values.
	CompressionFormat pdf.Name

	// CompressionParams (optional) holds parameters specific to the
	// sound compression format.  References inside the wrapped value
	// are translated when the Sound is embedded into a different file.
	CompressionParams *opaque.Object

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
// sounds read from a PDF file.  It delegates to [opaque.Stream] for
// both decoded reads and verbatim cross-file re-emission, so internal
// references in /Filter and /DecodeParms are translated correctly
// when the sound moves between files.
type streamSource struct {
	inner *opaque.Stream
}

// Reader implements [Source] by decoding the source stream through its
// filter chain.
func (s *streamSource) Reader() (io.ReadCloser, error) {
	return s.inner.Reader()
}

// WriteStream implements [Source] by copying the raw, still-encoded
// stream bytes verbatim to the destination.
func (s *streamSource) WriteStream(e *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	return s.inner.WriteAt(e, ref, dict)
}

// Extract reads a sound object from the PDF file.  The isDirect parameter
// is accepted for signature uniformity but ignored, because sound objects
// are always indirect stream objects.
//
// Extract aborts with an error if the sound has obviously invalid
// metadata (negative or non-finite SampleRate, zero or absurdly large
// Channels or BitsPerSample, etc.).  Unknown Encoding values are
// substituted with the default EncodingRaw.
func Extract(c pdf.Cursor, obj pdf.Object, _ bool) (*Sound, error) {
	stream, err := c.Stream(obj)
	if err != nil {
		return nil, err
	} else if stream == nil {
		return nil, pdf.Error("missing sound stream")
	}
	if err := c.CheckDictType(stream.Dict, "Sound"); err != nil {
		return nil, err
	}

	res := &Sound{
		Channels:      1, // PDF default for missing /C
		BitsPerSample: 8, // PDF default for missing /B
		Encoding:      EncodingRaw,
	}

	// SampleRate (R) — required.
	if r, err := pdf.Optional(c.Number(stream.Dict["R"])); err != nil {
		return nil, err
	} else {
		res.SampleRate = r
	}

	// Channels (C) — optional in PDF, default 1.  When present, the
	// value must be a positive integer in the supported range;
	// non-integer or out-of-range values are rejected rather than
	// silently fixed.
	if cObj := stream.Dict["C"]; cObj != nil {
		c, err := c.Integer(cObj)
		if err != nil {
			return nil, err
		}
		if c <= 0 || c > maxChannels {
			return nil, pdf.Errorf("sound: invalid C entry %d", c)
		}
		res.Channels = int(c)
	}

	// BitsPerSample (B) — optional in PDF, default 8.  Same strictness
	// as /C: non-integer or out-of-range values are rejected.
	if bObj := stream.Dict["B"]; bObj != nil {
		b, err := c.Integer(bObj)
		if err != nil {
			return nil, err
		}
		if b <= 0 || b > maxBitsPerSample {
			return nil, pdf.Errorf("sound: invalid B entry %d", b)
		}
		res.BitsPerSample = int(b)
	}

	// Encoding (E) — optional, default Raw; unknown names ignored.
	if enc, err := pdf.Optional(c.Name(stream.Dict["E"])); err != nil {
		return nil, err
	} else if enc != "" {
		switch Encoding(enc) {
		case EncodingRaw, EncodingSigned, EncodingMuLaw, EncodingALaw:
			res.Encoding = Encoding(enc)
		}
	}

	// CompressionFormat (CO) — optional, opaque.
	if co, err := pdf.Optional(c.Name(stream.Dict["CO"])); err != nil {
		return nil, err
	} else if co != "" {
		res.CompressionFormat = co
	}

	// CompressionParams (CP) — optional, opaque.  References inside CP
	// are translated to the destination on Embed.
	if cp := stream.Dict["CP"]; cp != nil {
		res.CompressionParams = opaque.Extract(c.Extractor(), cp)
	}

	// ExternalFile (F) — optional.  When present, the sample data lives
	// in an external file; the inline stream body is ignored.
	externalFile, err := pdf.DecodeOptional(c, stream.Dict["F"], file.ExtractSpecification)
	if err != nil {
		return nil, err
	}
	if externalFile != nil {
		res.Data = &ExternalFileSource{File: externalFile}
	} else {
		res.Data = &streamSource{inner: opaque.ExtractStream(c.Extractor(), stream)}
	}

	if err := res.Validate(); err != nil {
		return nil, err
	}
	return res, nil
}

// Plausibility limits for sound metadata.  These are tighter than the
// WAV-format limits: any real sound annotation falls well inside them,
// so values outside indicate a malformed or hostile file.
const (
	// maxChannels accommodates immersive audio layouts (Dolby Atmos and
	// similar) up to 16 channels, well beyond the 7.1 ceiling of common
	// surround formats and the 1–2 channels recommended by the PDF
	// specification.  This is a sanity bound, not a spec limit.
	maxChannels      = 16
	maxBitsPerSample = 32      // covers all common PCM bit depths
	maxSampleRate    = 1 << 20 // 1 MHz, well above any consumer audio rate
)

// Validate reports whether s holds plausible metadata for a PDF sound
// object.  It checks that SampleRate is positive, finite, and within
// realistic bounds, that Channels lies in 1–16, that BitsPerSample
// lies in 1–32, that Encoding is one of the named constants (or empty,
// treated as Raw on write), and that Data is non-nil.  When Encoding
// is [EncodingMuLaw] or [EncodingALaw], BitsPerSample must be 8: those
// are 8-bit-by-definition G.711 codecs.
//
// Validate is called by both [Sound.Embed] (to refuse writing a
// malformed sound) and [Extract] (to refuse loading one).  External
// callers may use it to pre-validate a sound built in memory.
func (s *Sound) Validate() error {
	if s == nil {
		return errors.New("sound: nil Sound")
	}
	if s.SampleRate <= 0 || math.IsNaN(s.SampleRate) || math.IsInf(s.SampleRate, 0) {
		return errors.New("sound: SampleRate must be positive and finite")
	}
	if s.SampleRate > maxSampleRate {
		return fmt.Errorf("sound: implausible SampleRate %g", s.SampleRate)
	}
	if s.Channels <= 0 || s.Channels > maxChannels {
		return fmt.Errorf("sound: invalid Channels %d", s.Channels)
	}
	if s.BitsPerSample <= 0 || s.BitsPerSample > maxBitsPerSample {
		return fmt.Errorf("sound: invalid BitsPerSample %d", s.BitsPerSample)
	}
	switch s.Encoding {
	case "", EncodingRaw, EncodingSigned:
		// valid; bit depth unconstrained at the object level
	case EncodingMuLaw, EncodingALaw:
		if s.BitsPerSample != 8 {
			return fmt.Errorf("sound: %s requires BitsPerSample=8 (got %d)", s.Encoding, s.BitsPerSample)
		}
	default:
		return fmt.Errorf("sound: invalid Encoding %q", string(s.Encoding))
	}
	if s.Data == nil {
		return errors.New("sound: Data is required")
	}
	return nil
}

// Embed writes the sound object to the PDF file as an indirect stream
// object and returns a reference to it.
func (s *Sound) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "sound object", pdf.V1_2); err != nil {
		return nil, err
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"R": pdf.Number(s.SampleRate),
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Sound")
	}
	if s.Channels != 1 {
		dict["C"] = pdf.Integer(s.Channels)
	}
	if s.BitsPerSample != 8 {
		dict["B"] = pdf.Integer(s.BitsPerSample)
	}
	if s.Encoding != "" && s.Encoding != EncodingRaw {
		dict["E"] = pdf.Name(s.Encoding)
	}
	if s.CompressionFormat != "" {
		dict["CO"] = s.CompressionFormat
	}
	if s.CompressionParams != nil {
		cp, err := e.Embed(s.CompressionParams)
		if err != nil {
			return nil, err
		}
		dict["CP"] = cp
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
