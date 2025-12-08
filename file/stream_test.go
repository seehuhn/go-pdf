// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package file

import (
	"bytes"
	"crypto/md5"
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var streamTestCases = []struct {
	name    string
	version pdf.Version
	stream  *Stream
}{
	{
		name:    "minimal stream",
		version: pdf.V1_3,
		stream: &Stream{
			WriteData: func(w io.Writer) error {
				_, err := w.Write([]byte("Hello, World!"))
				return err
			},
		},
	},
	{
		name:    "stream with mime type",
		version: pdf.V1_3,
		stream: &Stream{
			MimeType: "text/plain",
			WriteData: func(w io.Writer) error {
				_, err := w.Write([]byte("Hello, World!"))
				return err
			},
		},
	},
	{
		name:    "stream with size",
		version: pdf.V1_3,
		stream: &Stream{
			Size: 42,
			WriteData: func(w io.Writer) error {
				_, err := w.Write(bytes.Repeat([]byte("A"), 42))
				return err
			},
		},
	},
	{
		name:    "stream with dates",
		version: pdf.V1_3,
		stream: &Stream{
			CreationDate: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			ModDate:      time.Date(2023, 6, 15, 14, 30, 0, 0, time.UTC),
			WriteData: func(w io.Writer) error {
				_, err := w.Write([]byte("Document content"))
				return err
			},
		},
	},
	{
		name:    "stream with checksum",
		version: pdf.V1_3,
		stream: &Stream{
			CheckSum: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
				0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
			WriteData: func(w io.Writer) error {
				_, err := w.Write([]byte("Content with checksum"))
				return err
			},
		},
	},
	{
		name:    "comprehensive stream",
		version: pdf.V1_7,
		stream: &Stream{
			MimeType:     "application/pdf",
			Size:         1024,
			CreationDate: time.Date(2023, 3, 15, 10, 30, 45, 0, time.UTC),
			ModDate:      time.Date(2023, 8, 22, 16, 45, 30, 0, time.UTC),
			CheckSum: func() []byte {
				hash := md5.Sum([]byte("comprehensive test data"))
				return hash[:]
			}(),
			WriteData: func(w io.Writer) error {
				_, err := w.Write([]byte("comprehensive test data"))
				return err
			},
		},
	},
}

func streamRoundTripTest(t *testing.T, version pdf.Version, stream *Stream) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)

	// Embed the stream
	rm := pdf.NewResourceManager(w)
	obj, err := rm.Embed(stream)
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("ResourceManager.Close failed: %v", err)
	}

	// Store in trailer for extraction
	w.GetMeta().Trailer["Quir:E"] = obj
	err = w.Close()
	if err != nil {
		t.Fatalf("Writer.Close failed: %v", err)
	}

	// Extract the stream back
	x := pdf.NewExtractor(w)
	streamObj := w.GetMeta().Trailer["Quir:E"]
	if streamObj == nil {
		t.Fatal("missing test object")
	}

	decoded, err := ExtractStream(x, streamObj)
	if err != nil {
		t.Fatalf("ExtractStream failed: %v", err)
	}

	// Compare fields (excluding WriteData which can't be compared directly)
	if diff := cmp.Diff(stream.MimeType, decoded.MimeType); diff != "" {
		t.Errorf("MimeType mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(stream.Size, decoded.Size); diff != "" {
		t.Errorf("Size mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(stream.CreationDate.Unix(), decoded.CreationDate.Unix()); diff != "" {
		t.Errorf("CreationDate mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(stream.ModDate.Unix(), decoded.ModDate.Unix()); diff != "" {
		t.Errorf("ModDate mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(stream.CheckSum, decoded.CheckSum); diff != "" {
		t.Errorf("CheckSum mismatch (-want +got):\n%s", diff)
	}

	// Test WriteData functionality by comparing output
	var originalBuf, decodedBuf bytes.Buffer
	if stream.WriteData != nil {
		err = stream.WriteData(&originalBuf)
		if err != nil {
			t.Fatalf("Original WriteData failed: %v", err)
		}
	}

	if decoded.WriteData != nil {
		err = decoded.WriteData(&decodedBuf)
		if err != nil {
			t.Fatalf("Decoded WriteData failed: %v", err)
		}
	}

	if diff := cmp.Diff(originalBuf.Bytes(), decodedBuf.Bytes()); diff != "" {
		t.Errorf("WriteData output mismatch (-want +got):\n%s", diff)
	}
}

func TestStreamRoundTrip(t *testing.T) {
	for _, tc := range streamTestCases {
		t.Run(tc.name, func(t *testing.T) {
			streamRoundTripTest(t, tc.version, tc.stream)
		})
	}
}

func TestStreamValidation(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_3, nil)
	rm := pdf.NewResourceManager(w)

	// Test missing WriteData
	t.Run("missing WriteData", func(t *testing.T) {
		stream := &Stream{
			MimeType: "text/plain",
		}
		_, err := rm.Embed(stream)
		if err == nil {
			t.Error("Expected error for missing WriteData")
		}
	})

	// Test invalid CheckSum length
	t.Run("invalid checksum length", func(t *testing.T) {
		stream := &Stream{
			CheckSum: []byte{0x01, 0x02, 0x03}, // Wrong length
			WriteData: func(w io.Writer) error {
				return nil
			},
		}
		_, err := rm.Embed(stream)
		if err == nil {
			t.Error("Expected error for invalid CheckSum length")
		}
	})

	rm.Close()
	w.Close()
}

func TestStreamVersionRequirement(t *testing.T) {
	// Test that PDF 1.2 fails
	w, _ := memfile.NewPDFWriter(pdf.V1_2, nil)
	rm := pdf.NewResourceManager(w)

	stream := &Stream{
		WriteData: func(w io.Writer) error {
			_, err := w.Write([]byte("test"))
			return err
		},
	}

	_, err := rm.Embed(stream)
	if err == nil {
		t.Error("Expected version error for PDF 1.2")
	}

	rm.Close()
	w.Close()
}

func FuzzStreamRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range streamTestCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)
		rm := pdf.NewResourceManager(w)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		obj, err := rm.Embed(tc.stream)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = obj
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		stream, err := ExtractStream(x, objPDF)
		if err != nil {
			t.Skip("malformed stream object")
		}

		streamRoundTripTest(t, pdf.V1_7, stream)
	})
}
