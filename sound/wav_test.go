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
	"io"
	"testing"

	"seehuhn.de/go/pdf/optional"
)

// makeSound returns a Sound whose inline data is the given bytes.
func makeSound(rate float64, channels, bits uint, enc Encoding, data []byte) *Sound {
	s := &Sound{
		SampleRate: rate,
		Encoding:   enc,
		Data: &InlineSource{
			WriteData: func(w io.Writer) error {
				_, err := w.Write(data)
				return err
			},
		},
	}
	if channels != 1 {
		s.Channels = optional.NewUInt(channels)
	}
	if bits != 8 {
		s.BitsPerSample = optional.NewUInt(bits)
	}
	return s
}

func parseHeader(t *testing.T, b []byte) (channels uint16, rate uint32, bits uint16, dataSize uint32) {
	t.Helper()
	if len(b) < 44 {
		t.Fatalf("WAV output too short: %d bytes", len(b))
	}
	if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		t.Fatalf("missing RIFF/WAVE marker")
	}
	if string(b[12:16]) != "fmt " {
		t.Fatalf("missing fmt  chunk")
	}
	if string(b[36:40]) != "data" {
		t.Fatalf("missing data chunk")
	}
	if got := binary.LittleEndian.Uint16(b[20:22]); got != 1 {
		t.Fatalf("audio format = %d, want 1 (PCM)", got)
	}
	channels = binary.LittleEndian.Uint16(b[22:24])
	rate = binary.LittleEndian.Uint32(b[24:28])
	bits = binary.LittleEndian.Uint16(b[34:36])
	dataSize = binary.LittleEndian.Uint32(b[40:44])
	return
}

func TestWriteWAVHeader(t *testing.T) {
	s := makeSound(22050, 2, 16, EncodingSigned, []byte{0, 0, 0, 0})
	var buf bytes.Buffer
	if err := WriteWAV(&buf, s); err != nil {
		t.Fatalf("WriteWAV: %v", err)
	}
	channels, rate, bits, dataSize := parseHeader(t, buf.Bytes())
	if channels != 2 || rate != 22050 || bits != 16 {
		t.Errorf("header = (channels=%d, rate=%d, bits=%d), want (2, 22050, 16)", channels, rate, bits)
	}
	// 4 input bytes / 2 (16-bit input) = 2 samples; 2 samples * 2 (16-bit output) = 4
	if dataSize != 4 {
		t.Errorf("data size = %d, want 4", dataSize)
	}
	if int(dataSize)+44 != buf.Len() {
		t.Errorf("file size = %d, want %d", buf.Len(), dataSize+44)
	}
}

func TestWriteWAVSampleDecodingRaw8(t *testing.T) {
	// 8-bit unsigned with values 0, 128, 255 → s16 -32768, 0, 32512
	in := []byte{0, 128, 255}
	want := []int16{-32768, 0, 32512}
	checkSamples(t, makeSound(8000, 1, 8, EncodingRaw, in), want)
}

func TestWriteWAVSampleDecodingRaw16(t *testing.T) {
	// big-endian unsigned 16-bit: 0x0000, 0x8000, 0xFFFF
	in := []byte{0x00, 0x00, 0x80, 0x00, 0xFF, 0xFF}
	want := []int16{-32768, 0, 32767}
	checkSamples(t, makeSound(8000, 1, 16, EncodingRaw, in), want)
}

func TestWriteWAVSampleDecodingSigned8(t *testing.T) {
	in := []byte{0x80, 0x00, 0x7F} // -128, 0, 127
	want := []int16{-32768, 0, 32512}
	checkSamples(t, makeSound(8000, 1, 8, EncodingSigned, in), want)
}

func TestWriteWAVSampleDecodingSigned16(t *testing.T) {
	in := []byte{0x80, 0x00, 0x00, 0x00, 0x7F, 0xFF}
	want := []int16{-32768, 0, 32767}
	checkSamples(t, makeSound(8000, 1, 16, EncodingSigned, in), want)
}

