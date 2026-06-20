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

package movie

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func basicSpec() *file.Specification {
	return &file.Specification{
		FileName:       "movie.mov",
		AFRelationship: file.RelationshipUnspecified,
	}
}

var movieRoundTripCases = []struct {
	name    string
	version pdf.Version
	movie   *Movie
}{
	{
		name:    "minimal V1.7",
		version: pdf.V1_7,
		movie: &Movie{
			File: basicSpec(),
		},
	},
	{
		name:    "minimal V2.0",
		version: pdf.V2_0,
		movie: &Movie{
			File: basicSpec(),
		},
	},
	{
		name:    "with aspect and rotation",
		version: pdf.V1_7,
		movie: &Movie{
			File:   basicSpec(),
			Aspect: Aspect{Width: 640, Height: 480},
			Rotate: 90,
		},
	},
	{
		name:    "rotation 180",
		version: pdf.V1_7,
		movie: &Movie{
			File:   basicSpec(),
			Rotate: 180,
		},
	},
	{
		name:    "rotation 270",
		version: pdf.V2_0,
		movie: &Movie{
			File:   basicSpec(),
			Rotate: 270,
		},
	},
	{
		name:    "poster from movie file",
		version: pdf.V1_7,
		movie: &Movie{
			File:   basicSpec(),
			Aspect: Aspect{Width: 320, Height: 240},
			Poster: PosterFromMovieFile,
		},
	},
	{
		name:    "single-use direct dict",
		version: pdf.V1_7,
		movie: &Movie{
			File:      basicSpec(),
			SingleUse: true,
		},
	},
}

func roundTripMovie(t *testing.T, version pdf.Version, m *Movie) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)
	obj, err := rm.Embed(m)
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
	stored := w.GetMeta().Trailer["Quir:E"]
	_, isDirect := stored.(pdf.Reference)
	isDirect = !isDirect
	got, err := Extract(pdf.CursorAt(x, nil), stored, isDirect)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if diff := cmp.Diff(m, got); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestMovieRoundTrip(t *testing.T) {
	for _, tc := range movieRoundTripCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripMovie(t, tc.version, tc.movie)
		})
	}
}

func TestMovieEmbedValidation(t *testing.T) {
	cases := []struct {
		name  string
		movie *Movie
	}{
		{
			name:  "missing File",
			movie: &Movie{},
		},
		{
			name: "non-multiple-of-90 rotation",
			movie: &Movie{
				File:   basicSpec(),
				Rotate: 45,
			},
		},
		{
			name: "negative aspect width",
			movie: &Movie{
				File:   basicSpec(),
				Aspect: Aspect{Width: -1, Height: 1},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(tc.movie); err == nil {
				t.Errorf("expected error, got nil")
			}
			rm.Close()
			w.Close()
		})
	}
}

func TestMovieEmbedVersionRequirement(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_1, nil)
	rm := pdf.NewResourceManager(w)
	defer w.Close()
	defer rm.Close()

	m := &Movie{File: basicSpec()}
	_, err := rm.Embed(m)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !pdf.IsWrongVersion(err) {
		t.Errorf("expected wrong-version error, got %v", err)
	}
}

// TestPosterFromMovieFileEmbedRejects verifies that the sentinel cannot
// be silently passed to rm.Embed: its Embed method must return an error.
func TestPosterFromMovieFileEmbedRejects(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	defer w.Close()
	defer rm.Close()
	if _, err := rm.Embed(PosterFromMovieFile); err == nil {
		t.Error("expected error embedding the marker, got nil")
	}
}

func FuzzMovieRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range movieRoundTripCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)
		if err := memfile.AddBlankPage(w); err != nil {
			continue
		}
		rm := pdf.NewResourceManager(w)
		obj, err := rm.Embed(tc.movie)
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
		_, isDirect := objPDF.(pdf.Reference)
		isDirect = !isDirect
		first, err := Extract(pdf.CursorAt(x, nil), objPDF, isDirect)
		if err != nil {
			t.Skip("malformed movie dictionary")
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
		stored := w.GetMeta().Trailer["Quir:E"]
		_, isDirect2 := stored.(pdf.Reference)
		isDirect2 = !isDirect2
		second, err := Extract(pdf.CursorAt(x2, nil), stored, isDirect2)
		if err != nil {
			t.Fatalf("second extract: %v", err)
		}

		if diff := cmp.Diff(first, second); diff != "" {
			t.Errorf("round trip not stable (-first +second):\n%s", diff)
		}
	})
}
