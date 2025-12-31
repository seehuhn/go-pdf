package converter

import (
	"bytes"
	goimage "image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"math"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
	"golang.org/x/image/vector"
	"seehuhn.de/go/geom/matrix"
	geompath "seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	pdfcolor "seehuhn.de/go/pdf/graphics/color"
	pdfimage "seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"
)

type cachedFont struct {
	font *sfnt.Font
	data []byte // Raw font data
}

// ImageRenderer implements reader.Reader callbacks to render PDF content to an image.
type ImageRenderer struct {
	Converter   *Converter
	Image       *goimage.RGBA
	Raster      *vector.Rasterizer
	Width       int
	Height      int
	DPI         float64
	OffsetX     float64
	OffsetY     float64
	fontCache   map[font.Instance]*cachedFont
	currentPath []pathInfo
}

type pathInfo struct {
	op   int // 0:Move, 1:Line, 2:Quad, 3:Cube, 4:Close
	args []float64
}

// NewImageRenderer creates a new ImageRenderer.
func NewImageRenderer(c *Converter, width, height int, dpi float64, offsetX, offsetY float64) *ImageRenderer {
	img := goimage.NewRGBA(goimage.Rect(0, 0, width, height))
	// Initialize with white background
	draw.Draw(img, img.Bounds(), goimage.White, goimage.Point{}, draw.Src)

	return &ImageRenderer{
		Converter: c,
		Image:     img,
		Raster:    vector.NewRasterizer(width, height),
		Width:     width,
		Height:    height,
		DPI:       dpi,
		OffsetX:   offsetX,
		OffsetY:   offsetY,
		fontCache: make(map[font.Instance]*cachedFont),
	}
}

func (r *ImageRenderer) Setup() {
	c := r.Converter
	c.Reader.PathMoveTo = r.PathMoveTo
	c.Reader.PathLineTo = r.PathLineTo
	c.Reader.PathCurveTo = r.PathCurveTo
	c.Reader.PathRectangle = r.PathRectangle
	c.Reader.PathClose = r.PathClose
	c.Reader.PathPaint = r.PathPaint
	c.Reader.DrawXObject = r.DrawXObject
	c.Reader.Character = r.Character
}

func (r *ImageRenderer) Character(cid cid.CID, text string, width float64) error {
	f := r.Converter.Reader.TextFont
	if f == nil {
		return nil
	}

	cached, err := r.getFont(f)
	if err != nil || cached == nil {
		// Fallback to placeholder if font cannot be loaded
		x, y := r.Converter.Reader.GetTextPositionDevice()
		fs := r.Converter.Reader.TextFontSize
		r.PathRectangle(x, y, width, fs)
		r.fill(true)
		r.Raster.Reset(r.Width, r.Height)
		return nil
	}

	tm := r.Converter.Reader.TextMatrix
	ctm := r.Converter.Reader.CTM
	fs := r.Converter.Reader.TextFontSize
	hs := r.Converter.Reader.TextHorizontalScaling

	// Scale for glyph units
	upem := float64(cached.font.UnitsPerEm)
	scale := fs / upem
	mGlyph := matrix.Matrix{scale * hs, 0, 0, scale, 0, 0}.Mul(tm).Mul(ctm)

	// Use unicode from info.Text if available to find GID
	var gid glyph.ID
	runes := []rune(text)

	// First try to use the CMap if we have unicode text
	if len(runes) > 0 {
		// Attempt to get the best CMap subtable
		subtable, errCmap := cached.font.CMapTable.GetBest()
		if errCmap == nil && subtable != nil {
			g := subtable.Lookup(runes[0])
			if g != 0 {
				gid = g
			} else {
				// Fallback: assume CID is GID
				gid = glyph.ID(cid)
			}
		} else {
			gid = glyph.ID(cid)
		}
	} else {
		gid = glyph.ID(cid)
	}

	if cached.font.Outlines == nil {
		return nil
	}

	// Extract path from sfnt.Font
	p := cached.font.Outlines.Path(gid)
	mAff3 := f64.Aff3{mGlyph[0], mGlyph[2], mGlyph[4], mGlyph[1], mGlyph[3], mGlyph[5]}

	// Iterate over path commands
	for cmd, points := range p {
		switch cmd {
		case geompath.CmdMoveTo:
			x, y := r.transformVec(points[0], mAff3)
			r.Raster.MoveTo(float32(x), float32(y))
		case geompath.CmdLineTo:
			x, y := r.transformVec(points[0], mAff3)
			r.Raster.LineTo(float32(x), float32(y))
		case geompath.CmdQuadTo:
			x1, y1 := r.transformVec(points[0], mAff3)
			x2, y2 := r.transformVec(points[1], mAff3)
			r.Raster.QuadTo(float32(x1), float32(y1), float32(x2), float32(y2))
		case geompath.CmdCubeTo:
			x1, y1 := r.transformVec(points[0], mAff3)
			x2, y2 := r.transformVec(points[1], mAff3)
			x3, y3 := r.transformVec(points[2], mAff3)
			r.Raster.CubeTo(float32(x1), float32(y1), float32(x2), float32(y2), float32(x3), float32(y3))
		case geompath.CmdClose:
			r.Raster.ClosePath()
		}
	}

	r.fill(true)
	r.Raster.Reset(r.Width, r.Height)
	return nil
}

