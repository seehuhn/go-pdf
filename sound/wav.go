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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// WriteWAV writes the sample data of s to w as a RIFF/WAVE container
// with 16-bit signed little-endian PCM samples — the standard
// uncompressed WAV format accepted by most audio playback libraries.
//
// The output preserves the sample rate and channel count of s; stereo
// samples are interleaved.  Input samples are decoded from any of the
// four PDF encodings (Raw, Signed, muLaw, ALaw); only 8- and 16-bit
// input is supported.
//
// Compressed sounds (where s.CompressionFormat is non-empty) and
// external-file sources are not supported.
func WriteWAV(w io.Writer, s *Sound) error {
	if s == nil {
		return errors.New("sound: WriteWAV: nil Sound")
	}
	if s.SampleRate <= 0 || math.IsNaN(s.SampleRate) || math.IsInf(s.SampleRate, 0) {
		return errors.New("sound: WriteWAV: invalid SampleRate")
	}
	if s.SampleRate > math.MaxUint32 {
		return errors.New("sound: WriteWAV: SampleRate too large for WAV")
	}
	if s.CompressionFormat != "" {
		return fmt.Errorf("sound: WriteWAV: unsupported CompressionFormat %q", string(s.CompressionFormat))
	}
	if s.Data == nil {
		return errors.New("sound: WriteWAV: nil Data")
	}

	channels := uint(1)
	if c, ok := s.Channels.Get(); ok {
		channels = c
	}
	if channels == 0 || channels > math.MaxUint16 {
		return fmt.Errorf("sound: WriteWAV: invalid Channels=%d", channels)
	}

	bits := uint(8)
	if b, ok := s.BitsPerSample.Get(); ok {
		bits = b
	}

	encoding := s.Encoding
	if encoding == "" {
		encoding = EncodingRaw
	}
	switch encoding {
	case EncodingRaw, EncodingSigned:
		if bits != 8 && bits != 16 {
			return fmt.Errorf("sound: WriteWAV: unsupported BitsPerSample=%d for %s", bits, encoding)
		}
	case EncodingMuLaw, EncodingALaw:
		if bits != 8 {
			return fmt.Errorf("sound: WriteWAV: %s requires BitsPerSample=8 (got %d)", encoding, bits)
		}
	default:
		return fmt.Errorf("sound: WriteWAV: unknown Encoding %q", string(encoding))
	}

	rc, err := s.Data.Reader()
	if err != nil {
		return err
	}
	defer rc.Close()

	// buffer the input so we know its exact byte count: the RIFF and
	// data chunk-size fields in the WAV header sit at the start of the
	// file and must be correct for AVAudioPlayer-class consumers.
	// Memory is bounded by the inline-stream size.
	var inBuf bytes.Buffer
	inputLen, err := io.Copy(&inBuf, rc)
	if err != nil {
		return err
	}
	if inputLen == 0 {
		return errors.New("sound: WriteWAV: no sample data")
	}

	inputBytesPerSample := int64(bits / 8)
	if inputLen%inputBytesPerSample != 0 {
		return fmt.Errorf("sound: WriteWAV: input length %d is not a multiple of %d-bit sample size", inputLen, bits)
	}
	sampleCount := inputLen / inputBytesPerSample
	dataSize := sampleCount * 2 // 16-bit output
	if dataSize > 0xFFFFFFFF-36 {
		return errors.New("sound: WriteWAV: audio data exceeds 32-bit WAV size limit")
	}

	if err := writeWAVHeader(w, uint16(channels), uint32(s.SampleRate), uint32(dataSize)); err != nil {
		return err
	}
	return writeWAVSamples(w, inBuf.Bytes(), encoding, bits)
}

func writeWAVHeader(w io.Writer, channels uint16, sampleRate, dataSize uint32) error {
	const fmtChunkSize = 16
	const bitsPerSample = 16
	blockAlign := channels * (bitsPerSample / 8)
	byteRate := sampleRate * uint32(blockAlign)

	var hdr [44]byte
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], 36+dataSize)
	copy(hdr[8:12], "WAVE")
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], fmtChunkSize)
	binary.LittleEndian.PutUint16(hdr[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(hdr[22:24], channels)
	binary.LittleEndian.PutUint32(hdr[24:28], sampleRate)
	binary.LittleEndian.PutUint32(hdr[28:32], byteRate)
	binary.LittleEndian.PutUint16(hdr[32:34], blockAlign)
	binary.LittleEndian.PutUint16(hdr[34:36], bitsPerSample)
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], dataSize)
	_, err := w.Write(hdr[:])
	return err
}

// writeWAVSamples decodes input samples (PDF format) and writes 16-bit
// signed little-endian samples to w in 4 KiB chunks.
func writeWAVSamples(w io.Writer, in []byte, encoding Encoding, bits uint) error {
	const chunk = 4096
	out := make([]byte, 0, chunk)

	put := func(s int16) error {
		u := uint16(s)
		out = append(out, byte(u), byte(u>>8))
		if len(out) >= chunk {
			if _, err := w.Write(out); err != nil {
				return err
			}
			out = out[:0]
		}
		return nil
	}

	switch encoding {
	case EncodingRaw:
		if bits == 8 {
			for _, b := range in {
				if err := put(int16(int(b)-128) << 8); err != nil {
					return err
				}
			}
		} else { // 16
			for i := 0; i+1 < len(in); i += 2 {
				u := uint16(in[i])<<8 | uint16(in[i+1])
				if err := put(int16(int32(u) - 32768)); err != nil {
					return err
				}
			}
		}
	case EncodingSigned:
		if bits == 8 {
			for _, b := range in {
				if err := put(int16(int8(b)) << 8); err != nil {
					return err
				}
			}
		} else { // 16, big-endian
			for i := 0; i+1 < len(in); i += 2 {
				s := int16(uint16(in[i])<<8 | uint16(in[i+1]))
				if err := put(s); err != nil {
					return err
				}
			}
		}
	case EncodingMuLaw:
		for _, b := range in {
			if err := put(muLawDecode(b)); err != nil {
				return err
			}
		}
	case EncodingALaw:
		for _, b := range in {
			if err := put(aLawDecode(b)); err != nil {
				return err
			}
		}
	}

	if len(out) > 0 {
		if _, err := w.Write(out); err != nil {
			return err
		}
	}
	return nil
}

// muLawDecode returns the 16-bit signed PCM sample for a single G.711
// µ-law byte (Sun reference algorithm).
func muLawDecode(b byte) int16 {
	b = ^b
	t := (int16(b&0x0F) << 3) + 0x84
	t <<= (uint(b) & 0x70) >> 4
	if b&0x80 != 0 {
		return 0x84 - t
	}
	return t - 0x84
}

// aLawDecode returns the 16-bit signed PCM sample for a single G.711
// A-law byte (CCITT reference algorithm).
func aLawDecode(b byte) int16 {
	b ^= 0x55
	t := int16(b&0x0F) << 4
	seg := (b >> 4) & 0x07
	switch seg {
	case 0:
		t += 8
	case 1:
		t += 0x108
	default:
		t += 0x108
		t <<= (seg - 1)
	}
	if b&0x80 != 0 {
		return t
	}
	return -t
}
