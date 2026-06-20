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
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/opaque"
)

// inlineSourceWith returns an *InlineSource whose WriteData emits the
// given bytes.
func inlineSourceWith(sample []byte) *InlineSource {
	return &InlineSource{
		WriteData: func(out io.Writer) error {
			_, err := out.Write(sample)
			return err
		},
	}
}

// readSampleBytes returns the decoded sample bytes from a Source.
func readSampleBytes(t *testing.T, src Source) []byte {
	t.Helper()
	r, err := src.Reader()
	if err != nil {
		t.Fatalf("Reader: %v", err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return data
}

func TestEmbedMinimal(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_3, nil)
	rm := pdf.NewResourceManager(w)

	s := &Sound{
		SampleRate:    22050,
		Channels:      1,
		BitsPerSample: 8,
		Data:          inlineSourceWith(bytes.Repeat([]byte{0x80}, 16)),
	}

	obj, err := rm.Embed(s)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}
	if _, ok := obj.(pdf.Reference); !ok {
		t.Fatalf("embed returned %T, want pdf.Reference", obj)
	}

	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}
}

func TestExtractMinimal(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_3, nil)
	rm := pdf.NewResourceManager(w)

	original := &Sound{
		SampleRate:    22050,
		Channels:      1,
		BitsPerSample: 8,
		Data:          inlineSourceWith(bytes.Repeat([]byte{0x80}, 16)),
	}
	obj, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}
	w.GetMeta().Trailer["Quir:E"] = obj
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}

	x := pdf.NewExtractor(w)
	got, err := Extract(pdf.CursorAt(x, nil), w.GetMeta().Trailer["Quir:E"], false)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	want := &Sound{
		SampleRate:    22050,
		Channels:      1,
		BitsPerSample: 8,
		Encoding:      EncodingRaw,
	}
	if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(Sound{}, "Data")); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}

	if want := bytes.Repeat([]byte{0x80}, 16); !bytes.Equal(readSampleBytes(t, got.Data), want) {
		t.Errorf("sample bytes mismatch")
	}
}

type roundTripCase struct {
	name    string
	version pdf.Version
	sample  []byte // nil for external-file cases
	sound   *Sound // Data left nil; populated by helper from sample
}

var roundTripCases = []roundTripCase{
	{
		name:    "minimal V1.7",
		version: pdf.V1_7,
		sample:  bytes.Repeat([]byte{0x80}, 16),
		sound: &Sound{
			SampleRate:    22050,
			Channels:      1,
			BitsPerSample: 8,
		},
	},
	{
		name:    "minimal V2.0",
		version: pdf.V2_0,
		sample:  bytes.Repeat([]byte{0x80}, 16),
		sound: &Sound{
			SampleRate:    22050,
			Channels:      1,
			BitsPerSample: 8,
		},
	},
	{
		name:    "explicit defaults",
		version: pdf.V1_7,
		sample:  bytes.Repeat([]byte{0x00}, 16),
		sound: &Sound{
			SampleRate:    22050,
			Channels:      1,
			BitsPerSample: 8,
			Encoding:      EncodingRaw,
		},
	},
	{
		name:    "stereo signed 16-bit 44.1kHz",
		version: pdf.V1_7,
		sample:  bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 8),
		sound: &Sound{
			SampleRate:    44100,
			Channels:      2,
			BitsPerSample: 16,
			Encoding:      EncodingSigned,
		},
	},
	{
		name:    "muLaw mono 8000Hz",
		version: pdf.V1_7,
		sample:  bytes.Repeat([]byte{0x7f}, 32),
		sound: &Sound{
			SampleRate:    8000,
			Channels:      1,
			BitsPerSample: 8,
			Encoding:      EncodingMuLaw,
		},
	},
	{
		name:    "ALaw stereo",
		version: pdf.V2_0,
		sample:  bytes.Repeat([]byte{0xaa, 0x55}, 16),
		sound: &Sound{
			SampleRate:    8000,
			Channels:      2,
			BitsPerSample: 8,
			Encoding:      EncodingALaw,
		},
	},
	{
		name:    "with sound compression",
		version: pdf.V1_7,
		sample:  []byte("opaque-encoded-bytes"),
		sound: &Sound{
			SampleRate:        22050,
			Channels:          1,
			BitsPerSample:     8,
			CompressionFormat: "ExampleCodec",
			CompressionParams: opaque.Direct(pdf.Dict{"Quality": pdf.Integer(5)}),
		},
	},
	{
		name:    "FlateDecode-compressed inline samples",
		version: pdf.V1_7,
		sample:  bytes.Repeat([]byte{0x80, 0x81, 0x82, 0x83}, 64),
		sound: &Sound{
			SampleRate:    22050,
			Channels:      1,
			BitsPerSample: 8,
			Data: &InlineSource{
				Filter: []pdf.Filter{pdf.FilterFlate{}},
			},
		},
	},
	{
		name:    "external file (no inline samples)",
		version: pdf.V1_7,
		sample:  nil,
		sound: &Sound{
			SampleRate:    22050,
			Channels:      1,
			BitsPerSample: 8,
			Data: &ExternalFileSource{
				File: &file.Specification{
					FileName:       "samples.au",
					AFRelationship: file.RelationshipUnspecified,
				},
			},
		},
	},
}

