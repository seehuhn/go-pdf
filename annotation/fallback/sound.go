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

package fallback

import (
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

// addSoundAppearance generates a fallback appearance for a sound
// annotation, mirroring the layout used by file-attachment annotations:
// a fixed 24×24 icon anchored at the upper-left of the supplied Rect,
// with NoZoom and NoRotate forced so the icon stays icon-sized at any
// zoom level.
func (s *Style) addSoundAppearance(a *annotation.Sound) (*form.Form, error) {
	a.Rect = pdf.Rectangle{
		LLx: a.Rect.LLx,
		LLy: a.Rect.URy - 24,
		URx: a.Rect.LLx + 24,
		URy: a.Rect.URy,
	}
	a.Flags |= annotation.FlagNoZoom | annotation.FlagNoRotate

	col := a.Color
	if col == nil {
		col = quireInk2
	}

	b := builder.New(content.Form, nil, s.version)
	b.SetExtGState(s.reset)

	// slate card backdrop, identical to file attachments
	b.SetLineWidth(0.5)
	b.SetStrokeColor(quireSlate3)
	b.SetFillColor(quireSlate1)
	b.Rectangle(0.5, 0.5, 23, 23)
	b.FillAndStroke()

	switch a.Icon {
	case annotation.SoundIconMic:
		drawMicIcon(b, col)
	default:
		// Speaker is the spec default and also our fallback for unknown names
		drawSpeakerIcon(b, col)
	}

	return harvest(b, pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24})
}

// drawSpeakerIcon draws a loudspeaker silhouette as a single filled
// polygon (back box on the left + trapezoidal cone flaring to the right),
// followed by two stroked sound-wave arcs to the right of the cone front.
func drawSpeakerIcon(b *builder.Builder, col color.Color) {
	b.SetFillColor(col)
	b.MoveTo(4, 9)
	b.LineTo(6, 9)
	b.LineTo(10, 5)
	b.LineTo(10, 19)
	b.LineTo(6, 15)
	b.LineTo(4, 15)
	b.ClosePath()
	b.Fill()

	b.SetStrokeColor(col)
	b.SetLineWidth(1)
	b.SetLineCap(graphics.LineCapRound)

	// concentric arcs centred inside the cone, opening to the right
	const cx, cy = 8.0, 12.0
	const sweep = math.Pi / 4
	for _, r := range []float64{5.5, 8, 10.5} {
		b.MoveTo(cx+r*math.Cos(-sweep), cy+r*math.Sin(-sweep))
		b.LineToArc(cx, cy, r, -sweep, sweep)
	}
	b.Stroke()
}

// drawMicIcon draws a stadium-shaped microphone capsule with a stand and
// base bar, all stroked in col.
func drawMicIcon(b *builder.Builder, col color.Color) {
	b.SetStrokeColor(col)
	b.SetLineWidth(1)
	b.SetLineCap(graphics.LineCapRound)
	b.SetLineJoin(graphics.LineJoinRound)

	// capsule (stadium): vertical pill centred on x=12, total y from 8 to 20
	b.MoveTo(15, 17)
	b.LineToArc(12, 17, 3, 0, math.Pi)
	b.LineTo(9, 11)
	b.LineToArc(12, 11, 3, math.Pi, 2*math.Pi)
	b.ClosePath()
	b.Stroke()

	// stand and base bar
	b.MoveTo(12, 8)
	b.LineTo(12, 4)
	b.MoveTo(8, 4)
	b.LineTo(16, 4)
	b.Stroke()
}
