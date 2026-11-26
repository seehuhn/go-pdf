// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	goimage "image"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/jpx"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/measure"
	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/webcapture"
)

const (
	imgW, imgH = 10, 10

	// placeAt100 paints image /Im1 on the square from (100,100) to (200,200)
	placeAt100 = "q 100 0 0 100 100 100 cm /Im1 Do Q\n"
)

var imgBox = box{100, 100, 200, 200}

// region is the rectangle which the tests redact, in page space.  It does
// not lie on cell boundaries.
var region = [4]int{127, 126, 143, 144}

func regionBox() box {
	return box{float64(region[0]), float64(region[1]), float64(region[2]), float64(region[3])}
}

// singleImageDoc is a document with one page, which paints the image img as
// /Im1.
func singleImageDoc(t testing.TB, img pdf.Embedder, contentData string, redacts [][4]int) *pdf.Reader {
	t.Helper()
	return imageDoc(t, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
		ref, err := rm.Embed(img)
		if err != nil {
			t.Fatal(err)
		}
		return []imgPage{{
			content:  contentData,
			xobjects: pdf.Dict{"Im1": ref},
			redacts:  redacts,
		}}
	})
}

// onlyImage returns the single image of the first page of a redacted
// document, and checks that the content paints it once.
func onlyImage(t testing.TB, out *pdf.Reader) (pdf.Dict, pdf.Name) {
	t.Helper()
	page := pageAt(t, out, 0)
	names := doNames(t, out, page)
	var imgs []pdf.Name
	for _, n := range names {
		if slices.Contains(imageNames(t, out, page), n) {
			imgs = append(imgs, n)
		}
	}
	if len(imgs) != 1 {
		t.Fatalf("page paints %d images, want 1", len(imgs))
	}
	return page, imgs[0]
}

func TestImagePartial(t *testing.T) {
	data := secretRGB(imgW, imgH, func(x, y int) bool {
		return x >= 2 && x <= 4 && y >= 5 && y <= 7
	})
	src := singleImageDoc(t, rgbDict(imgW, imgH, data), placeAt100, [][4]int{region})
	out := writeAndRead(t, src, nil, "")

	page, name := onlyImage(t, out)
	if name == "Im1" {
		t.Errorf("the redacted image keeps the name of the original")
	}
	d, got := samplesOf(t, out, xobject(t, out, page, name))
	if d.Width != imgW || d.Height != imgH || d.BitsPerComponent != 8 {
		t.Errorf("image has size %dx%d at %d bits", d.Width, d.Height, d.BitsPerComponent)
	}

	want := slices.Clone(data)
	hit := 0
	for r := range imgH {
		for c := range imgW {
			if !cellBox(imgBox, imgW, imgH, r, c).overlaps(regionBox()) {
				continue
			}
			hit++
			for k := range 3 {
				want[(r*imgW+c)*3+k] = 255
			}
		}
	}
	if hit == 0 {
		t.Fatal("test region meets no cell")
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("wrong samples (-want +got):\n%s", diff)
	}
	checkNoSecret(t, out)
}

// TestImageAligned checks that a region whose edges lie on cell boundaries
// overwrites the cells it covers and keeps the cells which only touch it,
// both for image XObjects and for inline images.
func TestImageAligned(t *testing.T) {
	const n = 4
	data := make([]byte, n*n)
	for i := range data {
		data[i] = byte(16 * i)
	}
	// the region covers the top left 2x2 cells
	redacts := [][4]int{{100, 150, 150, 200}}
	want := slices.Clone(data)
	for r := range 2 {
		for c := range 2 {
			want[r*n+c] = 255
		}
	}

	t.Run("xobject", func(t *testing.T) {
		src := singleImageDoc(t, grayDict(n, n, 8, data), placeAt100, redacts)
		out := writeAndRead(t, src, nil, "")
		page, name := onlyImage(t, out)
		_, got := samplesOf(t, out, xobject(t, out, page, name))
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("wrong samples (-want +got):\n%s", diff)
		}
	})

	t.Run("inline", func(t *testing.T) {
		contentData := "q 100 0 0 100 100 100 cm\nBI /W 4 /H 4 /CS /G /BPC 8 /F /AHx\nID\n" +
			fmt.Sprintf("%x>", data) + "\nEI Q\n"
		src := imageDocVersion(t, pdf.V2_0, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
			return []imgPage{{content: contentData, xobjects: pdf.Dict{}, redacts: redacts}}
		})
		out := writeAndRead(t, src, nil, "")
		var found bool
		for _, op := range pageOps(t, out, pageAt(t, out, 0)) {
			if op.name != content.OpInlineImage {
				continue
			}
			found = true
			got, err := content.DecodeInlineImage(content.Operator{Name: op.name, Args: op.args}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("wrong samples (-want +got):\n%s", diff)
			}
		}
		if !found {
			t.Error("inline image was removed")
		}
	})
}

func TestImageInterpolate(t *testing.T) {
	// interpolation does not widen the set of cells which are overwritten
	for _, interp := range []bool{false, true} {
		data := secretRGB(imgW, imgH, nil)
		d := rgbDict(imgW, imgH, data)
		d.Interpolate = interp
		src := singleImageDoc(t, d, placeAt100, [][4]int{region})
		out := writeAndRead(t, src, nil, "")
		page, name := onlyImage(t, out)
		_, got := samplesOf(t, out, xobject(t, out, page, name))

		hit := hitCells(imgW, imgH)
		for r := range imgH {
			for c := range imgW {
				i := (r*imgW + c) * 3
				changed := !bytes.Equal(got[i:i+3], data[i:i+3])
				if changed != hit[[2]int{r, c}] {
					t.Errorf("interpolate=%t: cell (%d,%d) changed=%t, want %t",
						interp, r, c, changed, hit[[2]int{r, c}])
				}
			}
		}
	}
}

func TestImageInsideRegion(t *testing.T) {
	data := secretRGB(imgW, imgH, nil)
	src := singleImageDoc(t, rgbDict(imgW, imgH, data), placeAt100,
		[][4]int{{90, 90, 210, 210}})
	out := writeAndRead(t, src, nil, "")
	page := pageAt(t, out, 0)
	for _, n := range doNames(t, out, page) {
		if slices.Contains(imageNames(t, out, page), n) {
			t.Errorf("image %s is still painted", n)
		}
	}
	if len(imageNames(t, out, page)) != 0 {
		t.Errorf("page resources still list an image")
	}
}

func TestImageOutsideRegion(t *testing.T) {
	data := secretRGB(imgW, imgH, nil)
	src := singleImageDoc(t, rgbDict(imgW, imgH, data), placeAt100,
		[][4]int{{210, 210, 250, 250}})
	out := writeAndRead(t, src, nil, "")
	page, name := onlyImage(t, out)
	if name != "Im1" {
		t.Errorf("unaffected image is renamed to %s", name)
	}
	_, got := samplesOf(t, out, xobject(t, out, page, name))
	if !bytes.Equal(got, data) {
		t.Errorf("unaffected image changed")
	}
}

func TestImageTouchingRegion(t *testing.T) {
	// the region only touches the image along an edge
	data := secretRGB(imgW, imgH, nil)
	src := singleImageDoc(t, rgbDict(imgW, imgH, data), placeAt100,
		[][4]int{{200, 100, 250, 200}})
	out := writeAndRead(t, src, nil, "")
	page, name := onlyImage(t, out)
	_, got := samplesOf(t, out, xobject(t, out, page, name))
	if !bytes.Equal(got, data) {
		t.Errorf("image changed by a region which only touches it")
	}
}