func (r *ImageRenderer) transformVec(v vec.Vec2, m f64.Aff3) (float64, float64) {
	xf := v.X
	yf := v.Y

	// Apply matrix
	tx := m[0]*xf + m[1]*yf + m[2]
	ty := m[3]*xf + m[4]*yf + m[5]

	// Map to device pixels
	dpiScale := r.DPI / 72.0
	dx := (tx - r.OffsetX) * dpiScale
	dy := float64(r.Height) - ((ty - r.OffsetY) * dpiScale)
	return dx, dy
}

func (r *ImageRenderer) transform(p vec.Vec2, m f64.Aff3) (float64, float64) {
	return r.transformVec(p, m)
}

func (r *ImageRenderer) getFont(f font.Instance) (*cachedFont, error) {
	if cached, ok := r.fontCache[f]; ok {
		return cached, nil
	}

	info := f.FontInfo()
	if info == nil {
		return nil, nil
	}

	var stream *glyphdata.Stream
	switch v := info.(type) {
	case *dict.FontInfoSimple:
		stream = v.FontFile
	case *dict.FontInfoGlyfEmbedded:
		stream = v.FontFile
	case *dict.FontInfoCID:
		stream = v.FontFile
	default:
		return nil, nil
	}

	if stream == nil {
		return nil, nil
	}

	var buf bytes.Buffer
	err := stream.WriteTo(&buf, nil)
	if err != nil {
		return nil, err
	}
	data := buf.Bytes()

	sf, err := sfnt.Read(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	cached := &cachedFont{
		font: sf,
		data: data,
	}
	r.fontCache[f] = cached
	return cached, nil
}

func (r *ImageRenderer) PathMoveTo(x, y float64) error {
	r.currentPath = append(r.currentPath, pathInfo{0, []float64{x, y}})
	return nil
}

func (r *ImageRenderer) PathLineTo(x, y float64) error {
	r.currentPath = append(r.currentPath, pathInfo{1, []float64{x, y}})
	return nil
}

func (r *ImageRenderer) PathCurveTo(x1, y1, x2, y2, x3, y3 float64) error {
	r.currentPath = append(r.currentPath, pathInfo{2, []float64{x1, y1, x2, y2, x3, y3}})
	return nil
}

func (r *ImageRenderer) PathRectangle(x, y, w, h float64) error {
	r.PathMoveTo(x, y)
	r.PathLineTo(x+w, y)
	r.PathLineTo(x+w, y+h)
	r.PathLineTo(x, y+h)
	r.PathClose()
	return nil
}

func (r *ImageRenderer) PathClose() error {
	r.currentPath = append(r.currentPath, pathInfo{3, nil})
	return nil
}

func (r *ImageRenderer) PathPaint(op string) error {
	switch op {
	case "f", "F", "f*":
		nonzero := (op != "f*")
		r.drawPath()
		r.fill(nonzero)
	case "S", "s":
		if op == "s" {
			r.PathClose()
		}
		r.stroke()
	case "B", "B*", "b", "b*":
		nonzero := (op != "B*" && op != "b*")
		if op == "b" || op == "b*" {
			r.PathClose()
		}
		r.drawPath()
		r.fill(nonzero)
		r.stroke()
	case "n":
		// End path without painting
	}
	r.currentPath = nil // Clear path after painting
	r.Raster.Reset(r.Width, r.Height)
	return nil
}

func (r *ImageRenderer) DrawXObject(name string) error {
	obj, ok := r.Converter.Reader.Resources.XObject[pdf.Name(name)]
	if !ok {
		return nil
	}

	ext := pdf.NewExtractor(r.Converter.Reader.R)
	imgDict, err := pdfimage.ExtractDict(ext, obj)
	if err != nil {
		// Possibly a form XObject, not implemented yet
		return nil
	}

	// 1. Get image data
	var buf bytes.Buffer
	err = imgDict.WriteData(&buf)
	if err != nil {
		return err
	}
	data := buf.Bytes()

	// 2. Map PDF image to Go image (simplified)
	var srcImg goimage.Image

	// Try image.Decode first (handles filters like DCTDecode if we passed the data through)
	if decoded, _, err := goimage.Decode(bytes.NewReader(data)); err == nil {
		srcImg = decoded
	} else {
		// Fallback for raw data
		switch imgDict.ColorSpace.Family() {
		case pdfcolor.FamilyDeviceGray:
			gray := goimage.NewGray(goimage.Rect(0, 0, imgDict.Width, imgDict.Height))
			if len(data) >= len(gray.Pix) {
				copy(gray.Pix, data)
			}
			srcImg = gray
		case pdfcolor.FamilyDeviceRGB:
			rgba := goimage.NewRGBA(goimage.Rect(0, 0, imgDict.Width, imgDict.Height))
			// PDF RGB is usually R, G, B triplets without alpha
			if len(data) >= imgDict.Width*imgDict.Height*3 {
				for i := 0; i < imgDict.Width*imgDict.Height; i++ {
					rgba.Pix[i*4+0] = data[i*3+0]
					rgba.Pix[i*4+1] = data[i*3+1]
					rgba.Pix[i*4+2] = data[i*3+2]
					rgba.Pix[i*4+3] = 255
				}
			}
			srcImg = rgba
		}
	}

	if srcImg == nil {
		return nil
	}

	// 3. Draw image with current CTM
	// In PDF, images are drawn in a 1x1 unit square at the origin.
	// The CTM maps this square to the page.
	m := r.Converter.Reader.CTM

	// Combined matrix: UnitSquare -> DevicePixels
	// UnitSquare (0,0)-(1,1) -> PagePoints (via CTM) -> DevicePixels (via DPI)

	r.drawTransformedImage(srcImg, m)

	return nil
}

func (r *ImageRenderer) drawTransformedImage(src goimage.Image, m matrix.Matrix) {
	// m maps the unit square (0,0)-(1,1) to user space points.
	// In PDF, the image maps such that top-left of image is (0,1) in unit square.

	b := src.Bounds()
	W := float64(b.Dx())
	H := float64(b.Dy())
	ds := r.DPI / 72.0

	// Refined matrix calculation to map image pixels (0,0)-(W,H) to device pixels.
	// We account for:
	// 1. Image coords to unit square (Y-flip)
	// 2. Unit square to user space (CTM)
	// 3. User space (points) to page pixels (dpiScale)
	// 4. Page pixels to device pixels (target Y-flip)

	// dr maps (sx, sy) to (dx, dy):
	// dx = sx * (a*ds/W) + sy * (-c*ds/H) + (c + tx) * ds
	// dy = sx * (-b*ds/W) + sy * (d*ds/H) + (Height - (d + ty) * ds)

	a, b2, c, d, tx, ty := m[0], m[1], m[2], m[3], m[4], m[5]

	// Determine mapping from Image (sx, sy) to Unit Square (u, v)
	// We want Image(0,0) (Top-Left) to map to visual Top-Left in PDF space.
	// PDF Visual Top is determined by CTM Y-direction 'd'.
	// If d > 0: Visual Top is v=1. So sy=0 -> v=1 (v = 1 - sy/H).
	// If d < 0: Visual Top is v=0. So sy=0 -> v=0 (v = sy/H).
	// Similarly for X 'a'.
	// If a > 0: Visual Left is u=0. So sx=0 -> u=0 (u = sx/W).
	// If a < 0: Visual Left is u=1. So sx=0 -> u=1 (u = 1 - sx/W).

	var u_coeff, u_off float64
	if a >= 0 {
		u_coeff = 1.0 / W
		u_off = 0.0
	} else {
		u_coeff = -1.0 / W
		u_off = 1.0
	}

	var v_coeff, v_off float64
	if d >= 0 {
		v_coeff = -1.0 / H
		v_off = 1.0
	} else {
		v_coeff = 1.0 / H
		v_off = 0.0
	}

	// Calculate affine matrix 'dr' elements based on:
	// x_dev = scale * ( CTM[0]*(u_coeff*sx + u_off) + CTM[2]*(v_coeff*sy + v_off) + CTM[4] - OffsetX )
	// y_dev = Height - scale * ( CTM[1]*(u_coeff*sx + u_off) + CTM[3]*(v_coeff*sy + v_off) + CTM[5] - OffsetY )

	// dr[0] (sx coeff for x_dev) = scale * a * u_coeff
	// dr[1] (sy coeff for x_dev) = scale * c * v_coeff
	// dr[2] (const coeff for x_dev) = scale * (a*u_off + c*v_off + tx - OffsetX)

	// dr[5] (const coeff for y_dev)
	// If d < 0 (Flipped): y_dev = scale * (terms - OffsetY)
	// If d >= 0 (Standard): y_dev = Height - scale * (terms - OffsetY)

	var dy_const float64
	if d < 0 {
		dy_const = ds * (b2*u_off + d*v_off + ty - r.OffsetY)
	} else {
		dy_const = float64(r.Height) - ds*(b2*u_off+d*v_off+ty-r.OffsetY)
	}

	dr := f64.Aff3{
		ds * a * u_coeff,
		ds * c * v_coeff,
		ds * (a*u_off + c*v_off + tx - r.OffsetX),
		-ds * b2 * u_coeff, // Note: Aff3 Y coefficients might need adjustment for flip?
		// Actually, if d < 0, calculating y_dev = dy_const + ... lines up.
		// Let's stick to the mapping derivation.
		// xdraw Transform applies:
		// dstX = dr[0]*srcX + dr[1]*srcY + dr[2]
		// dstY = dr[3]*srcX + dr[4]*srcY + dr[5]
		//
		// My derivation:
		// y_dev = ... - scale * ( CTM[1]... + CTM[3]... )
		// If d < 0, we want positive contribution?
		// If d < 0, we are in Top-Down. y_dev = scale * y_points.
		// y_points = ... + d*(v_coeff*sy + v_off)
		// So y_dev = scale * ( ... + d*v_coeff*sy ... )
		// Coeff for sy is scale * d * v_coeff.
		// Wait, my prev code had '-ds * d * v_coeff'.
		// If d < 0, let's remove the negative sign if we are not flipping.

		-ds * d * v_coeff,
		dy_const,
	}

	xdraw.BiLinear.Transform(r.Image, dr, src, b, draw.Over, nil)
}

func (r *ImageRenderer) deviceCoords(x, y float64) (float64, float64) {
	// Map PDF coords to device coords.
	// Check if CTM defines a flipped coordinate system (d < 0)
	m := r.Converter.Reader.CTM

	// Apply CTM to get "Page Points"
	x, y = m[0]*x+m[2]*y+m[4], m[1]*x+m[3]*y+m[5]

	// Map to pixels based on DPI.
	scale := r.DPI / 72.0
	dx := (x - r.OffsetX) * scale
	var dy float64

	if m[3] < 0 {
		// Flipped system (Top-Down): y is already distance from visual top.
		// Don't flip again.
		dy = (y - r.OffsetY) * scale
	} else {
		// Standard system (Bottom-Up): y is distance from bottom.
		// Flip to get distance from top.
		dy = float64(r.Height) - ((y - r.OffsetY) * scale)
	}

	return dx, dy
}

func (r *ImageRenderer) fill(nonzero bool) {
	if nonzero {
		// Non-zero winding (default for vector.Rasterizer)
	}
	r.Raster.Draw(r.Image, r.Image.Bounds(), goimage.NewUniform(r.toGoColor(r.Converter.Reader.FillColor)), goimage.Point{})
}

func (r *ImageRenderer) drawPath() {
	r.Raster.Reset(r.Width, r.Height)
	for _, p := range r.currentPath {
		switch p.op {
		case 0:
			dx, dy := r.deviceCoords(p.args[0], p.args[1])
			r.Raster.MoveTo(float32(dx), float32(dy))
		case 1:
			dx, dy := r.deviceCoords(p.args[0], p.args[1])
			r.Raster.LineTo(float32(dx), float32(dy))
		case 2:
			dx1, dy1 := r.deviceCoords(p.args[0], p.args[1])
			dx2, dy2 := r.deviceCoords(p.args[2], p.args[3])
			dx3, dy3 := r.deviceCoords(p.args[4], p.args[5])
			r.Raster.CubeTo(float32(dx1), float32(dy1), float32(dx2), float32(dy2), float32(dx3), float32(dy3))
		case 3:
			r.Raster.ClosePath()
		}
	}
}

func (r *ImageRenderer) stroke() {
	r.Raster.Reset(r.Width, r.Height)
	lineWidth := r.Converter.Reader.LineWidth
	if lineWidth <= 0 {
		lineWidth = 1.0
	}

	// Scale line width to device pixels (approximate using CTM [0])
	// A better way would be transforming vectors, but uniform scaling is a decent start.
	ctm := r.Converter.Reader.CTM
	// Estimate scale from CTM (avg of x/y scaling) + DPI
	// CTM is user->points. DPI is points->pixels.
	scale := (f64abs(ctm[0]) + f64abs(ctm[3])) / 2.0 * (r.DPI / 72.0)
	w := lineWidth * scale / 2.0 // Half width

	if w < 0.5 {
		w = 0.5
	} // Minimum visible width

	var startX, startY float64
	var curX, curY float64

	for _, p := range r.currentPath {
		switch p.op {
		case 0: // MoveTo
			dx, dy := r.deviceCoords(p.args[0], p.args[1])
			curX, curY = dx, dy
			startX, startY = dx, dy
		case 1, 3: // LineTo or Close
			var destX, destY float64
			if p.op == 1 {
				destX, destY = r.deviceCoords(p.args[0], p.args[1])
			} else {
				destX, destY = startX, startY
			}

			// Vector from cur to dest
			vx, vy := destX-curX, destY-curY
			vl := math.Sqrt(vx*vx + vy*vy)
			if vl > 0 {
				// Normal vector
				nx, ny := -vy/vl, vx/vl

				// 4 points of the segment rect
				x1, y1 := curX+nx*w, curY+ny*w
				x2, y2 := destX+nx*w, destY+ny*w
				x3, y3 := destX-nx*w, destY-ny*w
				x4, y4 := curX-nx*w, curY-ny*w

				r.Raster.MoveTo(float32(x1), float32(y1))
				r.Raster.LineTo(float32(x2), float32(y2))
				r.Raster.LineTo(float32(x3), float32(y3))
				r.Raster.LineTo(float32(x4), float32(y4))
				r.Raster.ClosePath()
			}
			curX, curY = destX, destY
		case 2: // CubeTo - treat as line for now (flattening is complex)
			// Draw line from current to end
			dx, dy := r.deviceCoords(p.args[4], p.args[5])
			// ... logic for stroke line cur->dest ...
			// Repetitive logic, simplified:
			destX, destY := dx, dy
			vx, vy := destX-curX, destY-curY
			vl := math.Sqrt(vx*vx + vy*vy)
			if vl > 0 {
				nx, ny := -vy/vl, vx/vl
				r.Raster.MoveTo(float32(curX+nx*w), float32(curY+ny*w))
				r.Raster.LineTo(float32(destX+nx*w), float32(destY+ny*w))
				r.Raster.LineTo(float32(destX-nx*w), float32(destY-ny*w))
				r.Raster.LineTo(float32(curX-nx*w), float32(curY-ny*w))
				r.Raster.ClosePath()
			}
			curX, curY = dx, dy
		}
	}

	r.Raster.Draw(r.Image, r.Image.Bounds(), goimage.NewUniform(r.toGoColor(r.Converter.Reader.StrokeColor)), goimage.Point{})
}

func f64abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func (r *ImageRenderer) toGoColor(c pdfcolor.Color) color.Color {
	if c == nil {
		return color.Black
	}

	family := c.ColorSpace().Family()
	vals, _, _ := pdfcolor.Operator(c)

	switch family {
	case pdfcolor.FamilyDeviceGray, pdfcolor.FamilyCalGray:
		g := uint8(clamp(vals[0]) * 255)
		return color.Gray{Y: g}
	case pdfcolor.FamilyDeviceRGB, pdfcolor.FamilyCalRGB:
		return color.RGBA{
			R: uint8(clamp(vals[0]) * 255),
			G: uint8(clamp(vals[1]) * 255),
			B: uint8(clamp(vals[2]) * 255),
			A: 255,
		}
	case pdfcolor.FamilyDeviceCMYK:
		return color.CMYK{
			C: uint8(clamp(vals[0]) * 255),
			M: uint8(clamp(vals[1]) * 255),
			Y: uint8(clamp(vals[2]) * 255),
			K: uint8(clamp(vals[3]) * 255),
		}
	}
	return color.Black
}

func clamp(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
