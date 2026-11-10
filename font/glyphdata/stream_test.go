// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package glyphdata

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/limits"
)

// TestExtractFontFile verifies that ExtractFontFile finds the font program
// under the correct descriptor key for each font dictionary type, applies the
// documented key precedence, and returns nil for an external font.
func TestExtractFontFile(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	// newStream embeds a font file stream with the given Subtype (omitted when
	// empty) and returns a reference to it.
	newStream := func(subtype pdf.Name) pdf.Reference {
		ref := w.Alloc()
		dict := pdf.Dict{}
		if subtype != "" {
			dict["Subtype"] = subtype
		}
		stm, err := w.OpenStream(ref, dict)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stm.Write([]byte("font program")); err != nil {
			t.Fatal(err)
		}
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}
		return ref
	}

	fontFile := newStream("")
	fontFile2 := newStream("")
	type1C := newStream("Type1C")
	openType := newStream("OpenType")
	cidType0C := newStream("CIDFontType0C")

	for _, tc := range []struct {
		name     string
		dictType pdf.Name
		fdDict   pdf.Dict
		want     Type // -1 means nil result (external font)
	}{
		{"type1 FontFile", "Type1", pdf.Dict{"FontFile": fontFile}, Type1},
		{"type1 FontFile3", "Type1", pdf.Dict{"FontFile3": type1C}, CFFSimple},
		{"type1 precedence", "Type1",
			pdf.Dict{"FontFile": fontFile, "FontFile3": type1C}, Type1},
		{"truetype FontFile2", "TrueType", pdf.Dict{"FontFile2": fontFile2}, TrueType},
		{"truetype FontFile3", "TrueType", pdf.Dict{"FontFile3": openType}, OpenTypeGlyf},
		{"type0 FontFile3", "Type0", pdf.Dict{"FontFile3": cidType0C}, CFF},
		{"external nil dict", "Type1", nil, -1},
		{"external empty dict", "Type1", pdf.Dict{}, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := ExtractFontFile(pdf.NewCursor(w), tc.fdDict, tc.dictType)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == -1 {
				if s != nil {
					t.Fatalf("expected nil for external font, got type %s", s.Type)
				}
				return
			}
			if s == nil {
				t.Fatal("expected a font file stream, got nil")
			}
			if s.Type != tc.want {
				t.Errorf("wrong font type: got %s, want %s", s.Type, tc.want)
			}
		})
	}
}

// TestExtractStreamOversize verifies that a font program stream larger
// than limits.MaxFontProgramBytes is rejected during WriteTo,
// blocking a decompression-bomb attack on font embedding.
func TestExtractStreamOversize(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	ref := w.Alloc()
	stm, err := w.OpenStream(ref, pdf.Dict{})
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, limits.MaxFontProgramBytes+1)
	if _, err := stm.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := extractStream(pdf.NewCursor(w), ref, "Type1", "FontFile")
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("extractStream returned nil stream")
	}
	if err := s.WriteTo(&bytes.Buffer{}, nil); err == nil {
		t.Fatal("expected error for oversize font program, got nil")
	}
}
