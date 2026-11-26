// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

// Property tests which hold for every redacted document: redaction is
// idempotent and leaves no redaction annotations.

import (
	"math"
	"testing"

	"seehuhn.de/go/pdf"
)

// propertyDocs returns documents which the property tests redact.
func propertyDocs(t *testing.T) map[string]*pdf.Reader {
	t.Helper()
	docs := map[string]*pdf.Reader{}

	const text = "BT /F1 12 Tf 10 20 Td (Hello Secret World) Tj 0 30 Td [(Kept) -120 (line)] TJ ET"
	plain := textSource(t, text)
	g := pageGlyphs(t, plain)
	docs["text"] = textSource(t, text, regionOver(g[6:12]))

	docs["form"] = singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", nil, secretText)(
		[]pdf.Dict{{"Rect": rect(20, 100, 120, 140)}})
	docs["structure"] = structDoc(t, func(p *structParts) {
		p.form = pdf.Dict{"StructParents": pdf.Integer(5)}
	})
	return docs
}

// sameGlyphs reports whether two glyph lists agree in text and position.
func sameGlyphs(a, b []glyph) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].text != b[i].text ||
			math.Abs(a[i].x-b[i].x) > 1e-3 || math.Abs(a[i].y-b[i].y) > 1e-3 {
			return false
		}
	}
	return true
}

// TestPropertyIdempotent checks that the output has no redaction
// annotations, and that redacting it again changes neither the text nor the
// position of the glyphs.
func TestPropertyIdempotent(t *testing.T) {
	for name, in := range propertyDocs(t) {
		t.Run(name, func(t *testing.T) {
			before := pageGlyphs(t, in)
			once := writeAndRead(t, in, nil, "")
			checkNoRedactAnnots(t, once)
			twice := writeAndRead(t, once, nil, "")

			g1, g2 := pageGlyphs(t, once), pageGlyphs(t, twice)
			if len(g1) >= len(before) {
				t.Errorf("redaction removed no glyphs (%d of %d remain)", len(g1), len(before))
			}
			if !sameGlyphs(g1, g2) {
				t.Errorf("second Write changed the text: got %q, want %q",
					glyphString(g2), glyphString(g1))
			}
		})
	}
}
