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

package image

import (
	"bytes"
	"maps"
	"math/rand/v2"
	"testing"

	"seehuhn.de/go/jpx"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// jpxTestImage returns an image with random unsigned samples and one
// colour channel per component.
func jpxTestImage(w, h int, depths []int, seed uint64) *jpx.Image {
	rng := rand.New(rand.NewPCG(seed, 2))
	img := &jpx.Image{Width: w, Height: h}
	for i, d := range depths {
		img.Components = append(img.Components, jpx.Component{Depth: d, DX: 1, DY: 1, Width: w, Height: h})
		pix := make([]int32, w*h)
		for k := range pix {
			pix[k] = rng.Int32N(1 << d)
		}
		img.Planes = append(img.Planes, jpx.Plane{Width: w, Height: h, Pix: pix})
		img.Channels = append(img.Channels, jpx.Channel{
			Component: i, PaletteColumn: -1, Kind: jpx.ChannelColor, Assoc: i + 1,
		})
	}
	return img
}

// encodeJPXBytes encodes img as a JP2 file.
func encodeJPXBytes(img *jpx.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpx.Encode(&buf, img, &jpx.EncodeParams{Wrapper: true, Levels: 2}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodeJPX encodes img as a JP2 file.
func encodeJPX(t *testing.T, img *jpx.Image) []byte {
	t.Helper()
	data, err := encodeJPXBytes(img)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// mustEncodeJPX is like encodeJPX, for package-level test tables.
func mustEncodeJPX(img *jpx.Image) []byte {
	data, err := encodeJPXBytes(img)
	if err != nil {
		panic(err)
	}
	return data
}

// writeImageStream writes an image XObject with the given entries and
// body to a file of version v, and returns a cursor for reading it back.
func writeImageStream(t *testing.T, v pdf.Version, entries pdf.Dict, body []byte) (pdf.Cursor, pdf.Reference) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, v, nil)
	ref := w.Alloc()
	dict := pdf.Dict{
		"Type":    pdf.Name("XObject"),
		"Subtype": pdf.Name("Image"),
	}
	maps.Copy(dict, entries)
	stm, err := w.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return pdf.CursorAt(pdf.NewExtractor(w), nil), ref
}

// extractJPXStream embeds data as a JPXDecode image XObject of the given
// size and with the given extra entries, and returns a cursor for reading
// it back.
func extractJPXStream(t *testing.T, data []byte, width, height int, extras pdf.Dict) (pdf.Cursor, pdf.Reference) {
	t.Helper()
	entries := pdf.Dict{
		"Width":  pdf.Integer(width),
		"Height": pdf.Integer(height),
		"Filter": pdf.Name("JPXDecode"),
	}
	maps.Copy(entries, extras)
	return writeImageStream(t, pdf.V2_0, entries, data)
}

// extractJPX embeds data as a JPXDecode image XObject and extracts it again.
func extractJPX(t *testing.T, data []byte, width, height int, extras pdf.Dict) *Dict {
	t.Helper()
	c, ref := extractJPXStream(t, data, width, height, extras)
	img, err := ExtractDict(c, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// extractJPXMask embeds data as a JPXDecode stencil mask and extracts it again.
func extractJPXMask(t *testing.T, data []byte, width, height int, extras pdf.Dict) *Mask {
	t.Helper()
	e := pdf.Dict{"ImageMask": pdf.Boolean(true)}
	maps.Copy(e, extras)
	c, ref := extractJPXStream(t, data, width, height, e)
	m, err := ExtractMask(c, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// extractJPXSoftMask embeds data as a JPXDecode soft mask and extracts it again.
func extractJPXSoftMask(t *testing.T, data []byte, width, height int, extras pdf.Dict) *SoftMask {
	t.Helper()
	c, ref := extractJPXStream(t, data, width, height, extras)
	sm, err := ExtractSoftMask(c, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	return sm
}

// roundTrip embeds obj into a new PDF file of version v and decodes it again.
func roundTrip[T pdf.Embedder](t *testing.T, v pdf.Version, obj T, extract func(pdf.Cursor, pdf.Object, bool) (T, error)) (T, error) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, v, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(obj)
	if err != nil {
		var zero T
		return zero, err
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return extract(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
}