func TestImageRotated(t *testing.T) {
	data := secretRGB(imgW, imgH, nil)
	// rotation by 30 degrees about (150,150), scaled to 100x100
	const c, s = 86.60254037844386, 50.0
	m := "q 86.60254 50 -50 86.60254 " // a b c d
	// the image centre maps to (150,150)
	e := 150 - (c*0.5 - s*0.5)
	f := 150 - (s*0.5 + c*0.5)
	cm := m + formatNum(e) + " " + formatNum(f) + " cm /Im1 Do Q\n"
	src := singleImageDoc(t, rgbDict(imgW, imgH, data), cm, [][4]int{region})
	out := writeAndRead(t, src, nil, "")
	page, name := onlyImage(t, out)
	_, got := samplesOf(t, out, xobject(t, out, page, name))

	// image space (x,y) in the unit square maps to the page by the matrix
	apply := func(x, y float64) (float64, float64) {
		return 86.60254*x - 50*y + e, 50*x + 86.60254*y + f
	}
	rg := regionBox()
	for r := range imgH {
		for col := range imgW {
			x0, x1 := float64(col)/imgW, float64(col+1)/imgW
			y0, y1 := 1-float64(r+1)/imgH, 1-float64(r)/imgH
			bb := box{1e9, 1e9, -1e9, -1e9}
			allIn := true
			for _, p := range [][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}} {
				px, py := apply(p[0], p[1])
				bb.x0, bb.y0 = min(bb.x0, px), min(bb.y0, py)
				bb.x1, bb.y1 = max(bb.x1, px), max(bb.y1, py)
				if px < rg.x0 || px > rg.x1 || py < rg.y0 || py > rg.y1 {
					allIn = false
				}
			}
			cell := got[(r*imgW+col)*3:][:3]
			orig := data[(r*imgW+col)*3:][:3]
			disjoint := bb.x1 < rg.x0-1 || bb.x0 > rg.x1+1 || bb.y1 < rg.y0-1 || bb.y0 > rg.y1+1
			switch {
			case allIn && !bytes.Equal(cell, []byte{255, 255, 255}):
				t.Errorf("cell (%d,%d) inside the region is %v", r, col, cell)
			case disjoint && !bytes.Equal(cell, orig):
				t.Errorf("cell (%d,%d) outside the region changed to %v", r, col, cell)
			}
		}
	}
	if bytes.Equal(got, data) {
		t.Errorf("image was not changed")
	}
}

func formatNum(x float64) string {
	return strconv.FormatFloat(x, 'f', 4, 64)
}

// hitCells returns the cells of a w x h image placed on imgBox which meet
// the region.
func hitCells(w, h int) map[[2]int]bool {
	res := map[[2]int]bool{}
	for r := range h {
		for c := range w {
			if cellBox(imgBox, w, h, r, c).overlaps(regionBox()) {
				res[[2]int{r, c}] = true
			}
		}
	}
	return res
}

// bitAt returns bit x of a packed row.
func bitAt(row []byte, x int) byte { return row[x/8] >> (7 - x%8) & 1 }

func TestImageMaskDecode(t *testing.T) {
	// 1 means painted, because of the Decode array
	const w, h = 16, 16
	rowBytes := w / 8
	data := bytes.Repeat([]byte{0xAA, 0x55}, h)
	mask := &image.Mask{
		Width: w, Height: h, Inverted: true,
		Source: image.NewCCITTFaxSource(w, writeBytes(data)),
	}
	src := singleImageDoc(t, mask, placeAt100, [][4]int{region})
	out := writeAndRead(t, src, nil, "")
	page, name := onlyImage(t, out)
	m, got := maskSamplesOf(t, out, xobject(t, out, page, name))
	if !m.Inverted {
		t.Errorf("/Decode array of the mask was lost")
	}

	hit := hitCells(w, h)
	if len(hit) == 0 {
		t.Fatal("region meets no cell")
	}
	for r := range h {
		for c := range w {
			orig := bitAt(data[r*rowBytes:], c)
			b := bitAt(got[r*rowBytes:], c)
			switch {
			case hit[[2]int{r, c}] && b != 0:
				t.Errorf("sample (%d,%d) of the mask is still painted", r, c)
			case !hit[[2]int{r, c}] && b != orig:
				t.Errorf("sample (%d,%d) of the mask changed", r, c)
			}
		}
	}
}

func TestImageMaskDefaultDecode(t *testing.T) {
	const w, h = 8, 8
	data := bytes.Repeat([]byte{0x00}, h) // everything painted
	mask := &image.Mask{
		Width: w, Height: h,
		Source: image.NewFlateSource(w, color.SpaceDeviceGray, 1, writeBytes(data)),
	}
	src := singleImageDoc(t, mask, placeAt100, [][4]int{region})
	out := writeAndRead(t, src, nil, "")
	page, name := onlyImage(t, out)
	_, got := maskSamplesOf(t, out, xobject(t, out, page, name))
	hit := hitCells(w, h)
	for r := range h {
		for c := range w {
			if want := hit[[2]int{r, c}]; (bitAt(got[r:], c) == 1) != want {
				t.Errorf("sample (%d,%d) is %d, unpainted is %v", r, c, bitAt(got[r:], c), want)
			}
		}
	}
}

func TestImageSoftMaskAndMask(t *testing.T) {
	// the masks have a different resolution than the image
	const mw, mh = 20, 20
	soft := make([]byte, mw*mh)
	for i := range soft {
		soft[i] = byte(100 + i%100)
	}
	sm := &image.SoftMask{
		Width: mw, Height: mh, BitsPerComponent: 8,
		Source: image.NewFlateSource(mw, color.SpaceDeviceGray, 8, writeBytes(soft)),
	}
	img1 := rgbDict(imgW, imgH, secretRGB(imgW, imgH, nil))
	img1.SMask = sm

	stencil := bytes.Repeat([]byte{0x0F, 0xF0, 0x00}, mh)[:mh*3]
	bm := &image.Mask{
		Width: mw, Height: mh,
		Source: image.NewFlateSource(mw, color.SpaceDeviceGray, 1, writeBytes(stencil)),
	}
	img2 := rgbDict(imgW, imgH, secretRGB(imgW, imgH, nil))
	img2.MaskImage = bm

	src := imageDoc(t, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
		r1, err := rm.Embed(img1)
		if err != nil {
			t.Fatal(err)
		}
		r2, err := rm.Embed(img2)
		if err != nil {
			t.Fatal(err)
		}
		return []imgPage{{
			content:  "q 100 0 0 100 100 100 cm /Im1 Do Q\nq 100 0 0 100 100 100 cm /Im2 Do Q\n",
			xobjects: pdf.Dict{"Im1": r1, "Im2": r2},
			redacts:  [][4]int{region},
		}}
	})
	out := writeAndRead(t, src, nil, "")
	page := pageAt(t, out, 0)
	names := doNames(t, out, page)
	if len(names) != 3 { // two images and the redaction mark
		t.Fatalf("page paints %v", names)
	}
	cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
	hit := hitCells(mw, mh)

	d1, err := image.ExtractDict(cur, xobject(t, out, page, names[0]), false)
	if err != nil {
		t.Fatal(err)
	}
	if d1.SMask == nil {
		t.Fatal("soft mask was lost")
	}
	sr, err := d1.SMask.SampleRows()
	if err != nil {
		t.Fatal(err)
	}
	var got []byte
	for _, row := range sr.All() {
		got = append(got, row...)
	}
	for r := range mh {
		for c := range mw {
			i := r*mw + c
			switch {
			case hit[[2]int{r, c}] && got[i] != 0:
				t.Errorf("soft mask sample (%d,%d) is %d, want 0", r, c, got[i])
			case !hit[[2]int{r, c}] && got[i] != soft[i]:
				t.Errorf("soft mask sample (%d,%d) changed", r, c)
			}
		}
	}

	d2, err := image.ExtractDict(cur, xobject(t, out, page, names[1]), false)
	if err != nil {
		t.Fatal(err)
	}
	if d2.MaskImage == nil {
		t.Fatal("mask image was lost")
	}
	got = maskRows(t, d2.MaskImage)
	rowBytes := (mw + 7) / 8
	for r := range mh {
		for c := range mw {
			orig := bitAt(stencil[r*rowBytes:], c)
			b := bitAt(got[r*rowBytes:], c)
			switch {
			case hit[[2]int{r, c}] && b != 1:
				t.Errorf("mask sample (%d,%d) is painted", r, c)
			case !hit[[2]int{r, c}] && b != orig:
				t.Errorf("mask sample (%d,%d) changed", r, c)
			}
		}
	}
}

func TestImageGrayDecode(t *testing.T) {
	const w, h = 10, 10
	data := make([]byte, w*h)
	for i := range data {
		data[i] = byte(i)
	}
	img := grayDict(w, h, 8, data)
	img.Decode = []float64{1, 0}
	src := singleImageDoc(t, img, placeAt100, [][4]int{region})
	out := writeAndRead(t, src, nil, "")
	page, name := onlyImage(t, out)
	d, got := samplesOf(t, out, xobject(t, out, page, name))
	if !slices.Equal(d.Decode, []float64{1, 0}) {
		t.Errorf("/Decode is %v", d.Decode)
	}
	hit := hitCells(w, h)
	for r := range h {
		for c := range w {
			i := r*w + c
			if hit[[2]int{r, c}] != (got[i] == 255) || (!hit[[2]int{r, c}] && got[i] != data[i]) {
				t.Errorf("sample (%d,%d) is %d", r, c, got[i])
			}
		}
	}
}

