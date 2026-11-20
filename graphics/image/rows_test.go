// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

import (
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// rowsImage holds the rows of an image, one after another.
type rowsImage struct {
	Pix   []float32
	Alpha []float32
	Key   []uint8
}

// collectRows reads all rows of an image.
func collectRows(t *testing.T, d *Dict) (*RowReader, *rowsImage) {
	t.Helper()
	r, err := d.Rows()
	if err != nil {
		t.Fatal(err)
	}
	res := &rowsImage{}
	next := 0
	for y, row := range r.All() {
		if y != next {
			t.Fatalf("got row %d, want row %d", y, next)
		}
		next++
		res.Pix = append(res.Pix, row.Pix...)
		res.Alpha = append(res.Alpha, row.Alpha...)
		res.Key = append(res.Key, row.Key...)
	}
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
	if next != r.Height {
		t.Fatalf("got %d rows, want %d", next, r.Height)
	}
	return r, res
}

// checkRowsMatchLoad checks that the rows of an image hold the values which
// Load returns.
func checkRowsMatchLoad(t *testing.T, d *Dict) {
	t.Helper()
	data, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := &rowsImage{Pix: data.Pix, Alpha: data.Alpha}
	if data.KeyMask != nil {
		want.Key = data.KeyMask.Pix
	}

	r, got := collectRows(t, d)
	if r.Width != data.Rect.Dx() || r.Height != data.Rect.Dy() {
		t.Errorf("size is %dx%d, want %dx%d",
			r.Width, r.Height, data.Rect.Dx(), data.Rect.Dy())
	}
	if r.CS != data.CS {
		t.Errorf("colour space is %v, want %v", r.CS, data.CS)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("rows differ from Load (-want +got):\n%s", diff)
	}
}

// packedDict returns an image of the given shape whose samples follow a
// fixed pattern.  The image has a Decode array, unless its colour space is
// Indexed, and a colour key which masks some of the pixels.
func packedDict(cs color.Space, bpc, width, height int) *Dict {
	ncomp := cs.Channels()
	data := make([]byte, expectedDataSize(width, ncomp, bpc, height))
	for i := range data {
		data[i] = byte(i*37 + 11)
	}
	var decode []float64
	if _, indexed := cs.(*color.SpaceIndexed); !indexed {
		decode = make([]float64, 2*ncomp)
		for c := range ncomp {
			decode[2*c] = 1
			decode[2*c+1] = float64(-c)
		}
	}
	key := make([]uint16, 2*ncomp)
	for c := range ncomp {
		key[2*c+1] = uint16((1<<bpc - 1) / 2)
	}
	return &Dict{
		Width:            width,
		Height:           height,
		ColorSpace:       cs,
		BitsPerComponent: bpc,
		Decode:           decode,
		MaskColors:       key,
		Data: &FlateSource{
			Width:            width,
			Colors:           ncomp,
			BitsPerComponent: bpc,
			WriteData: func(w io.Writer) error {
				_, err := w.Write(data)
				return err
			},
		},
	}
}

func TestRowsMatchLoad(t *testing.T) {
	spaces := []color.Space{
		color.SpaceDeviceGray,
		color.SpaceDeviceRGB,
		color.SpaceDeviceCMYK,
		testPalette,
	}
	// odd widths, so that rows end inside a byte wherever a pixel does not
	// fill whole bytes
	sizes := [][2]int{{5, 3}, {67, 40}}
	for _, cs := range spaces {
		for _, bpc := range []int{1, 2, 4, 8, 16} {
			if _, indexed := cs.(*color.SpaceIndexed); indexed && bpc > 8 {
				continue
			}
			for _, size := range sizes {
				name := fmt.Sprintf("%s/%dbit/%dx%d", cs.Family(), bpc, size[0], size[1])
				t.Run(name, func(t *testing.T) {
					checkRowsMatchLoad(t, packedDict(cs, bpc, size[0], size[1]))
				})
			}
		}
	}
}

// grayStream returns an 8-bit gray image of the given size which is read
// from a file, where the image stream holds the given bytes.
func grayStream(t *testing.T, width, height int, body []byte) *Dict {
	t.Helper()
	c, ref := writeImageStream(t, pdf.V1_7, pdf.Dict{
		"Width":            pdf.Integer(width),
		"Height":           pdf.Integer(height),
		"ColorSpace":       pdf.Name("DeviceGray"),
		"BitsPerComponent": pdf.Integer(8),
	}, body)
	d, err := ExtractDict(c, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestRowsFromFile checks the rows of an image which is read from a file.
func TestRowsFromFile(t *testing.T) {
	const width, height = 8, 4
	body := make([]byte, width*height)
	for i := range body {
		body[i] = byte(i * 8)
	}
	d := grayStream(t, width, height, body)

	_, got := collectRows(t, d)
	want := make([]float32, len(body))
	for i, v := range body {
		want[i] = float32(float64(v) / 255)
	}
	if diff := cmp.Diff(want, got.Pix); diff != "" {
		t.Errorf("wrong samples (-want +got):\n%s", diff)
	}
}

// TestRowsShortData checks that an image whose data ends inside a row can
// be read.
func TestRowsShortData(t *testing.T) {
	const width, height = 8, 4
	body := make([]byte, width+2)
	for i := range body {
		body[i] = 255
	}
	checkRowsMatchLoad(t, grayStream(t, width, height, body))
}

// TestRowsEarlyExit checks that a loop over the rows can be left early, and
// that the rows can then be read again.
func TestRowsEarlyExit(t *testing.T) {
	cases := map[string]*Dict{
		"packed": packedDict(color.SpaceDeviceRGB, 8, 5, 3),
		"JPX":    extractJPX(t, encodeJPX(t, jpxTestImage(5, 3, []int{8, 8, 8}, 6)), 5, 3, nil),
	}
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := d.Rows()
			if err != nil {
				t.Fatal(err)
			}

			var first []float32
			for _, row := range r.All() {
				first = append(first, row.Pix...)
				break
			}
			if err := r.Err(); err != nil {
				t.Fatal(err)
			}

			var all []float32
			for _, row := range r.All() {
				all = append(all, row.Pix...)
			}
			if err := r.Err(); err != nil {
				t.Fatal(err)
			}
			if len(all) != 5*3*3 {
				t.Fatalf("got %d samples, want %d", len(all), 5*3*3)
			}
			if diff := cmp.Diff(all[:len(first)], first); diff != "" {
				t.Errorf("first row differs between loops (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRowsJPX(t *testing.T) {
	src := opacityImage(false)
	d := extractJPX(t, encodeJPX(t, src), 5, 4, pdf.Dict{
		"SMaskInData": pdf.Integer(1),
	})
	checkRowsMatchLoad(t, d)

	_, rows := collectRows(t, d)
	if len(rows.Alpha) != 5*4 {
		t.Errorf("got %d opacity values, want %d", len(rows.Alpha), 5*4)
	}
}

// TestRowsJPXColorKey checks the colour key mask in the rows of a JPXDecode
// image.
func TestRowsJPXColorKey(t *testing.T) {
	d := extractJPX(t, encodeJPX(t, jpxTestImage(6, 5, []int{8, 8, 8}, 6)), 6, 5, nil)
	d.MaskColors = []uint16{0, 127, 0, 255, 50, 200}
	checkRowsMatchLoad(t, d)

	_, rows := collectRows(t, d)
	if len(rows.Key) != 6*5 {
		t.Errorf("got %d key mask values, want %d", len(rows.Key), 6*5)
	}
}