func roundTripTest(t *testing.T, tc roundTripCase) {
	t.Helper()

	original := *tc.sound
	if original.Data == nil {
		original.Data = inlineSourceWith(tc.sample)
	} else if inline, ok := original.Data.(*InlineSource); ok && inline.WriteData == nil {
		sample := tc.sample
		inline.WriteData = func(out io.Writer) error {
			_, err := out.Write(sample)
			return err
		}
	}

	w, _ := memfile.NewPDFWriter(tc.version, nil)
	rm := pdf.NewResourceManager(w)
	obj, err := rm.Embed(&original)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}
	w.GetMeta().Trailer["Quir:E"] = obj
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}

	x := pdf.NewExtractor(w)
	got, err := Extract(pdf.CursorAt(x, nil), w.GetMeta().Trailer["Quir:E"], false)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	want := *tc.sound
	if want.Channels == 0 {
		want.Channels = 1
	}
	if want.BitsPerSample == 0 {
		want.BitsPerSample = 8
	}
	if want.Encoding == "" {
		want.Encoding = EncodingRaw
	}
	if diff := cmp.Diff(&want, got, cmpopts.IgnoreFields(Sound{}, "Data")); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}

	if tc.sample != nil {
		if !bytes.Equal(readSampleBytes(t, got.Data), tc.sample) {
			t.Errorf("sample bytes mismatch")
		}
	} else {
		ext, ok := got.Data.(*ExternalFileSource)
		if !ok {
			t.Fatalf("expected *ExternalFileSource, got %T", got.Data)
		}
		wantExt := tc.sound.Data.(*ExternalFileSource)
		if diff := cmp.Diff(wantExt.File, ext.File); diff != "" {
			t.Errorf("external file mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range roundTripCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, tc)
		})
	}
}