func TestImagePackedSamples(t *testing.T) {
	// 4-bit gray: two samples per byte
	const w, h = 10, 10
	rowBytes := (w*4 + 7) / 8
	data := make([]byte, rowBytes*h)
	for i := range data {
		data[i] = byte(i*37) | 0x11
	}
	src := singleImageDoc(t, grayDict(w, h, 4, data), placeAt100, [][4]int{region})
	out := writeAndRead(t, src, nil, "")
	page, name := onlyImage(t, out)
	_, got := samplesOf(t, out, xobject(t, out, page, name))
	nib := func(b []byte, x int) byte {
		if x%2 == 0 {
			return b[x/2] >> 4
		}
		return b[x/2] & 15
	}
	hit := hitCells(w, h)
	for r := range h {
		for c := range w {
			want := nib(data[r*rowBytes:], c)
			if hit[[2]int{r, c}] {
				want = 15
			}
			if g := nib(got[r*rowBytes:], c); g != want {
				t.Errorf("sample (%d,%d) is %d, want %d", r, c, g, want)
			}
		}
	}
}

func TestImageDCT(t *testing.T) {
	const w, h = 16, 16
	pix := make([]byte, w*h)
	for i := range pix {
		pix[i] = byte(30 + (i%w)*6 + (i/w)*3)
	}
	img := &image.Dict{
		Width: w, Height: h,
		ColorSpace: color.SpaceDeviceGray, BitsPerComponent: 8,
		Data: image.NewDCTSource(w, h, color.SpaceDeviceGray, writeBytes(pix)),
	}
	src := singleImageDoc(t, img, placeAt100, [][4]int{region})

	// the samples a reader of the input obtains
	var orig []byte
	{
		page := pageAt(t, src, 0)
		_, orig = samplesOf(t, src, xobject(t, src, page, "Im1"))
	}

	out := writeAndRead(t, src, nil, "")
	page, name := onlyImage(t, out)
	xo := xobject(t, out, page, name)
	d, got := samplesOf(t, out, xo)
	if d.BitsPerComponent != 8 {
		t.Errorf("got %d bits per component", d.BitsPerComponent)
	}
	cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
	stm, err := cur.Stream(xo)
	if err != nil {
		t.Fatal(err)
	}
	filters, err := cur.Filters(stm.Dict)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range filters {
		if _, isDCT := f.(pdf.FilterDCT); isDCT {
			t.Errorf("redacted image is still JPEG encoded")
		}
	}

	hit := hitCells(w, h)
	for r := range h {
		for c := range w {
			i := r*w + c
			switch {
			case hit[[2]int{r, c}] && got[i] != 255:
				t.Errorf("sample (%d,%d) is %d, want 255", r, c, got[i])
			case !hit[[2]int{r, c}] && got[i] != orig[i]:
				t.Errorf("sample (%d,%d) changed from %d to %d", r, c, orig[i], got[i])
			}
		}
	}
}

func TestImageInline(t *testing.T) {
	data := secretRGB(imgW, imgH, func(x, y int) bool {
		return x >= 2 && x <= 4 && y >= 5 && y <= 7
	})
	var hex bytes.Buffer
	for _, b := range data {
		hex.WriteByte("0123456789abcdef"[b>>4])
		hex.WriteByte("0123456789abcdef"[b&15])
	}
	hex.WriteByte('>')
	contentData := "q 100 0 0 100 100 100 cm\nBI /W 10 /H 10 /CS /RGB /BPC 8 /F /AHx\nID\n" +
		hex.String() + "\nEI Q\n"
	src := imageDoc(t, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
		return []imgPage{{content: contentData, xobjects: pdf.Dict{}, redacts: [][4]int{region}}}
	})
	out := writeAndRead(t, src, nil, "")
	page := pageAt(t, out, 0)

	var found bool
	for _, op := range pageOps(t, out, page) {
		if op.name != content.OpInlineImage {
			continue
		}
		found = true
		dict := op.args[0].(pdf.Dict)
		if dict["F"] != pdf.Name("Fl") {
			t.Errorf("inline image filter is %v", dict["F"])
		}
		raw, err := content.DecodeInlineImage(content.Operator{Name: op.name, Args: op.args}, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := slices.Clone(data)
		for cell := range hitCells(imgW, imgH) {
			for k := range 3 {
				want[(cell[0]*imgW+cell[1])*3+k] = 255
			}
		}
		if diff := cmp.Diff(want, raw); diff != "" {
			t.Errorf("wrong samples (-want +got):\n%s", diff)
		}
	}
	if !found {
		t.Error("inline image was removed")
	}
	checkNoSecret(t, out)
}

func TestImageInlineInsideRegion(t *testing.T) {
	contentData := "q 100 0 0 100 100 100 cm\nBI /W 1 /H 1 /CS /G /BPC 8 /F /AHx\nID\n7f>\nEI Q\n"
	src := imageDoc(t, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
		return []imgPage{{content: contentData, xobjects: pdf.Dict{}, redacts: [][4]int{{90, 90, 210, 210}}}}
	})
	out := writeAndRead(t, src, nil, "")
	for _, op := range pageOps(t, out, pageAt(t, out, 0)) {
		if op.name == content.OpInlineImage {
			t.Error("inline image inside the region was kept")
		}
	}
}

func TestImageSingularMatrix(t *testing.T) {
	data := secretRGB(imgW, imgH, nil)
	src := singleImageDoc(t, rgbDict(imgW, imgH, data),
		"q 100 0 0 0 100 150 cm /Im1 Do Q\n", [][4]int{{120, 100, 130, 200}})
	out := writeAndRead(t, src, nil, "")
	page := pageAt(t, out, 0)
	if names := imageNames(t, out, page); len(names) != 0 {
		t.Errorf("flat image under a region was kept: %v", names)
	}
}

func TestImageSharedWithOtherPage(t *testing.T) {
	data := secretRGB(imgW, imgH, nil)
	src := imageDoc(t, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
		ref, err := rm.Embed(rgbDict(imgW, imgH, data))
		if err != nil {
			t.Fatal(err)
		}
		xo := pdf.Dict{"Im1": ref}
		return []imgPage{
			{content: placeAt100, xobjects: xo, redacts: [][4]int{region}},
			{content: placeAt100, xobjects: xo},
		}
	})
	out := writeAndRead(t, src, nil, "")

	page0, page1 := pageAt(t, out, 0), pageAt(t, out, 1)
	_, name0 := onlyImage(t, out)
	_, got0 := samplesOf(t, out, xobject(t, out, page0, name0))
	if bytes.Equal(got0, data) {
		t.Errorf("redacted page shows the original image")
	}
	if n := imageNames(t, out, page1); len(n) != 1 || n[0] != "Im1" {
		t.Fatalf("second page has images %v", n)
	}
	_, got1 := samplesOf(t, out, xobject(t, out, page1, "Im1"))
	if !bytes.Equal(got1, data) {
		t.Errorf("unredacted page lost the original image")
	}
}

