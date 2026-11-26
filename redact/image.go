// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"iter"
	"maps"
	"math"
	"slices"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/internal/limits"
	"seehuhn.de/go/pdf/measure"
	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/optional"
	"seehuhn.de/go/pdf/webcapture"
)

// errUndecodable is returned for an image whose samples cannot be read.
var errUndecodable = errors.New("image cannot be decoded")

// An imageSrc is an image XObject among the resources of a stream.
type imageSrc struct {
	// ref is the reference of the image stream, and dict its dictionary
	ref  pdf.Reference
	dict pdf.Dict

	// img is the decoded image, or nil if the image cannot be decoded
	img graphics.Image
}

// shareImage makes the redacted copy img of the image src share with the rest
// of the output the values which it takes over from the source image: the
// colour space, the optional content, the measure, the point data and the
// web capture identifier, also of the stencil mask.
func (red *redactor) shareImage(img graphics.Image, src *imageSrc) error {
	switch im := img.(type) {
	case *image.Dict:
		// a JPXDecode image can take its colour space from the codestream
		if orig, ok := src.img.(*image.Dict); ok && im.ColorSpace != nil && im.ColorSpace == orig.ColorSpace {
			if err := red.share(im.ColorSpace, src.dict["ColorSpace"]); err != nil {
				return err
			}
		}
		err := red.shareEntries(src.dict, im.OptionalContent, im.Measure, im.PtData, im.WebCaptureID)
		if err != nil || im.MaskImage == nil {
			return err
		}
		mask, err := pdf.Optional(pdf.CursorAt(red.x, nil).Stream(src.dict["Mask"]))
		if err != nil || mask == nil {
			return err
		}
		m := im.MaskImage
		return red.shareEntries(mask.Dict, m.OptionalContent, m.Measure, m.PtData, m.WebCaptureID)
	case *image.Mask:
		return red.shareEntries(src.dict, im.OptionalContent, im.Measure, im.PtData, im.WebCaptureID)
	}
	return nil
}

// renumberImage gives a redacted copy of an image, and its stencil mask,
// their own /StructParent keys, see [redactor.renumberCopy].
func (red *redactor) renumberImage(img graphics.Image) error {
	var keys []*optional.UInt
	switch im := img.(type) {
	case *image.Dict:
		keys = append(keys, &im.StructParent)
		if im.MaskImage != nil {
			keys = append(keys, &im.MaskImage.StructParent)
		}
	case *image.Mask:
		keys = append(keys, &im.StructParent)
	}
	for _, key := range keys {
		if err := red.renumberKey(key); err != nil {
			return err
		}
	}
	return nil
}

// renumberKey replaces the /StructParent key of a redacted copy of an image
// by a new key, which repeats the parent tree entry of the old one.  If no
// key can be added to the parent tree, the key is removed.
func (red *redactor) renumberKey(key *optional.UInt) error {
	old, ok := key.Get()
	if !ok || old > math.MaxInt32 {
		return nil
	}
	k, ok, err := red.copyParentKey(pdf.Integer(old))
	if err != nil {
		return err
	}
	if ok {
		key.Set(uint(k))
	} else {
		key.Clear()
	}
	return nil
}

// shareEntries implements shareImage for the entries which images and
// stencil masks have in common.  The source dictionary is dict.
func (red *redactor) shareEntries(dict pdf.Dict, c oc.Conditional, m measure.Measure, pt *measure.PtData, id *webcapture.Identifier) error {
	if c != nil {
		if err := red.share(c, dict["OC"]); err != nil {
			return err
		}
	}
	if m != nil {
		if err := red.share(m, dict["Measure"]); err != nil {
			return err
		}
	}
	if pt != nil {
		if err := red.share(pt, dict["PtData"]); err != nil {
			return err
		}
	}
	if id != nil {
		if err := red.share(id, dict["ID"]); err != nil {
			return err
		}
	}
	return nil
}

