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
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// defaultsActivation returns a fresh *Activation populated with the
// PDF specification defaults.  It is structurally equal to
// DefaultActivation but a separate value so tests can mutate it.
func defaultsActivation() *Activation {
	return &Activation{
		Rate:   1.0,
		Volume: 1.0,
		Mode:   ModeOnce,
	}
}

var activationRoundTripCases = []struct {
	name    string
	version pdf.Version
	act     *Activation
}{
	{
		name:    "defaults V1.7",
		version: pdf.V1_7,
		act:     defaultsActivation(),
	},
	{
		name:    "defaults V2.0",
		version: pdf.V2_0,
		act:     defaultsActivation(),
	},
	{
		name:    "small Start integer form",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.Start = Timestamp{Value: 1000}
			return a
		}(),
	},
	{
		name:    "large Start 8-byte string form",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			// > MaxInt32 forces the byte-string encoding
			a.Start = Timestamp{Value: int64(math.MaxInt32) + 17}
			return a
		}(),
	},
	{
		name:    "Start with custom time scale",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.Start = Timestamp{Value: 5000, TimeScale: 1000}
			return a
		}(),
	},
	{
		name:    "Duration set",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.Duration = Timestamp{Value: 100, TimeScale: 30}
			return a
		}(),
	},
	{
		name:    "explicit non-default Rate",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.Rate = -1.0
			return a
		}(),
	},
	{
		name:    "Volume at lower bound",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.Volume = -1.0
			return a
		}(),
	},
	{
		name:    "Volume mid-range",
		version: pdf.V2_0,
		act: func() *Activation {
			a := defaultsActivation()
			a.Volume = 0.5
			return a
		}(),
	},
	{
		name:    "Volume zero (silent)",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.Volume = 0
			return a
		}(),
	},
	{
		name:    "ShowControls",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.ShowControls = true
			return a
		}(),
	},
	{
		name:    "Mode Open",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.Mode = ModeOpen
			return a
		}(),
	},
	{
		name:    "Mode Repeat",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.Mode = ModeRepeat
			return a
		}(),
	},
	{
		name:    "Mode Palindrome",
		version: pdf.V2_0,
		act: func() *Activation {
			a := defaultsActivation()
			a.Mode = ModePalindrome
			return a
		}(),
	},
	{
		name:    "Synchronous",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.Synchronous = true
			return a
		}(),
	},
	{
		name:    "floating window with explicit position",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.FWScale = Scale{Numerator: 3, Denominator: 2}
			a.FWPosition = &Position{Horizontal: 0.0, Vertical: 1.0}
			return a
		}(),
	},
	{
		name:    "floating window default position",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.FWScale = Scale{Numerator: 1, Denominator: 1}
			return a
		}(),
	},
	{
		name:    "all features",
		version: pdf.V1_7,
		act: &Activation{
			Start:        Timestamp{Value: 7, TimeScale: 2},
			Duration:     Timestamp{Value: 11, TimeScale: 2},
			Rate:         1.5,
			Volume:       -0.5,
			ShowControls: true,
			Mode:         ModeRepeat,
			Synchronous:  true,
			FWScale:      Scale{Numerator: 2, Denominator: 1},
			FWPosition:   &Position{Horizontal: 0.25, Vertical: 0.75},
		},
	},
	{
		name:    "single-use direct dict",
		version: pdf.V1_7,
		act: func() *Activation {
			a := defaultsActivation()
			a.SingleUse = true
			return a
		}(),
	},
}

func roundTripActivation(t *testing.T, version pdf.Version, a *Activation) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)
	obj, err := rm.Embed(a)
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
	_, isRef := stored.(pdf.Reference)
	got, err := ExtractActivation(pdf.CursorAt(x, nil), stored, !isRef)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if diff := cmp.Diff(a, got); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestActivationRoundTrip(t *testing.T) {
	for _, tc := range activationRoundTripCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripActivation(t, tc.version, tc.act)
		})
	}
}

func TestActivationEmbedValidation(t *testing.T) {
	cases := []struct {
		name string
		act  *Activation
	}{
		{
			name: "negative Start.Value",
			act: &Activation{
				Rate: 1.0, Volume: 1.0, Mode: ModeOnce,
				Start: Timestamp{Value: -1},
			},
		},
		{
			name: "negative Start.TimeScale",
			act: &Activation{
				Rate: 1.0, Volume: 1.0, Mode: ModeOnce,
				Start: Timestamp{Value: 10, TimeScale: -1},
			},
		},
		{
			name: "Volume above 1",
			act: &Activation{
				Rate: 1.0, Volume: 1.5, Mode: ModeOnce,
			},
		},
		{
			name: "Volume below -1",
			act: &Activation{
				Rate: 1.0, Volume: -1.5, Mode: ModeOnce,
			},
		},
		{
			name: "non-finite Rate",
			act: &Activation{
				Rate: math.Inf(1), Volume: 1.0, Mode: ModeOnce,
			},
		},
		{
			name: "FWScale with zero denominator",
			act: &Activation{
				Rate: 1.0, Volume: 1.0, Mode: ModeOnce,
				FWScale: Scale{Numerator: 1, Denominator: 0},
			},
		},
		{
			name: "FWPosition out of range",
			act: &Activation{
				Rate: 1.0, Volume: 1.0, Mode: ModeOnce,
				FWPosition: &Position{Horizontal: 1.5, Vertical: 0.5},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(tc.act); err == nil {
				t.Errorf("expected error, got nil")
			}
			rm.Close()
			w.Close()
		})
	}
}