func TestImageRedirectedInAppearance(t *testing.T) {
	// a kept annotation draws the image, which the page paints under a
	// region, in its appearance stream; the appearance keeps the original
	data := secretRGB(imgW, imgH, func(x, y int) bool {
		return x >= 2 && x <= 4 && y >= 5 && y <= 7
	})
	src := imageDoc(t, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
		ref, err := rm.Embed(rgbDict(imgW, imgH, data))
		if err != nil {
			t.Fatal(err)
		}
		ap := w.Alloc()
		stm, err := w.OpenStream(ap, pdf.Dict{
			"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
			"BBox":      rect(0, 0, 100, 100),
			"Resources": pdf.Dict{"XObject": pdf.Dict{"Im1": ref}},
		}, pdf.FilterCompress{})
		if err != nil {
			t.Fatal(err)
		}
		stm.Write([]byte("q 100 0 0 100 0 0 cm /Im1 Do Q\n"))
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}
		annot := w.Alloc()
		if err := w.Put(annot, pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Stamp"),
			"Rect": rect(220, 220, 320, 320),
			"AP":   pdf.Dict{"N": ap},
		}); err != nil {
			t.Fatal(err)
		}
		return []imgPage{{
			content: placeAt100, xobjects: pdf.Dict{"Im1": ref},
			redacts: [][4]int{region},
			annots:  []pdf.Object{annot},
		}}
	})
	out := writeAndRead(t, src, nil, "")
	page := pageAt(t, out, 0)
	for _, name := range imageNames(t, out, page) {
		if _, got := samplesOf(t, out, xobject(t, out, page, name)); bytes.Equal(got, data) {
			t.Error("page content paints the original image")
		}
	}
	annot := pageAnnot(t, out, func(d pdf.Dict) bool { return d["Subtype"] == pdf.Name("Stamp") })
	ap := resolveDict(t, out, path(t, out, annot, "AP", "N"))
	im := path(t, out, ap, "Resources", "XObject", "Im1")
	if _, got := samplesOf(t, out, im); !bytes.Equal(got, data) {
		t.Error("appearance lost the original image")
	}
}

func TestImageActualText(t *testing.T) {
	data := secretRGB(imgW, imgH, nil)
	contentData := "/Span <</ActualText (secret)>> BDC\n" + placeAt100 + "EMC\n"
	src := singleImageDoc(t, rgbDict(imgW, imgH, data), contentData, [][4]int{region})
	out := writeAndRead(t, src, nil, "")
	var seen bool
	for _, op := range pageOps(t, out, pageAt(t, out, 0)) {
		if op.name != content.OpBeginMarkedContentWithProperties {
			continue
		}
		if d, ok := op.args[1].(pdf.Dict); ok {
			if _, has := d["ActualText"]; has {
				t.Errorf("/ActualText of the redacted span was kept")
			}
			seen = true
		}
	}
	if !seen {
		t.Error("no marked-content span with properties found")
	}
}

func TestImageUndecodable(t *testing.T) {
	build := func(contentData string, r [4]int) *pdf.Reader {
		return imageDoc(t, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
			ref := w.Alloc()
			stm, err := w.OpenStream(ref, pdf.Dict{
				"Subtype": pdf.Name("Image"),
				"Width":   pdf.Integer(4), "Height": pdf.Integer(4),
				"ColorSpace":       pdf.Name("DeviceFoo"),
				"BitsPerComponent": pdf.Integer(8),
			})
			if err != nil {
				t.Fatal(err)
			}
			stm.Write(make([]byte, 16))
			if err := stm.Close(); err != nil {
				t.Fatal(err)
			}
			return []imgPage{{
				content: contentData, xobjects: pdf.Dict{"Im1": ref},
				redacts: [][4]int{r},
			}}
		})
	}

	out := writeAndRead(t, build(placeAt100, region), nil, "")
	for _, n := range doNames(t, out, pageAt(t, out, 0)) {
		if n == "Im1" {
			t.Error("undecodable image under a region was kept")
		}
	}
	out = writeAndRead(t, build(placeAt100, [4]int{210, 210, 250, 250}), nil, "")
	found := false
	for _, n := range doNames(t, out, pageAt(t, out, 0)) {
		found = found || n == "Im1"
	}
	if !found {
		t.Error("undecodable image outside the regions was removed")
	}
}

func TestImageSeveralRegions(t *testing.T) {
	// the third region overlaps the first, and the fourth lies in the same
	// rows as the first two
	data := secretRGB(imgW, imgH, nil)
	regions := [][4]int{region, {165, 165, 188, 188}, {135, 128, 158, 141}, {175, 128, 185, 141}}
	src := singleImageDoc(t, rgbDict(imgW, imgH, data), placeAt100, regions)
	out := writeAndRead(t, src, nil, "")
	page, name := onlyImage(t, out)
	_, got := samplesOf(t, out, xobject(t, out, page, name))
	for r := range imgH {
		for c := range imgW {
			cb := cellBox(imgBox, imgW, imgH, r, c)
			hit := slices.ContainsFunc(regions, func(rg [4]int) bool {
				return cb.overlaps(box{float64(rg[0]), float64(rg[1]), float64(rg[2]), float64(rg[3])})
			})
			cell := got[(r*imgW+c)*3:][:3]
			if hit {
				if !bytes.Equal(cell, []byte{255, 255, 255}) {
					t.Errorf("cell (%d,%d) is %v", r, c, cell)
				}
			} else if !bytes.Equal(cell, data[(r*imgW+c)*3:][:3]) {
				t.Errorf("cell (%d,%d) changed", r, c)
			}
		}
	}
}

func TestImageJPX(t *testing.T) {
	const w, h = 10, 10
	im := &jpx.Image{Width: w, Height: h}
	for c := range 3 {
		im.Components = append(im.Components, jpx.Component{Depth: 8, DX: 1, DY: 1, Width: w, Height: h})
		pix := make([]int32, w*h)
		for i := range pix {
			pix[i] = int32((i*(c+3) + c*40) % 200)
		}
		im.Planes = append(im.Planes, jpx.Plane{Width: w, Height: h, Pix: pix})
		im.Channels = append(im.Channels, jpx.Channel{
			Component: c, PaletteColumn: -1, Kind: jpx.ChannelColor, Assoc: c + 1,
		})
	}
	var enc bytes.Buffer
	if err := jpx.Encode(&enc, im, &jpx.EncodeParams{Wrapper: true, Levels: 2}); err != nil {
		t.Fatal(err)
	}
	img := &image.Dict{
		Width: w, Height: h, ColorSpace: color.SpaceDeviceRGB,
		Data: image.NewJPXSource(enc.Bytes()),
	}
	src := singleImageDoc(t, img, placeAt100, [][4]int{region})
	out := writeAndRead(t, src, nil, "")

	cur := pdf.CursorAt(pdf.NewExtractor(src), nil)
	orig, err := image.ExtractDict(cur, xobject(t, src, pageAt(t, src, 0), "Im1"), false)
	if err != nil {
		t.Fatal(err)
	}
	want, err := orig.Load()
	if err != nil {
		t.Fatal(err)
	}

	page, name := onlyImage(t, out)
	cur = pdf.CursorAt(pdf.NewExtractor(out), nil)
	d, err := image.ExtractDict(cur, xobject(t, out, page, name), false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.Load()
	if err != nil {
		t.Fatal(err)
	}
	hit := hitCells(w, h)
	if len(hit) == 0 {
		t.Fatal("region meets no cell")
	}
	for r := range h {
		for c := range w {
			for k := range 3 {
				g := got.Pix[r*got.Stride+c*3+k]
				v := want.Pix[r*want.Stride+c*3+k]
				if hit[[2]int{r, c}] {
					v = 1
				}
				if math.Abs(float64(g-v)) > 1e-5 {
					t.Errorf("sample (%d,%d,%d) is %v, want %v", r, c, k, g, v)
				}
			}
		}
	}
}

func TestImageThinMatrix(t *testing.T) {
	// The image is 1e-307 units high, so that the inverse of its matrix maps
	// the region beyond the range of float64.
	data := secretRGB(imgW, imgH, func(x, y int) bool { return x >= 5 })
	tiny := "0." + strings.Repeat("0", 306) + "1"
	cm := "q 100 0 0 " + tiny + " 100 0 cm /Im1 Do Q\n"
	src := singleImageDoc(t, rgbDict(imgW, imgH, data), cm, [][4]int{{150, -10, 250, 50}})
	out := writeAndRead(t, src, nil, "")
	page, name := onlyImage(t, out)
	_, got := samplesOf(t, out, xobject(t, out, page, name))

	// columns 5 to 9 lie under the region, column 4 touches it
	white := []byte{255, 255, 255}
	for r := range imgH {
		for c := range imgW {
			i := (r*imgW + c) * 3
			switch {
			case c >= 5 && !bytes.Equal(got[i:i+3], white):
				t.Errorf("cell (%d,%d) under the region is %v", r, c, got[i:i+3])
			case c < 4 && !bytes.Equal(got[i:i+3], data[i:i+3]):
				t.Errorf("cell (%d,%d) outside the region changed", r, c)
			}
		}
	}
	checkNoSecret(t, out)
}

func TestImageFarRegion(t *testing.T) {
	// A thin region reaches from inside the image to x = 1e13.  The image
	// is wide enough that the far end of the region, in image space, lies
	// beyond the range of int.
	const w, h = 65536, 10
	data := bytes.Repeat([]byte{0x40}, w*h)
	src := imageDoc(t, func(pw *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
		ref, err := rm.Embed(grayDict(w, h, 8, data))
		if err != nil {
			t.Fatal(err)
		}
		quad := []float64{0.005, 0.95, 0.005, 0.96, 1e13, 0, 1e13, 0.2}
		var qp pdf.Array
		for _, x := range quad {
			qp = append(qp, pdf.Number(x))
		}
		aRef := pw.Alloc()
		if err := pw.Put(aRef, pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Redact"),
			"Rect":       pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Number(1e13), pdf.Integer(1)},
			"QuadPoints": qp,
		}); err != nil {
			t.Fatal(err)
		}
		return []imgPage{{
			content:  "q 0.02 0 0 1 0 0 cm /Im1 Do Q\n",
			xobjects: pdf.Dict{"Im1": ref},
			annots:   []pdf.Object{aRef},
		}}
	})
	out := writeAndRead(t, src, nil, "")
	page, name := onlyImage(t, out)
	_, got := samplesOf(t, out, xobject(t, out, page, name))

	// In image space the region covers x from 0.25 to beyond 1 and y from
	// 0.95 to 0.96, which lies in the first row.
	for r := range h {
		for c := range w {
			s := got[r*w+c]
			switch {
			case r == 0 && c >= w/4:
				if s != 255 {
					t.Fatalf("cell (%d,%d) under the region is %d", r, c, s)
				}
			case r > 0 || c < w/4-1:
				if s != 0x40 {
					t.Fatalf("cell (%d,%d) outside the region changed", r, c)
				}
			}
		}
	}
}