// placement says what happens to an image which is painted under a matrix.
type placement int

const (
	// placeKeep leaves the image alone: it misses all regions
	placeKeep placement = iota

	// placeDrop removes the image from the content
	placeDrop

	// placeRedact overwrites the samples under the regions
	placeRedact
)

// unitSquare is the image space of an image.
var unitSquare = []vec.Vec2{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}}

// imageRegions are the regions which meet an image, in image space.
type imageRegions struct {
	hulls []poly

	// touch is a distance in image space which the matrix of the image maps
	// to at most touchTol/√2 in page space
	touch float64
}

// placeImage decides what happens to an image painted with the matrix m,
// which maps image space to page space.  For placeRedact the result gives
// the regions which meet the image, in image space.
func placeImage(m matrix.Matrix, hulls []poly) (placement, imageRegions) {
	corners := transformPoints(unitSquare, m)
	footprint := convexHull(corners)
	var hit []poly
	for _, h := range hulls {
		if !intersects(footprint, h, touchTol) {
			continue
		}
		if inside(corners, h, 0) {
			return placeDrop, imageRegions{}
		}
		hit = append(hit, h)
	}
	if len(hit) == 0 {
		return placeKeep, imageRegions{}
	}

	inv, ok := m.Inv()
	if !ok || len(footprint) < 3 {
		return placeDrop, imageRegions{}
	}

	// Clipping the regions to the footprint keeps their image-space
	// coordinates in range, even where the inverse matrix is huge.  The
	// margin keeps rounding from removing the parts at the edges of a thin
	// footprint.
	scale := 0.0
	for _, v := range footprint {
		scale = max(scale, math.Abs(v.X), math.Abs(v.Y))
	}
	clipTo := footprint.grow(clipMargin * (1 + scale))
	for i, h := range hit {
		hit[i] = imageHull(clipConvex(h, clipTo), inv)
	}
	touch := 0.0
	if s := sigmaMax(m); s > 0 {
		touch = touchTol / (math.Sqrt2 * s)
	}
	return placeRedact, imageRegions{hulls: hit, touch: touch}
}

const (
	// clipMargin is the margin, relative to the size of the page-space
	// coordinates, by which the footprint of an image is grown before the
	// regions are clipped to it
	clipMargin = 1e-9

	// roundErr bounds the relative rounding error of a coordinate which is
	// computed by clipping and then transforming a polygon
	roundErr = 64 * 0x1p-52

	// boxLo and boxHi bound the square to which polygons in image space are
	// clipped, which contains the image with a margin
	boxLo, boxHi = -1.0, 2.0
)

// imageHull maps the convex polygon with vertices pts, which are given in
// page space and clipped to the footprint of an image, to image space using
// inv.  The result is enlarged by a bound on the rounding error and clipped
// to a square around the image, so that its coordinates are small.  An empty
// polygon, which only rounding or overflow can produce, and an image-space
// polygon which is not finite both give the whole image.
func imageHull(pts []vec.Vec2, inv matrix.Matrix) poly {
	if len(pts) == 0 {
		return poly(unitSquare)
	}
	size := 0.0
	for _, v := range pts {
		size = max(size, math.Abs(v.X), math.Abs(v.Y))
	}
	// the order of the operations avoids overflow for large inv
	rx := (roundErr*math.Abs(inv[0])+roundErr*math.Abs(inv[2]))*size + roundErr*math.Abs(inv[4])
	ry := (roundErr*math.Abs(inv[1])+roundErr*math.Abs(inv[3]))*size + roundErr*math.Abs(inv[5])

	img := transformPoints(pts, inv)
	if !(rx <= cellEps && ry <= cellEps) {
		img = boxCorners(img, rx, ry)
	}
	if !poly(img).isFinite() {
		return poly(unitSquare)
	}
	return convexHull(clipBox(scaledHull(img), boxLo, boxHi))
}