func TestActivationEmbedVersionRequirement(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_1, nil)
	rm := pdf.NewResourceManager(w)
	defer w.Close()
	defer rm.Close()

	_, err := rm.Embed(defaultsActivation())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !pdf.IsWrongVersion(err) {
		t.Errorf("expected wrong-version error, got %v", err)
	}
}

// TestTimestampWireForms verifies that small values encode as integers
// and large values encode as 8-byte two's-complement strings.
func TestTimestampWireForms(t *testing.T) {
	t.Run("integer", func(t *testing.T) {
		obj := EncodeTimestamp(Timestamp{Value: 12345})
		if _, ok := obj.(pdf.Integer); !ok {
			t.Errorf("expected pdf.Integer, got %T", obj)
		}
	})
	t.Run("byte string", func(t *testing.T) {
		obj := EncodeTimestamp(Timestamp{Value: int64(math.MaxInt32) + 1})
		s, ok := obj.(pdf.String)
		if !ok {
			t.Fatalf("expected pdf.String, got %T", obj)
		}
		if len(s) != 8 {
			t.Errorf("string length = %d, want 8", len(s))
		}
	})
	t.Run("array with scale", func(t *testing.T) {
		obj := EncodeTimestamp(Timestamp{Value: 100, TimeScale: 30})
		arr, ok := obj.(pdf.Array)
		if !ok {
			t.Fatalf("expected pdf.Array, got %T", obj)
		}
		if len(arr) != 2 {
			t.Errorf("array length = %d, want 2", len(arr))
		}
	})
	t.Run("zero is nil", func(t *testing.T) {
		if obj := EncodeTimestamp(Timestamp{}); obj != nil {
			t.Errorf("expected nil for zero timestamp, got %v", obj)
		}
	})
}

// TestActivationRateZeroShorthand verifies that Rate=0 on the wire
// decodes as the default 1.0 (zero is treated as a shorthand for the
// PDF default).
func TestActivationRateZeroShorthand(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"Rate": pdf.Number(0)}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	w.GetMeta().Trailer["Quir:E"] = ref
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}

	x := pdf.NewExtractor(w)
	got, err := ExtractActivation(pdf.CursorAt(x, nil), w.GetMeta().Trailer["Quir:E"], false)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if got.Rate != 1.0 {
		t.Errorf("Rate = %g, want 1.0 (zero shorthand)", got.Rate)
	}
}

// TestActivationDecodePermissive verifies that an out-of-range Volume
// on the wire is silently coerced to the default 1.0.
func TestActivationDecodePermissive(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref := w.Alloc()
	dict := pdf.Dict{
		"Volume": pdf.Number(2.5), // out of range
	}
	if err := w.Put(ref, dict); err != nil {
		t.Fatalf("Put: %v", err)
	}
	w.GetMeta().Trailer["Quir:E"] = ref
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}

	x := pdf.NewExtractor(w)
	got, err := ExtractActivation(pdf.CursorAt(x, nil), w.GetMeta().Trailer["Quir:E"], false)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if got.Volume != 1.0 {
		t.Errorf("Volume = %g, want 1.0 (default after invalid value)", got.Volume)
	}
}

func FuzzActivationRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range activationRoundTripCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)
		if err := memfile.AddBlankPage(w); err != nil {
			continue
		}
		rm := pdf.NewResourceManager(w)
		obj, err := rm.Embed(tc.act)
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
		_, isRef := objPDF.(pdf.Reference)
		first, err := ExtractActivation(pdf.CursorAt(x, nil), objPDF, !isRef)
		if err != nil {
			t.Skip("malformed activation dictionary")
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
		_, isRef2 := stored.(pdf.Reference)
		second, err := ExtractActivation(pdf.CursorAt(x2, nil), stored, !isRef2)
		if err != nil {
			t.Fatalf("second extract: %v", err)
		}

		if diff := cmp.Diff(first, second); diff != "" {
			t.Errorf("round trip not stable (-first +second):\n%s", diff)
		}
	})
}