func TestImageOptionalContent(t *testing.T) {
	// the redacted copy of an image uses the optional content group of the
	// document, not a copy of it
	data := secretRGB(imgW, imgH, nil)
	stencil := &image.Mask{
		Width: imgW, Height: imgH,
		Source: image.NewFlateSource(imgW, color.SpaceDeviceGray, 1,
			writeBytes(make([]byte, 2*imgH))),
	}
	cases := []struct {
		name string
		img  func(g *oc.Group) pdf.Embedder
		oc   func(t *testing.T, out *pdf.Reader, img pdf.Dict) pdf.Object
	}{
		{"image", func(g *oc.Group) pdf.Embedder {
			d := rgbDict(imgW, imgH, data)
			d.OptionalContent = g
			return d
		}, func(t *testing.T, out *pdf.Reader, img pdf.Dict) pdf.Object {
			return img["OC"]
		}},
		{"stencil mask", func(g *oc.Group) pdf.Embedder {
			m := *stencil
			m.OptionalContent = g
			return &m
		}, func(t *testing.T, out *pdf.Reader, img pdf.Dict) pdf.Object {
			return img["OC"]
		}},
		{"mask of an image", func(g *oc.Group) pdf.Embedder {
			m := *stencil
			m.OptionalContent = g
			d := rgbDict(imgW, imgH, data)
			d.MaskImage = &m
			return d
		}, func(t *testing.T, out *pdf.Reader, img pdf.Dict) pdf.Object {
			return resolveDict(t, out, img["Mask"])["OC"]
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := imageDoc(t, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
				g := &oc.Group{Name: pdf.Text("Layer")}
				gRef, err := rm.Embed(g)
				if err != nil {
					t.Fatal(err)
				}
				ref, err := rm.Embed(c.img(g))
				if err != nil {
					t.Fatal(err)
				}
				w.GetMeta().Catalog.OCProperties = pdf.Dict{
					"OCGs": pdf.Array{gRef},
					"D":    pdf.Dict{"ON": pdf.Array{gRef}},
				}
				return []imgPage{{
					content:  placeAt100,
					xobjects: pdf.Dict{"Im1": ref},
					redacts:  [][4]int{region},
				}}
			})
			out := writeAndRead(t, src, nil, "")
			page, name := onlyImage(t, out)
			if name == "Im1" {
				t.Fatal("image was not redacted")
			}
			got := c.oc(t, out, resolveDict(t, out, xobject(t, out, page, name)))
			props := resolveDict(t, out, out.GetMeta().Catalog.OCProperties)
			ocgs, err := pdf.Resolve(out, props["OCGs"])
			if err != nil {
				t.Fatal(err)
			}
			want := ocgs.(pdf.Array)[0]
			if got != want {
				t.Errorf("image uses %v, but /OCProperties lists %v", got, want)
			}
		})
	}
}

// TestImageSharedEntries checks that the point data and the web capture
// identifier of a redacted image are written once, together with a kept image
// which uses them.
func TestImageSharedEntries(t *testing.T) {
	pt := &measure.PtData{
		Subtype: measure.PtDataSubtypeCloud,
		Names:   []string{"LAT", "LON"},
		XPTS:    [][]pdf.Object{{pdf.Number(1), pdf.Number(2)}},
	}
	id := &webcapture.Identifier{ID: []byte("0123456789abcdef")}
	src := imageDocVersion(t, pdf.V2_0, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
		var refs [2]pdf.Native
		for i := range refs {
			d := rgbDict(imgW, imgH, secretRGB(imgW, imgH, nil))
			d.PtData = pt
			d.WebCaptureID = id
			ref, err := rm.Embed(d)
			if err != nil {
				t.Fatal(err)
			}
			refs[i] = ref
		}
		return []imgPage{{
			content:  placeAt100 + "q 50 0 0 50 220 220 cm /Im2 Do Q\n",
			xobjects: pdf.Dict{"Im1": refs[0], "Im2": refs[1]},
			redacts:  [][4]int{region},
		}}
	})
	out := writeAndRead(t, src, nil, "")

	names := imageNames(t, out, pageAt(t, out, 0))
	if len(names) != 2 || !slices.Contains(names, "Im2") || slices.Contains(names, "Im1") {
		t.Fatalf("got images %v, want Im2 and a redacted copy of Im1", names)
	}
	ptData := countObjects(t, out, func(obj pdf.Native) bool {
		return dictOf(obj)["Type"] == pdf.Name("PtData")
	})
	if ptData != 1 {
		t.Errorf("output has %d point data dictionaries, want 1", ptData)
	}
	ids := countObjects(t, out, func(obj pdf.Native) bool {
		str, ok := obj.(pdf.String)
		return ok && bytes.Equal(str, id.ID)
	})
	if ids != 1 {
		t.Errorf("output has %d identifiers, want 1", ids)
	}
}

func TestImageCopyStructParent(t *testing.T) {
	// the redacted copy on the first page and the unchanged image on the
	// second page do not share their parent tree key
	elem := tagEntry{4, -1}
	in := nestDoc(t, []pdf.Dict{regionDict()}, func(b *nestBuilder) []nestPage {
		b.structTree(elem)
		d := rgbDict(imgW, imgH, secretRGB(imgW, imgH, inRegion))
		d.StructParent.Set(4)
		img, err := b.rm.Embed(d)
		if err != nil {
			t.Fatal(err)
		}
		res := pdf.Dict{"XObject": pdf.Dict{"Im1": img}}
		return []nestPage{
			{content: placeAt100, res: res},
			{content: placeAt100, res: res},
		}
	})
	out := writeAndRead(t, in, nil, "")
	checkDistinctKeys(t, out, streamKeys(t, out, "Image", "StructParent"), elem.idx)
	checkTagged(t, out, elem, true)
}

// jpxTestImage returns a JPEG 2000 image with one colour channel for each
// component.  The sample of component c at pixel i is pix(c, i).
func jpxTestImage(w, h int, depths []int, pix func(c, i int) int32) *jpx.Image {
	im := &jpx.Image{Width: w, Height: h}
	for c, d := range depths {
		im.Components = append(im.Components, jpx.Component{Depth: d, DX: 1, DY: 1, Width: w, Height: h})
		p := make([]int32, w*h)
		for i := range p {
			p[i] = pix(c, i)
		}
		im.Planes = append(im.Planes, jpx.Plane{Width: w, Height: h, Pix: p})
		im.Channels = append(im.Channels, jpx.Channel{
			Component: c, PaletteColumn: -1, Kind: jpx.ChannelColor, Assoc: c + 1,
		})
	}
	return im
}