// cellsFor returns, for each row of an image of the given size, the sorted
// and disjoint half-open ranges of the columns of the cells which are
// overwritten.
//
// A cell is overwritten if a region covers a point of the cell which lies
// further inside the cell than reg.touch, or than a quarter of the size of
// the cell.  Where a region covers only points closer to the edges, the cell
// is kept: in page space, the part of a kept cell which lies under the
// regions is within touchTol of the edges of the cell.  Thus cells which only
// touch a region are kept, and so is an image which meets the regions by no
// more than touchTol, see [placeImage].
func cellsFor(reg imageRegions, width, height int) [][][2]int {
	ex := min(reg.touch, 0.25/float64(width))
	ey := min(reg.touch, 0.25/float64(height))
	cells := make([][][2]int, height)
	for _, h := range reg.hulls {
		for r, rng := range cellRanges(h, width, height, ex, ey) {
			cells[r] = append(cells[r], rng)
		}
	}
	for r, rs := range cells {
		if len(rs) < 2 {
			continue
		}
		slices.SortFunc(rs, func(a, b [2]int) int { return a[0] - b[0] })
		merged := rs[:1]
		for _, x := range rs[1:] {
			last := &merged[len(merged)-1]
			if x[0] <= last[1] {
				last[1] = max(last[1], x[1])
			} else {
				merged = append(merged, x)
			}
		}
		cells[r] = merged
	}
	return cells
}

// setSample stores the value v as sample i of a packed row of samples of bpc
// bits.
func setSample(row []byte, i, bpc int, v uint16) {
	switch bpc {
	case 8:
		row[i] = byte(v)
	case 16:
		row[2*i] = byte(v >> 8)
		row[2*i+1] = byte(v)
	default:
		bit := i * bpc
		shift := 8 - bpc - bit%8
		mask := byte(1<<bpc-1) << shift
		row[bit/8] = row[bit/8]&^mask | byte(v)<<shift
	}
}

// fillCells overwrites the pixels in the column ranges with the sample
// values fill, one for each component.
func fillCells(row []byte, ranges [][2]int, bpc int, fill []uint16) {
	ncomp := len(fill)
	for _, r := range ranges {
		for p := r[0]; p < r[1]; p++ {
			for k, v := range fill {
				setSample(row, p*ncomp+k, bpc, v)
			}
		}
	}
}

// fillValues returns the raw sample values with which the pixels of an image
// are overwritten: the largest raw value for DeviceGray and DeviceRGB, which
// is white under the default Decode array, and 0 otherwise.
func fillValues(cs color.Space, bpc int) []uint16 {
	fill := make([]uint16, cs.Channels())
	switch cs.Family() {
	case color.FamilyDeviceGray, color.FamilyDeviceRGB:
		for i := range fill {
			fill[i] = uint16(1<<bpc - 1)
		}
	}
	return fill
}

// A rowSource delivers the packed rows of an image.
type rowSource interface {
	All() iter.Seq2[int, []byte]
	Err() error
}

// memRows delivers the rows of packed samples held in memory.
type memRows struct {
	data     []byte
	rowBytes int
	height   int
}

func (m *memRows) All() iter.Seq2[int, []byte] {
	return func(yield func(int, []byte) bool) {
		for y := range m.height {
			if !yield(y, m.data[y*m.rowBytes:][:m.rowBytes]) {
				return
			}
		}
	}
}

func (m *memRows) Err() error { return nil }

// An imgRedaction overwrites the samples of images.
type imgRedaction struct {
	// regions are the regions to remove
	regions imageRegions

	// charge accounts for the work and the bytes written
	charge func(n int64) error
}

// cells returns, for each row of an image of the given size, the ranges of
// columns which meet one of the regions.  The work and memory for this are
// proportional to the height of the image.
func (x *imgRedaction) cells(width, height int) ([][][2]int, error) {
	if err := x.charge(int64(height) * int64(1+len(x.regions.hulls))); err != nil {
		return nil, err
	}
	return cellsFor(x.regions, width, height), nil
}