// TestCompressionParamsCrossFile verifies that a /CP dict containing an
// indirect reference to another object in the source file survives a
// cross-file embed: references are translated, the destination has no
// dangling pointers, and the round-tripped Sound compares equal.
func TestCompressionParamsCrossFile(t *testing.T) {
	// Build source: an inner dict referenced from the /CP dict of a Sound.
	src, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := memfile.AddBlankPage(src); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
	innerRef := src.Alloc()
	if err := src.Put(innerRef, pdf.Dict{"Quality": pdf.Integer(7)}); err != nil {
		t.Fatalf("Put inner: %v", err)
	}
	cp := pdf.Dict{
		"Profile": innerRef,
		"Tag":     pdf.Name("ExampleProfile"),
	}

	srcRM := pdf.NewResourceManager(src)
	original := &Sound{
		SampleRate:        22050,
		Channels:          1,
		BitsPerSample:     8,
		CompressionFormat: "ExampleCodec",
		CompressionParams: opaque.Direct(cp),
		Data:              inlineSourceWith([]byte("opaque-encoded-bytes")),
	}
	srcSoundRef, err := srcRM.Embed(original)
	if err != nil {
		t.Fatalf("src Embed: %v", err)
	}
	if err := srcRM.Close(); err != nil {
		t.Fatalf("srcRM.Close: %v", err)
	}
	src.GetMeta().Trailer["Quir:E"] = srcSoundRef
	if err := src.Close(); err != nil {
		t.Fatalf("src.Close: %v", err)
	}

	// Read the Sound from the source.
	srcX := pdf.NewExtractor(src)
	srcSnd, err := Extract(pdf.CursorAt(srcX, nil), src.GetMeta().Trailer["Quir:E"], false)
	if err != nil {
		t.Fatalf("src Extract: %v", err)
	}
	if srcSnd.CompressionParams == nil {
		t.Fatal("source CompressionParams is nil after Extract")
	}

	// Re-embed into a different file.
	dst, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	dstRM := pdf.NewResourceManager(dst)
	dstSoundRef, err := dstRM.Embed(srcSnd)
	if err != nil {
		t.Fatalf("dst Embed: %v", err)
	}
	if err := dstRM.Close(); err != nil {
		t.Fatalf("dstRM.Close: %v", err)
	}
	dst.GetMeta().Trailer["Quir:E"] = dstSoundRef
	if err := dst.Close(); err != nil {
		t.Fatalf("dst.Close: %v", err)
	}

	// Verify the destination's /CP entry: the inner reference must be
	// fresh (not the source's), and resolve to the expected payload.
	dstX := pdf.NewExtractor(dst)
	dstStream, err := pdf.CursorAt(dstX, nil).Stream(dst.GetMeta().Trailer["Quir:E"])
	if err != nil {
		t.Fatalf("dst GetStream: %v", err)
	}
	dstCP, err := pdf.CursorAt(dstX, nil).Dict(dstStream.Dict["CP"])
	if err != nil {
		t.Fatalf("dst /CP GetDict: %v", err)
	}
	dstProfileRef, ok := dstCP["Profile"].(pdf.Reference)
	if !ok {
		t.Fatalf("dst /CP/Profile is %T, want pdf.Reference", dstCP["Profile"])
	}
	if dstProfileRef == innerRef {
		t.Error("dst reused the source-file reference for /CP/Profile")
	}
	dstProfile, err := pdf.CursorAt(dstX, nil).Dict(dstProfileRef)
	if err != nil {
		t.Fatalf("dst Profile GetDict: %v", err)
	}
	if dstProfile["Quality"] != pdf.Integer(7) {
		t.Errorf("dst Profile.Quality = %v, want 7", dstProfile["Quality"])
	}

	// Round-trip equality of the wrapped CompressionParams.
	dstSnd, err := Extract(pdf.CursorAt(dstX, nil), dst.GetMeta().Trailer["Quir:E"], false)
	if err != nil {
		t.Fatalf("dst Extract: %v", err)
	}
	if !srcSnd.CompressionParams.Equal(dstSnd.CompressionParams) {
		t.Error("CompressionParams.Equal across files = false")
	}
}