// encodeJPX returns the JP2 file for im.
func encodeJPX(t *testing.T, im *jpx.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpx.Encode(&buf, im, &jpx.EncodeParams{Wrapper: true, Levels: 2}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// plainSample is a sample value which is not 0 and not the maximum.
func plainSample(c, i, depth int) int32 {
	return int32(1+(i*(c+3)+c*40)%200) << (depth - 8)
}

// redactedImage redacts the only image of the document, and returns the
// original and the redacted image dictionaries.
func redactedImage(t *testing.T, img pdf.Embedder) (orig, got *image.Dict) {
	t.Helper()
	src := singleImageDoc(t, img, placeAt100, [][4]int{region})
	out := writeAndRead(t, src, nil, "")

	var err error
	cur := pdf.CursorAt(pdf.NewExtractor(src), nil)
	orig, err = image.ExtractDict(cur, xobject(t, src, pageAt(t, src, 0), "Im1"), false)
	if err != nil {
		t.Fatal(err)
	}
	page, name := onlyImage(t, out)
	cur = pdf.CursorAt(pdf.NewExtractor(out), nil)
	got, err = image.ExtractDict(cur, xobject(t, out, page, name), false)
	if err != nil {
		t.Fatal(err)
	}
	return orig, got
}

// alphaAt returns the 8-bit alpha of pixel (x, y).
func alphaAt(a *goimage.Alpha, x, y int) uint8 { return a.Pix[y*a.Stride+x] }

// checkColour compares the colour samples of the redacted image with the
// original: cells which meet the region are white, all others are unchanged.
func checkColour(t *testing.T, orig, got *image.Dict, w, h, ncomp int) {
	t.Helper()
	want, err := orig.Load()
	if err != nil {
		t.Fatal(err)
	}
	have, err := got.Load()
	if err != nil {
		t.Fatal(err)
	}
	hit := hitCells(w, h)
	if len(hit) == 0 {
		t.Fatal("region meets no cell")
	}
	for r := range h {
		for c := range w {
			for k := range ncomp {
				v := want.Pix[r*want.Stride+c*ncomp+k]
				if hit[[2]int{r, c}] {
					v = 1
				}
				g := have.Pix[r*have.Stride+c*ncomp+k]
				if math.Abs(float64(g-v)) > 1e-5 {
					t.Errorf("sample (%d,%d,%d) is %v, want %v", r, c, k, g, v)
				}
			}
		}
	}
}

func TestImageJPXAlpha(t *testing.T) {
	const w, h = 10, 10
	src := jpxTestImage(w, h, []int{8, 8, 8, 8}, func(c, i int) int32 { return plainSample(c, i, 8) })
	src.Channels[3] = jpx.Channel{Component: 3, PaletteColumn: -1, Kind: jpx.ChannelOpacity}
	img := &image.Dict{
		Width: w, Height: h, ColorSpace: color.SpaceDeviceRGB, SMaskInData: 1,
		Data: image.NewJPXSource(encodeJPX(t, src)),
	}
	orig, got := redactedImage(t, img)

	if got.SMaskInData != 0 || got.SMask == nil {
		t.Fatalf("got SMaskInData %d and soft mask %v", got.SMaskInData, got.SMask)
	}
	checkColour(t, orig, got, w, h, 3)

	want, err := orig.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want.Alpha == nil {
		t.Fatal("original has no opacity channel")
	}
	alpha, err := got.SMask.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	hit := hitCells(w, h)
	for r := range h {
		for c := range w {
			wa := uint8(math.Round(float64(want.Alpha[r*w+c]) * 255))
			if hit[[2]int{r, c}] {
				wa = 0
			}
			if ga := alphaAt(alpha, c, r); ga != wa {
				t.Errorf("alpha (%d,%d) is %d, want %d", r, c, ga, wa)
			}
		}
	}
}

func TestImageJPXColourKey(t *testing.T) {
	const w, h = 10, 10
	keyed := func(i int) bool { return i%7 == 0 }
	src := jpxTestImage(w, h, []int{8, 8, 8}, func(c, i int) int32 {
		if keyed(i) {
			return 5
		}
		return plainSample(c, i, 8) + 20
	})
	img := &image.Dict{
		Width: w, Height: h, ColorSpace: color.SpaceDeviceRGB,
		MaskColors: []uint16{0, 10, 0, 10, 0, 10},
		Data:       image.NewJPXSource(encodeJPX(t, src)),
	}
	orig, got := redactedImage(t, img)

	if got.MaskColors != nil || got.SMask == nil {
		t.Fatal("colour key was not turned into a soft mask")
	}
	checkColour(t, orig, got, w, h, 3)

	alpha, err := got.SMask.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	hit := hitCells(w, h)
	for r := range h {
		for c := range w {
			wa := uint8(255)
			if keyed(r*w+c) || hit[[2]int{r, c}] {
				wa = 0
			}
			if ga := alphaAt(alpha, c, r); ga != wa {
				t.Errorf("alpha (%d,%d) is %d, want %d", r, c, ga, wa)
			}
		}
	}
}

func TestImageJPXHighDepth(t *testing.T) {
	const w, h = 10, 10
	src := jpxTestImage(w, h, []int{12, 12, 12}, func(c, i int) int32 {
		return int32(100 + (i*(c+3)*31)%3000)
	})
	img := &image.Dict{
		Width: w, Height: h, ColorSpace: color.SpaceDeviceRGB,
		Data: image.NewJPXSource(encodeJPX(t, src)),
	}
	orig, got := redactedImage(t, img)
	if got.BitsPerComponent != 16 {
		t.Errorf("got %d bits per component, want 16", got.BitsPerComponent)
	}
	checkColour(t, orig, got, w, h, 3)
}

func TestImageJPXSoftMask(t *testing.T) {
	const mw, mh = 20, 20
	sm := jpxTestImage(mw, mh, []int{8}, func(c, i int) int32 { return plainSample(1, i, 8) })
	soft := &image.SoftMask{
		Width: mw, Height: mh,
		Source: image.NewJPXSource(encodeJPX(t, sm)),
	}
	img := rgbDict(imgW, imgH, secretRGB(imgW, imgH, nil))
	img.SMask = soft
	orig, got := redactedImage(t, img)

	if got.SMask == nil {
		t.Fatal("soft mask was lost")
	}
	want, err := orig.SMask.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	have, err := got.SMask.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	if have.Rect.Dx() != mw || have.Rect.Dy() != mh {
		t.Fatalf("soft mask has size %v", have.Rect)
	}
	hit := hitCells(mw, mh)
	for r := range mh {
		for c := range mw {
			wa := alphaAt(want, c, r)
			if hit[[2]int{r, c}] {
				wa = 0
			}
			if ga := alphaAt(have, c, r); ga != wa {
				t.Errorf("soft mask (%d,%d) is %d, want %d", r, c, ga, wa)
			}
		}
	}
}

func TestImageJPXMask(t *testing.T) {
	const mw, mh = 20, 20
	// the stencil leaves every third pixel unpainted
	st := jpxTestImage(mw, mh, []int{1}, func(c, i int) int32 { return int32(i % 3 / 2) })
	m := &image.Mask{
		Width: mw, Height: mh,
		Source: image.NewJPXSource(encodeJPX(t, st)),
	}
	img := rgbDict(imgW, imgH, secretRGB(imgW, imgH, nil))
	img.MaskImage = m
	orig, got := redactedImage(t, img)

	if got.MaskImage == nil {
		t.Fatal("mask was lost")
	}
	want, err := orig.MaskImage.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	have, err := got.MaskImage.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	hit := hitCells(mw, mh)
	painted := 0
	for r := range mh {
		for c := range mw {
			wa := alphaAt(want, c, r)
			if hit[[2]int{r, c}] {
				wa = 0
			}
			ga := alphaAt(have, c, r)
			if ga != wa {
				t.Errorf("mask (%d,%d) is %d, want %d", r, c, ga, wa)
			}
			if ga != 0 {
				painted++
			}
		}
	}
	if painted == 0 {
		t.Error("mask paints nothing")
	}
}

func TestImage16Bit(t *testing.T) {
	const w, h = 10, 10
	for _, tc := range []struct {
		name  string
		cs    color.Space
		ncomp int
	}{
		{"gray", color.SpaceDeviceGray, 1},
		{"rgb", color.SpaceDeviceRGB, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := make([]byte, w*h*tc.ncomp*2)
			for i := range w * h * tc.ncomp {
				v := uint16(0x1234 + 0x0101*(i%50))
				data[2*i], data[2*i+1] = byte(v>>8), byte(v)
			}
			img := &image.Dict{
				Width: w, Height: h, ColorSpace: tc.cs, BitsPerComponent: 16,
				Data: image.NewFlateSource(w, tc.cs, 16, writeBytes(data)),
			}
			src := singleImageDoc(t, img, placeAt100, [][4]int{region})
			out := writeAndRead(t, src, nil, "")
			page, name := onlyImage(t, out)
			d, got := samplesOf(t, out, xobject(t, out, page, name))
			if d.BitsPerComponent != 16 {
				t.Fatalf("got %d bits per component", d.BitsPerComponent)
			}
			hit := hitCells(w, h)
			if len(hit) == 0 {
				t.Fatal("region meets no cell")
			}
			for r := range h {
				for c := range w {
					for k := range tc.ncomp {
						i := 2 * ((r*w+c)*tc.ncomp + k)
						want := data[i : i+2]
						if hit[[2]int{r, c}] {
							want = []byte{0xFF, 0xFF}
						}
						if !bytes.Equal(got[i:i+2], want) {
							t.Errorf("sample (%d,%d,%d) is %x, want %x", r, c, k, got[i:i+2], want)
						}
					}
				}
			}
		})
	}
}

// imgPage describes a page of a document built by [imageDoc].
type imgPage struct {
	content string

	// xobjects are the XObject resources of the page
	xobjects pdf.Dict

	// redacts are the rectangles of the redaction annotations
	redacts [][4]int

	// annots are additional annotations
	annots []pdf.Object
}

// imageDoc builds a document with 300x300 pages.  The function setup writes
// the shared objects and returns the pages.
func imageDoc(t testing.TB, setup func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage) *pdf.Reader {
	t.Helper()
	return imageDocVersion(t, pdf.V1_7, setup)
}

// imageDocVersion is like [imageDoc], for a document of PDF version v.
func imageDocVersion(t testing.TB, v pdf.Version, setup func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage) *pdf.Reader {
	t.Helper()
	return openSource(t, imageDocData(t, v, setup))
}

// imageDocData is like [imageDocVersion], but returns the bytes of the file.
func imageDocData(t testing.TB, v pdf.Version, setup func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage) []byte {
	t.Helper()
	w, buf := memfile.NewPDFWriter(t, v, nil)
	rm := pdf.NewResourceManager(w)
	pages := setup(w, rm)

	pagesRef := w.Alloc()
	var kids pdf.Array
	for _, pg := range pages {
		pageRef := w.Alloc()
		kids = append(kids, pageRef)

		cref := w.Alloc()
		stm, err := w.OpenStream(cref, pdf.Dict{}, pdf.FilterCompress{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stm.Write([]byte(pg.content)); err != nil {
			t.Fatal(err)
		}
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}

		var annots pdf.Array
		for _, r := range pg.redacts {
			ref := w.Alloc()
			if err := w.Put(ref, pdf.Dict{
				"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Redact"),
				"Rect": rect(r[0], r[1], r[2], r[3]), "P": pageRef,
			}); err != nil {
				t.Fatal(err)
			}
			annots = append(annots, ref)
		}
		annots = append(annots, pg.annots...)

		d := pdf.Dict{
			"Type": pdf.Name("Page"), "Parent": pagesRef,
			"MediaBox":  rect(0, 0, 300, 300),
			"Resources": pdf.Dict{"XObject": pg.xobjects},
			"Contents":  cref,
		}
		if len(annots) > 0 {
			d["Annots"] = annots
		}
		if err := w.Put(pageRef, d); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Put(pagesRef, pdf.Dict{
		"Type": pdf.Name("Pages"), "Kids": kids, "Count": pdf.Integer(len(pages)),
	}); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = pagesRef
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Data
}

func writeBytes(data []byte) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	}
}