// readable returns the sample reader sr, or errUndecodable if the image has
// no samples to read or its stream cannot be opened because the file is
// malformed, for example because of an unknown filter.  Other errors in
// opening the stream, such as read errors, are returned unchanged.  Truncated
// or corrupt data needs no check here, since the reader pads it.
func readable(sr *image.SampleReader, err error) (*image.SampleReader, error) {
	if err != nil {
		return nil, errUndecodable
	}
	// the stream is opened on iteration
	for range sr.All() {
		break
	}
	switch err := sr.Err(); {
	case pdf.IsMalformed(err):
		return nil, errUndecodable
	case err != nil:
		return nil, err
	}
	return sr, nil
}

// writer returns a function which writes the rows of src with the cells
// overwritten.
func (x *imgRedaction) writer(src rowSource, cells [][][2]int, bpc int, fill []uint16) func(io.Writer) error {
	return func(w io.Writer) error {
		for y, row := range src.All() {
			fillCells(row, cells[y], bpc, fill)
			if err := x.charge(int64(len(row))); err != nil {
				return err
			}
			if _, err := w.Write(row); err != nil {
				return err
			}
		}
		return src.Err()
	}
}

// image returns a copy of img in which the samples under the regions are
// overwritten.  The copy does not carry the metadata and the alternate
// images, which can hold the original pixels.  The error is errUndecodable
// if the samples of the image cannot be read.
func (x *imgRedaction) image(img graphics.Image) (graphics.Image, error) {
	switch im := img.(type) {
	case *image.Dict:
		return x.dict(im)
	case *image.Mask:
		return x.mask(im)
	}
	return nil, errUndecodable
}

// dict redacts an image which is not an image mask.
func (x *imgRedaction) dict(d *image.Dict) (*image.Dict, error) {
	if d.Data == nil {
		return nil, errUndecodable
	}
	cp := *d
	cp.Alternates = nil
	cp.Metadata = nil
	cp.AssociatedFiles = nil
	cp.Name = ""

	if d.Data.IsJPX() {
		if err := x.jpxDict(d, &cp); err != nil {
			return nil, err
		}
	} else {
		sr, err := readable(d.SampleRows())
		if err != nil {
			return nil, err
		}
		cells, err := x.cells(d.Width, d.Height)
		if err != nil {
			return nil, err
		}
		cp.Data = image.NewFlateSource(d.Width, d.ColorSpace, d.BitsPerComponent,
			x.writer(sr, cells, d.BitsPerComponent, fillValues(d.ColorSpace, d.BitsPerComponent)))
	}

	if d.SMask != nil {
		sm, err := x.softMask(d.SMask)
		if err != nil {
			return nil, err
		}
		cp.SMask = sm
	}
	if d.MaskImage != nil {
		m, err := x.mask(d.MaskImage)
		if err != nil {
			return nil, err
		}
		cp.MaskImage = m
	}
	return &cp, nil
}

