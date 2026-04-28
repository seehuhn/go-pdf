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

package main

import (
	"fmt"
	"io"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/shading"
	"seehuhn.de/go/pdf/optional"
	pdfpage "seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/sound"
)

const (
	leftColStart  = 60.0
	leftColEnd    = 160.0
	rightColStart = 220.0
	rightColEnd   = 320.0
	commentStart  = 380.0

	startY   = 780.0
	iconSize = 24.0
)

func main() {
	if err := createDocument("test.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	paper := document.A4
	opt := &pdf.WriterOptions{HumanReadable: true}
	page, err := document.CreateSinglePage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	page.DrawShading(pageBackground(paper))

	B := font.Must(standard.TimesBold.New())
	H := font.Must(standard.Helvetica.New())

	w := &writer{
		page:  page,
		style: fallback.NewStyle(),
		yPos:  startY,
		font:  H,
		bold:  B,
	}

	// section 1: icon demonstration with side-by-side comparison
	page.TextBegin()
	page.TextSetFont(B, 12)
	page.TextSetMatrix(matrix.Translate(leftColStart-5, w.yPos))
	page.TextShow("Your PDF viewer")
	page.TextSetMatrix(matrix.Translate(rightColStart-5, w.yPos))
	page.TextShow("Quire appearance stream")
	page.TextEnd()
	w.yPos -= 24.0

	// shared sound for the two icon-demo rows: 22.05 kHz / 16-bit / Signed / mono
	iconSnd := buildSound(22050, 1, 16, sound.EncodingSigned)

	for _, row := range []struct {
		icon  annotation.SoundIcon
		label string
	}{
		{annotation.SoundIconSpeaker, "Icon: Speaker"},
		{annotation.SoundIconMic, "Icon: Mic"},
	} {
		a := &annotation.Sound{
			Common: annotation.Common{
				Contents: "Sound annotation",
				Border:   annotation.PDFDefaultBorder,
				Flags:    annotation.FlagPrint,
			},
			Markup: annotation.Markup{User: "Test User"},
			Icon:   row.icon,
			Sound:  iconSnd,
		}
		if err := w.addPair(a, row.label); err != nil {
			return err
		}
	}

	// section 2: encoding coverage (single-icon rows)
	w.yPos -= 18.0
	page.TextBegin()
	page.TextSetFont(B, 12)
	page.TextSetMatrix(matrix.Translate(leftColStart-5, w.yPos))
	page.TextShow("Encoding coverage")
	page.TextEnd()
	w.yPos -= 24.0

	encodings := []struct {
		rate     float64
		channels uint
		bits     uint
		enc      sound.Encoding
		label    string
	}{
		{8000, 1, 8, sound.EncodingRaw, "8 kHz / 8-bit / Raw / mono"},
		{22050, 1, 16, sound.EncodingSigned, "22.05 kHz / 16-bit / Signed / mono"},
		{44100, 2, 16, sound.EncodingSigned, "44.1 kHz / 16-bit / Signed / stereo"},
		{8000, 1, 8, sound.EncodingMuLaw, "8 kHz / 8-bit / muLaw / mono"},
		{8000, 1, 8, sound.EncodingALaw, "8 kHz / 8-bit / ALaw / mono"},
	}
	for _, e := range encodings {
		a := &annotation.Sound{
			Common: annotation.Common{
				Contents: "Sound annotation: " + e.label,
				Border:   annotation.PDFDefaultBorder,
				Flags:    annotation.FlagPrint,
			},
			Markup: annotation.Markup{User: "Test User"},
			Icon:   annotation.SoundIconSpeaker,
			Sound:  buildSound(e.rate, e.channels, e.bits, e.enc),
		}
		if err := w.addSingle(a, e.label); err != nil {
			return err
		}
	}

	return page.Close()
}

type writer struct {
	page  *document.Page
	style *fallback.Style
	yPos  float64
	font  font.Layouter
	bold  font.Layouter
}

func (w *writer) addAnnotation(a annotation.Annotation) {
	w.page.Page.Annots = append(w.page.Page.Annots, pdfpage.AnnotInfo{Annot: a, Ref: w.page.Out.Alloc()})
}

func (w *writer) addPair(left *annotation.Sound, label string) error {
	leftCenter := (leftColStart + leftColEnd) / 2
	rightCenter := (rightColStart + rightColEnd) / 2

	w.page.TextBegin()
	w.page.TextSetFont(w.font, 10)
	w.page.TextSetMatrix(matrix.Translate(commentStart, w.yPos-iconSize/2-3))
	w.page.TextShow(label)
	w.page.TextEnd()

	right := clone(left)

	left.Rect = pdf.Rectangle{
		LLx: leftCenter - iconSize/2,
		LLy: w.yPos - iconSize,
		URx: leftCenter + iconSize/2,
		URy: w.yPos,
	}
	left.Contents += " (viewer)"

	right.Rect = pdf.Rectangle{
		LLx: rightCenter - iconSize/2,
		LLy: w.yPos - iconSize,
		URx: rightCenter + iconSize/2,
		URy: w.yPos,
	}
	right.Contents += " (quire)"

	if err := w.style.AddAppearance(right); err != nil {
		return err
	}
	w.addAnnotation(left)
	w.addAnnotation(right)

	w.yPos -= iconSize + 12.0
	return nil
}

func (w *writer) addSingle(a *annotation.Sound, label string) error {
	rightCenter := (rightColStart + rightColEnd) / 2

	w.page.TextBegin()
	w.page.TextSetFont(w.font, 10)
	w.page.TextSetMatrix(matrix.Translate(commentStart, w.yPos-iconSize/2-3))
	w.page.TextShow(label)
	w.page.TextEnd()

	a.Rect = pdf.Rectangle{
		LLx: rightCenter - iconSize/2,
		LLy: w.yPos - iconSize,
		URx: rightCenter + iconSize/2,
		URy: w.yPos,
	}
	if err := w.style.AddAppearance(a); err != nil {
		return err
	}
	w.addAnnotation(a)

	w.yPos -= iconSize + 12.0
	return nil
}

func clone[T any](v *T) *T {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

// synthMelody returns a short C-major arpeggio at the given sample
// rate, as signed 16-bit mono samples.  Total length ~2.8 s.
func synthMelody(rate float64) []int16 {
	notes := []float64{
		523.25, 659.25, 783.99, 1046.50, 783.99, 659.25, 523.25,
	}
	const noteDur = 0.4
	const fadeDur = 0.008
	samplesPerNote := int(rate * noteDur)
	fadeSamples := int(rate * fadeDur)
	out := make([]int16, 0, samplesPerNote*len(notes))
	for _, freq := range notes {
		for i := range samplesPerNote {
			t := float64(i) / rate
			v := math.Sin(2 * math.Pi * freq * t)
			env := 1.0
			if i < fadeSamples {
				env = float64(i) / float64(fadeSamples)
			} else if i >= samplesPerNote-fadeSamples {
				env = float64(samplesPerNote-i) / float64(fadeSamples)
			}
			v *= env * 0.4
			out = append(out, int16(v*32767))
		}
	}
	return out
}

// buildSound encodes the synthesised melody at the given format and
// wraps it in a sound.Sound with an inline source.
func buildSound(rate float64, channels, bits uint, enc sound.Encoding) *sound.Sound {
	pcm := synthMelody(rate)

	if channels == 2 {
		stereo := make([]int16, len(pcm)*2)
		for i, s := range pcm {
			stereo[2*i] = s
			stereo[2*i+1] = s
		}
		pcm = stereo
	}

	var data []byte
	switch enc {
	case sound.EncodingRaw:
		if bits == 8 {
			data = make([]byte, len(pcm))
			for i, s := range pcm {
				data[i] = byte(int(s>>8) + 128)
			}
		} else { // 16-bit, big-endian, unsigned
			data = make([]byte, len(pcm)*2)
			for i, s := range pcm {
				u := uint16(int32(s) + 32768)
				data[2*i] = byte(u >> 8)
				data[2*i+1] = byte(u)
			}
		}
	case sound.EncodingSigned:
		if bits == 8 {
			data = make([]byte, len(pcm))
			for i, s := range pcm {
				data[i] = byte(int8(s >> 8))
			}
		} else { // 16-bit, big-endian, two's-complement
			data = make([]byte, len(pcm)*2)
			for i, s := range pcm {
				u := uint16(s)
				data[2*i] = byte(u >> 8)
				data[2*i+1] = byte(u)
			}
		}
	case sound.EncodingMuLaw:
		data = make([]byte, len(pcm))
		for i, s := range pcm {
			data[i] = muLawEncode(s)
		}
	case sound.EncodingALaw:
		data = make([]byte, len(pcm))
		for i, s := range pcm {
			data[i] = aLawEncode(s)
		}
	}

	snd := &sound.Sound{
		SampleRate: rate,
		Encoding:   enc,
		Data: &sound.InlineSource{
			WriteData: func(w io.Writer) error {
				_, err := w.Write(data)
				return err
			},
		},
	}
	if channels != 1 {
		snd.Channels = optional.NewUInt(channels)
	}
	if bits != 8 {
		snd.BitsPerSample = optional.NewUInt(bits)
	}
	return snd
}

// muLawEncode converts a 16-bit signed sample to a single G.711 µ-law
// byte (Sun reference algorithm).
func muLawEncode(s int16) byte {
	const bias = 0x84
	const clip = 32635
	var sign byte
	sample := int(s)
	if sample < 0 {
		sample = -sample
		sign = 0x80
	}
	if sample > clip {
		sample = clip
	}
	sample += bias
	seg := 0
	for v := sample >> 7; v > 0; v >>= 1 {
		seg++
	}
	if seg > 0 {
		seg--
	}
	if seg > 7 {
		seg = 7
	}
	mantissa := byte((sample >> (uint(seg) + 3)) & 0x0F)
	return ^(sign | byte(seg<<4) | mantissa)
}

// aLawEncode converts a 16-bit signed sample to a single G.711 A-law
// byte (CCITT reference algorithm).
func aLawEncode(s int16) byte {
	var sign byte = 0x80
	sample := int(s)
	if sample < 0 {
		sign = 0
		sample = -sample
	}
	if sample > 32635 {
		sample = 32635
	}
	var seg, mantissa int
	if sample < 256 {
		seg = 0
		mantissa = (sample >> 4) & 0x0F
	} else {
		seg = 1
		for v := sample >> 8; v > 1; v >>= 1 {
			seg++
		}
		if seg > 7 {
			seg = 7
		}
		mantissa = (sample >> (seg + 3)) & 0x0F
	}
	return (sign | byte(seg<<4) | byte(mantissa)) ^ 0x55
}

func pageBackground(paper *pdf.Rectangle) graphics.Shading {
	alpha := 30.0 / 360 * 2 * math.Pi
	nx := math.Cos(alpha)
	ny := math.Sin(alpha)

	t0 := pdf.Round(paper.LLx*nx+paper.LLy*ny, 1)
	t1 := pdf.Round(paper.URx*nx+paper.URy*ny, 1)

	F := &function.Type4{
		Domain:  []float64{t0, t1},
		Range:   []float64{0, 1, 0, 1, 0, 1},
		Program: "dup 16 div floor 16 mul sub 8 ge {0.99 0.98 0.95}{0.96 0.94 0.89}ifelse",
	}

	return &shading.Type2{
		ColorSpace: color.SpaceDeviceRGB,
		P0:         vec.Vec2{X: pdf.Round(t0*nx, 1), Y: pdf.Round(t0*ny, 1)},
		P1:         vec.Vec2{X: pdf.Round(t1*nx, 1), Y: pdf.Round(t1*ny, 1)},
		F:          F,
		TMin:       t0,
		TMax:       t1,
	}
}