// secretRGB returns the samples of a width x height RGB image.  The samples
// of the cells in cols x rows, which are listed in secret, hold the bytes
// DE AD BE; all other samples are smaller than 0xDE.
func secretRGB(width, height int, secret func(x, y int) bool) []byte {
	var data []byte
	for y := range height {
		for x := range width {
			if secret != nil && secret(x, y) {
				data = append(data, 0xDE, 0xAD, 0xBE)
				continue
			}
			data = append(data, byte(3*x+y), byte(x+5*y+1), byte(7*x+2*y+2))
		}
	}
	return data
}

func rgbDict(width, height int, data []byte) *image.Dict {
	return &image.Dict{
		Width: width, Height: height,
		ColorSpace: color.SpaceDeviceRGB, BitsPerComponent: 8,
		Data: image.NewFlateSource(width, color.SpaceDeviceRGB, 8, writeBytes(data)),
	}
}

func grayDict(width, height, bpc int, data []byte) *image.Dict {
	return &image.Dict{
		Width: width, Height: height,
		ColorSpace: color.SpaceDeviceGray, BitsPerComponent: bpc,
		Data: image.NewFlateSource(width, color.SpaceDeviceGray, bpc, writeBytes(data)),
	}
}

// page0 returns the dictionary of the page with the given index.
func pageAt(t testing.TB, r *pdf.Reader, n int) pdf.Dict {
	t.Helper()
	_, dicts := pageDicts(t, r)
	return dicts[n]
}

// doNames returns the names which the Do operators of a page use.
func doNames(t testing.TB, r *pdf.Reader, page pdf.Dict) []pdf.Name {
	t.Helper()
	var res []pdf.Name
	for _, op := range pageOps(t, r, page) {
		if op.name == content.OpXObject && len(op.args) > 0 {
			if n, ok := op.args[0].(pdf.Name); ok {
				res = append(res, n)
			}
		}
	}
	return res
}

// xobject returns the XObject resource of a page.
func xobject(t testing.TB, r *pdf.Reader, page pdf.Dict, name pdf.Name) pdf.Object {
	t.Helper()
	cur := pdf.CursorAt(pdf.NewExtractor(r), nil)
	res, err := cur.Dict(page["Resources"])
	if err != nil {
		t.Fatal(err)
	}
	xo, err := cur.Dict(res["XObject"])
	if err != nil {
		t.Fatal(err)
	}
	return xo[name]
}

// imageNames returns the names of the image XObjects of a page which are not
// the redaction marks.
func imageNames(t testing.TB, r *pdf.Reader, page pdf.Dict) []pdf.Name {
	t.Helper()
	cur := pdf.CursorAt(pdf.NewExtractor(r), nil)
	res, err := cur.Dict(page["Resources"])
	if err != nil {
		t.Fatal(err)
	}
	xo, err := cur.Dict(res["XObject"])
	if err != nil {
		t.Fatal(err)
	}
	var names []pdf.Name
	for name, obj := range xo {
		stm, err := cur.Stream(obj)
		if err != nil {
			t.Fatal(err)
		}
		if st, _ := cur.Name(stm.Dict["Subtype"]); st == "Image" {
			names = append(names, name)
		}
	}
	return names
}

// samplesOf returns the packed samples of an image object.
func samplesOf(t testing.TB, r *pdf.Reader, obj pdf.Object) (*image.Dict, []byte) {
	t.Helper()
	cur := pdf.CursorAt(pdf.NewExtractor(r), nil)
	d, err := image.ExtractDict(cur, obj, false)
	if err != nil {
		t.Fatal(err)
	}
	sr, err := d.SampleRows()
	if err != nil {
		t.Fatal(err)
	}
	var res []byte
	for _, row := range sr.All() {
		res = append(res, row...)
	}
	if err := sr.Err(); err != nil {
		t.Fatal(err)
	}
	return d, res
}

// maskSamplesOf returns the packed samples of an image mask object.
func maskSamplesOf(t testing.TB, r *pdf.Reader, obj pdf.Object) (*image.Mask, []byte) {
	t.Helper()
	cur := pdf.CursorAt(pdf.NewExtractor(r), nil)
	m, err := image.ExtractMask(cur, obj, false)
	if err != nil {
		t.Fatal(err)
	}
	return m, maskRows(t, m)
}

func maskRows(t testing.TB, m *image.Mask) []byte {
	t.Helper()
	sr, err := m.SampleRows()
	if err != nil {
		t.Fatal(err)
	}
	var res []byte
	for _, row := range sr.All() {
		res = append(res, row...)
	}
	if err := sr.Err(); err != nil {
		t.Fatal(err)
	}
	return res
}