// jpxDict decodes a JPXDecode image and stores its samples, with the cells
// overwritten, in cp.  The opacity channel and the colour key become a soft
// mask.
func (x *imgRedaction) jpxDict(d, cp *image.Dict) error {
	channels := 1 + limits.MaxImageChannels
	if d.ColorSpace != nil {
		channels = 1 + d.ColorSpace.Channels()
	}
	if limits.ImageDecodedExceedsLimit(d.Width, d.Height, channels) {
		return errUndecodable
	}
	data, err := d.Load()
	if err != nil {
		return errUndecodable
	}
	width, height := data.Rect.Dx(), data.Rect.Dy()
	cs, ncomp := data.CS, data.NComp
	if err := x.charge(int64(len(data.Pix))); err != nil {
		return err
	}

	// 8 bits suffice if the samples are multiples of 1/255 of the range
	bpc := 8
	dec := image.DefaultDecode(cs, 8)
	if cs.Family() != color.FamilyIndexed {
		for i, v := range data.Pix {
			c := i % ncomp
			s := rawSample(v, dec[2*c], dec[2*c+1], 255)
			if math.Abs(s-math.Round(s)) > 0.02 {
				bpc = 16
				break
			}
		}
	}
	maxRaw := float64(int(1)<<bpc - 1)
	rowBytes := (width*ncomp*bpc + 7) / 8
	raw := make([]byte, rowBytes*height)
	for y := range height {
		row := raw[y*rowBytes:][:rowBytes]
		for i := range width * ncomp {
			c := i % ncomp
			v := data.Pix[y*data.Stride+i]
			q := uint16(math.Round(min(max(rawSample(v, dec[2*c], dec[2*c+1], maxRaw), 0), maxRaw)))
			setSample(row, i, bpc, q)
		}
	}
	cells, err := x.cells(width, height)
	if err != nil {
		return err
	}
	src := &memRows{data: raw, rowBytes: rowBytes, height: height}

	cp.Width, cp.Height = width, height
	cp.ColorSpace = cs
	cp.BitsPerComponent = bpc
	cp.Decode = nil
	cp.SMaskInData = 0
	cp.Data = image.NewFlateSource(width, cs, bpc,
		x.writer(src, cells, bpc, fillValues(cs, bpc)))

	if d.SMask == nil && (data.Alpha != nil || data.KeyMask != nil) {
		alpha := make([]byte, width*height)
		for y := range height {
			for xx := range width {
				a := 255.0
				if data.Alpha != nil {
					a = math.Round(min(max(float64(data.Alpha[y*width+xx]), 0), 1) * 255)
				}
				if data.KeyMask != nil && data.KeyMask.Pix[y*data.KeyMask.Stride+xx] == 0 {
					a = 0
				}
				alpha[y*width+xx] = byte(a)
			}
		}
		cp.SMask = &image.SoftMask{
			Width: width, Height: height, BitsPerComponent: 8,
			Source: image.NewFlateSource(width, color.SpaceDeviceGray, 8,
				x.writer(&memRows{data: alpha, rowBytes: width, height: height},
					cells, 8, []uint16{0})),
		}
		cp.MaskImage = nil
	}
	cp.MaskColors = nil
	return nil
}

// rawSample returns the raw sample value, on the scale from 0 to maxRaw,
// which a decoded component value v corresponds to under the Decode range
// [dMin, dMax].  The result lies outside this scale if v lies outside the
// Decode range.
func rawSample(v float32, dMin, dMax, maxRaw float64) float64 {
	if dMax == dMin {
		return 0
	}
	return (float64(v) - dMin) / (dMax - dMin) * maxRaw
}

// softMask redacts a soft-mask image.  The samples under the regions become
// fully transparent.
func (x *imgRedaction) softMask(sm *image.SoftMask) (*image.SoftMask, error) {
	if sm.Source == nil {
		return nil, errUndecodable
	}
	cp := *sm
	if sm.Source.IsJPX() {
		alpha, err := sm.LoadAlpha()
		if err != nil {
			return nil, errUndecodable
		}
		width, height := alpha.Rect.Dx(), alpha.Rect.Dy()
		raw := make([]byte, width*height)
		for y := range height {
			copy(raw[y*width:][:width], alpha.Pix[y*alpha.Stride:])
		}
		cells, err := x.cells(width, height)
		if err != nil {
			return nil, err
		}
		cp.Width, cp.Height = width, height
		cp.BitsPerComponent = 8
		cp.Decode = nil
		cp.Source = image.NewFlateSource(width, color.SpaceDeviceGray, 8,
			x.writer(&memRows{data: raw, rowBytes: width, height: height},
				cells, 8, []uint16{0}))
		return &cp, nil
	}

	sr, err := readable(sm.SampleRows())
	if err != nil {
		return nil, err
	}
	transparent := uint16(0)
	if len(sm.Decode) >= 2 && sm.Decode[0] > sm.Decode[1] {
		transparent = uint16(1<<sm.BitsPerComponent - 1)
	}
	cells, err := x.cells(sm.Width, sm.Height)
	if err != nil {
		return nil, err
	}
	cp.Source = image.NewFlateSource(sm.Width, color.SpaceDeviceGray, sm.BitsPerComponent,
		x.writer(sr, cells, sm.BitsPerComponent, []uint16{transparent}))
	return &cp, nil
}

