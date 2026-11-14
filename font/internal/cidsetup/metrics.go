// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cidsetup

import (
	"math"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
)

// GlyphVMetrics returns the vertical metrics of a glyph, rounded to integer
// PDF glyph space units, as the composite font writers store them in the
// encoder.
func GlyphVMetrics(info *sfnt.Font, gid glyph.ID) cidenc.VMetrics {
	advance, originY := info.GlyphVerticalPDF(gid)
	return cidenc.VMetrics{
		OffsY:  math.Round(originY),
		DeltaY: -math.Round(advance),
	}
}

// Metrics returns the W, DW, DW2 and W2 entries of a CIDFont dictionary for
// the codes mapped by enc.
//
// The widths cover every CID in use, and DW is the width of CID 0, so that a
// CID which was never encoded reads back with the width enc reports for it.
// In horizontal writing mode, the vertical metrics are
// [dict.DefaultVMetricsDefault] and no per-CID entries.  In vertical writing
// mode, DW2 likewise holds the vertical metrics of CID 0.  Each CID in use
// whose metrics differ from DW2 gets a W2 entry, with OffsX set to half its
// width.
func Metrics(enc cidenc.CIDEncoder) (map[cid.CID]float64, float64, dict.DefaultVMetrics, map[cid.CID]dict.VMetrics) {
	ww := make(map[cid.CID]float64)
	vv := make(map[cid.CID]cidenc.VMetrics)
	for _, info := range enc.MappedCodes() {
		ww[info.CID] = info.Width
		vv[info.CID] = info.VMetrics
	}

	dw, notdef := enc.CID0Metrics()
	if enc.WritingMode() != font.Vertical {
		return ww, dw, dict.DefaultVMetricsDefault, nil
	}

	def := dict.DefaultVMetrics{OffsY: notdef.OffsY, DeltaY: notdef.DeltaY}
	var w2 map[cid.CID]dict.VMetrics
	for c, v := range vv {
		if v == notdef {
			continue
		}
		if w2 == nil {
			w2 = make(map[cid.CID]dict.VMetrics)
		}
		w2[c] = dict.VMetrics{OffsX: ww[c] / 2, OffsY: v.OffsY, DeltaY: v.DeltaY}
	}
	return ww, dw, def, w2
}