func TestEmbedValidation(t *testing.T) {
	cases := []struct {
		name  string
		sound *Sound
	}{
		{
			name: "missing SampleRate",
			sound: &Sound{
				Channels:      1,
				BitsPerSample: 8,
				Data:          inlineSourceWith(nil),
			},
		},
		{
			name: "negative SampleRate",
			sound: &Sound{
				SampleRate:    -1,
				Channels:      1,
				BitsPerSample: 8,
				Data:          inlineSourceWith(nil),
			},
		},
		{
			name: "implausible SampleRate",
			sound: &Sound{
				SampleRate:    2 << 20, // > maxSampleRate
				Channels:      1,
				BitsPerSample: 8,
				Data:          inlineSourceWith(nil),
			},
		},
		{
			name: "explicit zero Channels",
			sound: &Sound{
				SampleRate:    22050,
				Channels:      0,
				BitsPerSample: 8,
				Data:          inlineSourceWith(nil),
			},
		},
		{
			name: "channels above limit",
			sound: &Sound{
				SampleRate:    22050,
				Channels:      maxChannels + 1,
				BitsPerSample: 8,
				Data:          inlineSourceWith(nil),
			},
		},
		{
			name: "explicit zero BitsPerSample",
			sound: &Sound{
				SampleRate:    22050,
				Channels:      1,
				BitsPerSample: 0,
				Data:          inlineSourceWith(nil),
			},
		},
		{
			name: "bits above limit",
			sound: &Sound{
				SampleRate:    22050,
				Channels:      1,
				BitsPerSample: maxBitsPerSample + 1,
				Data:          inlineSourceWith(nil),
			},
		},
		{
			name: "missing Data",
			sound: &Sound{
				SampleRate:    22050,
				Channels:      1,
				BitsPerSample: 8,
			},
		},
		{
			name: "InlineSource missing WriteData",
			sound: &Sound{
				SampleRate:    22050,
				Channels:      1,
				BitsPerSample: 8,
				Data:          &InlineSource{},
			},
		},
		{
			name: "ExternalFileSource missing File",
			sound: &Sound{
				SampleRate:    22050,
				Channels:      1,
				BitsPerSample: 8,
				Data:          &ExternalFileSource{},
			},
		},
		{
			name: "unknown Encoding",
			sound: &Sound{
				SampleRate:    22050,
				Channels:      1,
				BitsPerSample: 8,
				Encoding:      "MP3",
				Data:          inlineSourceWith(nil),
			},
		},
		{
			name: "muLaw with non-8-bit",
			sound: &Sound{
				SampleRate:    22050,
				Channels:      1,
				BitsPerSample: 16,
				Encoding:      EncodingMuLaw,
				Data:          inlineSourceWith(nil),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(tc.sound); err == nil {
				t.Errorf("expected error, got nil")
			}
			rm.Close()
			w.Close()
		})
	}
}

// TestStreamSourcePreservesFilter verifies that a sound stream carrying
// a /Filter chain in its source PDF retains the same /Filter chain when
// re-embedded into a different PDF, without going through a
// decode-then-re-encode cycle.
func TestStreamSourcePreservesFilter(t *testing.T) {
	sample := bytes.Repeat([]byte{0x80, 0x81, 0x82, 0x83}, 64)

	src, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(src)
	original := &Sound{
		SampleRate:    22050,
		Channels:      1,
		BitsPerSample: 8,
		Data: &InlineSource{
			WriteData: func(w io.Writer) error {
				_, err := w.Write(sample)
				return err
			},
			Filter: []pdf.Filter{pdf.FilterFlate{}},
		},
	}
	obj, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}
	src.GetMeta().Trailer["Quir:E"] = obj
	if err := src.Close(); err != nil {
		t.Fatalf("src.Close: %v", err)
	}

	x := pdf.NewExtractor(src)
	got, err := Extract(pdf.CursorAt(x, nil), src.GetMeta().Trailer["Quir:E"], false)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	dst, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm2 := pdf.NewResourceManager(dst)
	obj2, err := rm2.Embed(got)
	if err != nil {
		t.Fatalf("re-embed: %v", err)
	}
	if err := rm2.Close(); err != nil {
		t.Fatalf("rm2.Close: %v", err)
	}
	if err := dst.Close(); err != nil {
		t.Fatalf("dst.Close: %v", err)
	}

	ref := obj2.(pdf.Reference)
	stream, err := pdf.NewCursor(dst).Stream(ref)
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	filter, _ := pdf.NewCursor(dst).Name(stream.Dict["Filter"])
	if filter != "FlateDecode" {
		t.Errorf("re-embedded /Filter = %q, want FlateDecode", filter)
	}
}

// TestExtractUnknownEncoding verifies that an unknown /E value in the
// stream dictionary is silently substituted with the default EncodingRaw,
// preserving the read-write-read round-trip property.
func TestExtractUnknownEncoding(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	ref := w.Alloc()
	dict := pdf.Dict{
		"R": pdf.Number(22050),
		"E": pdf.Name("MP3"), // not a valid encoding
	}
	stm, err := w.OpenStream(ref, dict)
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if _, err := stm.Write(bytes.Repeat([]byte{0x80}, 4)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := stm.Close(); err != nil {
		t.Fatalf("close stream: %v", err)
	}
	w.GetMeta().Trailer["Quir:E"] = ref
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}

	x := pdf.NewExtractor(w)
	got, err := Extract(pdf.CursorAt(x, nil), w.GetMeta().Trailer["Quir:E"], false)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if got.Encoding != EncodingRaw {
		t.Errorf("Encoding = %q, want %q", got.Encoding, EncodingRaw)
	}
}