// mask redacts an image mask.  The samples under the regions are not
// painted.
func (x *imgRedaction) mask(m *image.Mask) (*image.Mask, error) {
	if m.Source == nil {
		return nil, errUndecodable
	}
	cp := *m
	cp.Alternates = nil
	cp.Metadata = nil
	cp.AssociatedFiles = nil
	cp.Name = ""

	if m.Source.IsJPX() {
		alpha, err := m.LoadAlpha()
		if err != nil {
			return nil, errUndecodable
		}
		width, height := alpha.Rect.Dx(), alpha.Rect.Dy()
		rowBytes := (width + 7) / 8
		raw := make([]byte, rowBytes*height)
		for y := range height {
			for xx := range width {
				if alpha.Pix[y*alpha.Stride+xx] == 0 {
					setSample(raw[y*rowBytes:][:rowBytes], xx, 1, 1)
				}
			}
		}
		cells, err := x.cells(width, height)
		if err != nil {
			return nil, err
		}
		cp.Width, cp.Height = width, height
		cp.Inverted = false
		cp.Source = image.NewFlateSource(width, color.SpaceDeviceGray, 1,
			x.writer(&memRows{data: raw, rowBytes: rowBytes, height: height},
				cells, 1, []uint16{1}))
		return &cp, nil
	}

	sr, err := readable(m.SampleRows())
	if err != nil {
		return nil, err
	}
	unpainted := uint16(1)
	if m.Inverted {
		unpainted = 0
	}
	cells, err := x.cells(m.Width, m.Height)
	if err != nil {
		return nil, err
	}
	cp.Source = image.NewFlateSource(m.Width, color.SpaceDeviceGray, 1,
		x.writer(sr, cells, 1, []uint16{unpainted}))
	return &cp, nil
}

// inlineDropped lists the entries of an inline image dictionary which
// redactInline replaces.
var inlineDropped = []pdf.Name{
	"F", "Filter", "DP", "DecodeParms", "L", "Length", "BPC", "BitsPerComponent",
}

// redactInline returns the inline image operator with the samples under the
// regions overwritten.  The image data is stored with FlateDecode.  The
// error is errUndecodable if the samples of the image cannot be read.
func redactInline(op content.Operator, res *content.Resources, regions imageRegions, charge func(int64) error) (content.Operator, error) {
	img, err := image.ExtractInline(op, res)
	if err != nil {
		return content.Operator{}, errUndecodable
	}
	x := &imgRedaction{regions: regions, charge: charge}
	out, err := x.image(img)
	if err != nil {
		return content.Operator{}, err
	}

	var src *image.FlateSource
	dict := maps.Clone(op.Args[0].(pdf.Dict))
	for _, key := range inlineDropped {
		delete(dict, key)
	}
	switch o := out.(type) {
	case *image.Dict:
		src, _ = o.Data.(*image.FlateSource)
		dict["BPC"] = pdf.Integer(o.BitsPerComponent)
	case *image.Mask:
		src, _ = o.Source.(*image.FlateSource)
	}
	if src == nil {
		return content.Operator{}, errUndecodable
	}
	dict["F"] = pdf.Name("Fl")

	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if err := src.WriteData(zw); err != nil {
		return content.Operator{}, err
	}
	if err := zw.Close(); err != nil {
		return content.Operator{}, err
	}
	return content.Operator{
		Name: content.OpInlineImage,
		Args: []pdf.Object{dict, pdf.String(buf.Bytes())},
	}, nil
}