func TestWriteWAVSampleDecodingMuLaw(t *testing.T) {
	// Hand-computed reference values from the Sun µ-law algorithm.
	// Each input exercises a different segment/mantissa/sign so that a
	// regression in muLawDecode would shift at least one expected value:
	//   0xFF: positive zero (seg 0, mant 0, +)              →     0
	//   0x7F: negative zero (seg 0, mant 0, −)              →     0
	//   0x70: smallest-magnitude negative in seg 0          →  −120
	//   0x00: largest-magnitude negative (seg 7, mant 15)   → −32124
	//   0x80: largest-magnitude positive (seg 7, mant 15)   → +32124
	//   0xAA: seg 5, mant 5, positive                       →  +5372
	in := []byte{0xFF, 0x7F, 0x70, 0x00, 0x80, 0xAA}
	want := []int16{0, 0, -120, -32124, 32124, 5372}
	checkSamples(t, makeSound(8000, 1, 8, EncodingMuLaw, in), want)
}

func TestWriteWAVSampleDecodingALaw(t *testing.T) {
	// Hand-computed reference values from the CCITT G.711 A-law
	// algorithm.  A-law has no exact zero — its canonical zero codes
	// decode to ±8.
	//   0xD5: positive zero (seg 0, mant 0, +)              →     +8
	//   0x55: negative zero                                 →     −8
	//   0xAA: largest-magnitude positive (seg 7, mant 15)   → +32256
	//   0x2A: largest-magnitude negative                    → −32256
	//   0xE5: seg 3, mant 0, positive                       →  +1056
	in := []byte{0xD5, 0x55, 0xAA, 0x2A, 0xE5}
	want := []int16{8, -8, 32256, -32256, 1056}
	checkSamples(t, makeSound(8000, 1, 8, EncodingALaw, in), want)
}

func TestWriteWAVRejectsCompression(t *testing.T) {
	s := makeSound(8000, 1, 8, EncodingRaw, []byte{0, 1, 2})
	s.CompressionFormat = "GZip"
	if err := WriteWAV(io.Discard, s); err == nil {
		t.Fatal("expected error for set CompressionFormat")
	}
}

func TestWriteWAVRejectsOddBytes16(t *testing.T) {
	s := makeSound(8000, 1, 16, EncodingSigned, []byte{0, 1, 2})
	if err := WriteWAV(io.Discard, s); err == nil {
		t.Fatal("expected error for odd byte count at 16 bits")
	}
}

func TestWriteWAVRejectsUnsupportedBits(t *testing.T) {
	s := makeSound(8000, 1, 12, EncodingSigned, []byte{0, 1, 2})
	if err := WriteWAV(io.Discard, s); err == nil {
		t.Fatal("expected error for 12-bit input")
	}
}

func TestWriteWAVRejectsMuLaw16(t *testing.T) {
	s := makeSound(8000, 1, 16, EncodingMuLaw, []byte{0, 1, 2, 3})
	if err := WriteWAV(io.Discard, s); err == nil {
		t.Fatal("expected error for µ-law at 16 bits")
	}
}

// checkSamples writes the sound to a WAV blob and verifies the sample
// portion (after the 44-byte header) matches want as int16 little-endian.
func checkSamples(t *testing.T, s *Sound, want []int16) {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteWAV(&buf, s); err != nil {
		t.Fatalf("WriteWAV: %v", err)
	}
	body := buf.Bytes()[44:]
	if len(body) != len(want)*2 {
		t.Fatalf("body length = %d, want %d", len(body), len(want)*2)
	}
	for i, w := range want {
		got := int16(binary.LittleEndian.Uint16(body[2*i : 2*i+2]))
		if got != w {
			t.Errorf("sample %d = %d, want %d", i, got, w)
		}
	}
}