// box is an axis-parallel rectangle in page space.
type box struct{ x0, y0, x1, y1 float64 }

// overlaps reports whether the interiors of two boxes overlap.
func (b box) overlaps(o box) bool {
	return b.x0 < o.x1 && o.x0 < b.x1 && b.y0 < o.y1 && o.y0 < b.y1
}

// cellBox returns the page-space box of the cell (row, col) of an image of
// cols x rows cells which fills the square img.
func cellBox(img box, cols, rows, r, c int) box {
	w, h := (img.x1-img.x0)/float64(cols), (img.y1-img.y0)/float64(rows)
	return box{
		img.x0 + float64(c)*w, img.y1 - float64(r+1)*h,
		img.x0 + float64(c+1)*w, img.y1 - float64(r)*h,
	}
}

// fileStreams returns the decoded contents of all streams of a document.
func fileStreams(t testing.TB, r *pdf.Reader) [][]byte {
	t.Helper()
	var res [][]byte
	for i := uint32(1); i < 200; i++ {
		obj, err := r.Get(pdf.NewReference(i, 0), true)
		if err != nil {
			continue
		}
		stm, ok := obj.(*pdf.Stream)
		if !ok {
			continue
		}
		rd, err := pdf.DecodeStream(r, nil, stm)
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(rd)
		rd.Close()
		res = append(res, data)
	}
	return res
}

// checkNoSecret fails if a decoded stream of the document contains the
// distinctive sample bytes of [secretRGB].
func checkNoSecret(t testing.TB, r *pdf.Reader) {
	t.Helper()
	secret := []byte{0xDE, 0xAD, 0xBE, 0xDE, 0xAD, 0xBE}
	for i, data := range fileStreams(t, r) {
		if bytes.Contains(data, secret) {
			t.Errorf("stream %d holds samples of the redacted area", i)
		}
	}
}

// cutFlate returns the zlib encoding of data, cut off in the middle.
func cutFlate(data []byte) []byte {
	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	zw.Write(data)
	zw.Close()
	return zbuf.Bytes()[:zbuf.Len()/2]
}

// brokenFlate returns Flate data whose compressed bytes are corrupt.
func brokenFlate(data []byte) []byte {
	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	zw.Write(data)
	zw.Close()
	b := zbuf.Bytes()
	for i := 2; i < len(b)-4; i++ {
		b[i] = 0xFF
	}
	return b
}

// TestImageBrokenFlate checks that an image, or a mask of an image, whose
// Flate data is cut off or corrupt is redacted like any other image.  Such
// data reads as far as it decodes, followed by zero samples.
func TestImageBrokenFlate(t *testing.T) {
	const mw, mh = 40, 40
	secretAlpha := bytes.Repeat([]byte{0xAB}, mw*mh)
	secretStencil := bytes.Repeat([]byte{0xA5}, (mw+7)/8*mh)

	encodings := map[string]func([]byte) []byte{"cut": cutFlate, "corrupt": brokenFlate}
	for encName, enc := range encodings {
		for _, part := range []string{"image", "softmask", "mask"} {
			t.Run(encName+"/"+part, func(t *testing.T) {
				samples := secretRGB(mw, mh, nil)
				src := imageDoc(t, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
					rawStream := func(d pdf.Dict, data []byte) pdf.Reference {
						d["Filter"] = pdf.Name("FlateDecode")
						return putRawStream(t, w, d, data)
					}
					imgData := encodeFlate(samples)
					dict := pdf.Dict{
						"Subtype": pdf.Name("Image"),
						"Width":   pdf.Integer(mw), "Height": pdf.Integer(mh),
						"ColorSpace":       pdf.Name("DeviceRGB"),
						"BitsPerComponent": pdf.Integer(8),
					}
					switch part {
					case "image":
						imgData = enc(samples)
					case "softmask":
						dict["SMask"] = rawStream(pdf.Dict{
							"Subtype": pdf.Name("Image"),
							"Width":   pdf.Integer(mw), "Height": pdf.Integer(mh),
							"ColorSpace":       pdf.Name("DeviceGray"),
							"BitsPerComponent": pdf.Integer(8),
						}, enc(secretAlpha))
					case "mask":
						dict["Mask"] = rawStream(pdf.Dict{
							"Subtype": pdf.Name("Image"),
							"Width":   pdf.Integer(mw), "Height": pdf.Integer(mh),
							"ImageMask":        pdf.Boolean(true),
							"BitsPerComponent": pdf.Integer(1),
						}, enc(secretStencil))
					}
					ref := rawStream(dict, imgData)
					return []imgPage{{
						content: placeAt100, xobjects: pdf.Dict{"Im1": ref},
						redacts: [][4]int{region},
					}}
				})
				out := writeAndRead(t, src, nil, "")
				if part != "image" {
					return
				}
				page, name := onlyImage(t, out)
				cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
				got, err := image.ExtractDict(cur, xobject(t, out, page, name), false)
				if err != nil {
					t.Fatal(err)
				}
				have, err := got.Load()
				if err != nil {
					t.Fatal(err)
				}
				for c := range hitCells(mw, mh) {
					for k := range 3 {
						if v := have.Pix[c[0]*have.Stride+c[1]*3+k]; v != 1 {
							t.Fatalf("sample (%d,%d,%d) is %v, want 1", c[0], c[1], k, v)
						}
					}
				}
			})
		}
	}
}

// failingData is image data whose samples cannot be read, failing with err.
type failingData struct{ err error }

func (failingData) WriteStream(*pdf.EmbedHelper, pdf.Reference, pdf.Dict) error { return nil }
func (d failingData) Pixels() ([]byte, error)                                   { return nil, d.err }
func (failingData) IsJPX() bool                                                 { return false }

// unreadableDicts returns images in which the image data, the soft mask or
// the mask fails to open with err.
func unreadableDicts(err error) map[string]*image.Dict {
	good := func() *image.Dict { return rgbDict(4, 4, make([]byte, 48)) }
	d := good()
	d.Data = failingData{err}
	smDict := good()
	smDict.SMask = &image.SoftMask{Width: 4, Height: 4, BitsPerComponent: 8, Source: failingData{err}}
	maskDict := good()
	maskDict.MaskImage = &image.Mask{Width: 4, Height: 4, Source: failingData{err}}
	return map[string]*image.Dict{"image": d, "softmask": smDict, "mask": maskDict}
}

// TestImageUnreadableData checks that image data which cannot be opened
// because the file is malformed is reported as undecodable, also for the
// masks of an image.
func TestImageUnreadableData(t *testing.T) {
	x := &imgRedaction{regions: imageRegions{hulls: []poly{{v(0, 0), v(1, 0), v(1, 1), v(0, 1)}}}, charge: noCharge}
	for name, img := range unreadableDicts(pdf.Error("unsupported filter")) {
		if _, err := x.dict(img); !errors.Is(err, errUndecodable) {
			t.Errorf("%s: got error %v, want errUndecodable", name, err)
		}
	}
}

// faultyReaderAt reads data, but fails with err for reads which touch the
// byte range [from, to).
type faultyReaderAt struct {
	data     []byte
	from, to int64
	err      error
}

func (f *faultyReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < f.to && off+int64(len(p)) > f.from {
		return 0, f.err
	}
	return bytes.NewReader(f.data).ReadAt(p, off)
}

// encodeFlate returns the zlib encoding of data.
func encodeFlate(data []byte) []byte {
	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	zw.Write(data)
	zw.Close()
	return zbuf.Bytes()
}

func TestRawSample(t *testing.T) {
	cases := []struct {
		v          float32
		dMin, dMax float64
		want       float64
	}{
		{0, 0, 1, 0},
		{1, 0, 1, 255},
		{0.5, 0, 1, 127.5},
		{0, 1, 0, 255},
		{50, 0, 100, 127.5},
		{2, 0, 1, 510},
		// a Decode range without extent maps every value to 0
		{0.3, 0.5, 0.5, 0},
	}
	for _, c := range cases {
		if got := rawSample(c.v, c.dMin, c.dMax, 255); math.Abs(got-c.want) > 1e-4 {
			t.Errorf("rawSample(%g, %g, %g) = %g, want %g", c.v, c.dMin, c.dMax, got, c.want)
		}
	}
}