func TestEmbedVersionRequirement(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_1, nil)
	rm := pdf.NewResourceManager(w)
	defer w.Close()
	defer rm.Close()

	s := &Sound{
		SampleRate:    22050,
		Channels:      1,
		BitsPerSample: 8,
		Data:          inlineSourceWith(nil),
	}
	_, err := rm.Embed(s)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !pdf.IsWrongVersion(err) {
		t.Errorf("expected wrong-version error, got %v", err)
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range roundTripCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)
		if err := memfile.AddBlankPage(w); err != nil {
			continue
		}
		rm := pdf.NewResourceManager(w)
		s := *tc.sound
		if s.Data == nil {
			s.Data = inlineSourceWith(tc.sample)
		} else if inline, ok := s.Data.(*InlineSource); ok && inline.WriteData == nil {
			sample := tc.sample
			inline.WriteData = func(out io.Writer) error {
				_, err := out.Write(sample)
				return err
			}
		}
		obj, err := rm.Embed(&s)
		if err != nil {
			continue
		}
		if err := rm.Close(); err != nil {
			continue
		}
		w.GetMeta().Trailer["Quir:E"] = obj
		if err := w.Close(); err != nil {
			continue
		}
		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing test object")
		}
		x := pdf.NewExtractor(r)
		first, err := Extract(pdf.CursorAt(x, nil), objPDF, false)
		if err != nil {
			t.Skip("malformed sound object")
		}

		// Capture inline samples up-front so the second embed does not
		// depend on the source PDF's stream remaining valid.
		var firstSamples []byte
		var external bool
		if _, ok := first.Data.(*ExternalFileSource); ok {
			external = true
		} else {
			rd, err := first.Data.Reader()
			if err != nil {
				t.Skip("first Reader failed")
			}
			firstSamples, err = io.ReadAll(rd)
			rd.Close()
			if err != nil {
				t.Skip("first ReadAll failed")
			}
			first.Data = inlineSourceWith(firstSamples)
		}

		w, _ := memfile.NewPDFWriter(pdf.GetVersion(r), nil)
		rm := pdf.NewResourceManager(w)
		obj, err := rm.Embed(first)
		if err != nil {
			if pdf.IsWrongVersion(err) {
				t.Skip("version not supported")
			}
			t.Fatalf("re-embed: %v", err)
		}
		if err := rm.Close(); err != nil {
			t.Fatalf("rm.Close: %v", err)
		}
		w.GetMeta().Trailer["Quir:E"] = obj
		if err := w.Close(); err != nil {
			t.Fatalf("w.Close: %v", err)
		}

		x2 := pdf.NewExtractor(w)
		second, err := Extract(pdf.CursorAt(x2, nil), w.GetMeta().Trailer["Quir:E"], false)
		if err != nil {
			t.Fatalf("second extract: %v", err)
		}

		if diff := cmp.Diff(first, second, cmpopts.IgnoreFields(Sound{}, "Data")); diff != "" {
			t.Errorf("round trip not stable (-first +second):\n%s", diff)
		}

		if external {
			firstExt, ok := first.Data.(*ExternalFileSource)
			if !ok {
				t.Fatalf("first source changed type to %T", first.Data)
			}
			secondExt, ok := second.Data.(*ExternalFileSource)
			if !ok {
				t.Fatalf("external source became %T", second.Data)
			}
			if diff := cmp.Diff(firstExt.File, secondExt.File); diff != "" {
				t.Errorf("external file diverges (-first +second):\n%s", diff)
			}
		} else {
			rd, err := second.Data.Reader()
			if err != nil {
				t.Fatalf("second Reader: %v", err)
			}
			secondSamples, err := io.ReadAll(rd)
			rd.Close()
			if err != nil {
				t.Fatalf("second ReadAll: %v", err)
			}
			if !bytes.Equal(firstSamples, secondSamples) {
				t.Errorf("sample bytes diverge between reads")
			}
		}
	})
}
