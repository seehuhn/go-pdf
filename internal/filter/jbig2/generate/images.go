// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

// bilevel is a 1-bit image, packed MSB-first with rows padded to whole
// bytes.  A set bit is black, as in both PBM and JBIG2.
type bilevel struct {
	w, h   int
	stride int
	pix    []byte
}

func newBilevel(w, h int) *bilevel {
	stride := (w + 7) / 8
	return &bilevel{w: w, h: h, stride: stride, pix: make([]byte, stride*h)}
}

func (b *bilevel) get(x, y int) bool {
	return b.pix[y*b.stride+x/8]&(0x80>>(x%8)) != 0
}

func (b *bilevel) set(x, y int) {
	b.pix[y*b.stride+x/8] |= 0x80 >> (x % 8)
}

// blank reports whether b has no black pixels.
func (b *bilevel) blank() bool {
	for y := range b.h {
		for x := range b.w {
			if b.get(x, y) {
				return false
			}
		}
	}
	return true
}

func (b *bilevel) equal(c *bilevel) bool {
	if b.w != c.w || b.h != c.h {
		return false
	}
	for y := range b.h {
		for x := range b.w {
			if b.get(x, y) != c.get(x, y) {
				return false
			}
		}
	}
	return true
}

// crop returns the w x h rectangle of b with top left corner (x0, y0).
// The rectangle must lie inside b.
func (b *bilevel) crop(x0, y0, w, h int) *bilevel {
	if x0 < 0 || y0 < 0 || w <= 0 || h <= 0 || x0+w > b.w || y0+h > b.h {
		panic(fmt.Sprintf("crop %dx%d+%d+%d outside the %dx%d image", w, h, x0, y0, b.w, b.h))
	}
	c := newBilevel(w, h)
	for y := range h {
		for x := range w {
			if b.get(x0+x, y0+y) {
				c.set(x, y)
			}
		}
	}
	return c
}

// readPBM reads a binary ("P4") PBM file.
func readPBM(fileName string) (*bilevel, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(bytes.NewReader(data))

	// header tokens, with "#" comments up to the end of the line
	var tokens []string
	var tok []byte
	for len(tokens) < 3 {
		c, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("%s: truncated header", fileName)
		}
		switch {
		case c == '#':
			if _, err := r.ReadString('\n'); err != nil {
				return nil, fmt.Errorf("%s: truncated header", fileName)
			}
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if len(tok) > 0 {
				tokens = append(tokens, string(tok))
				tok = tok[:0]
			}
		default:
			tok = append(tok, c)
		}
	}
	if tokens[0] != "P4" {
		return nil, fmt.Errorf("%s: not a binary PBM file", fileName)
	}
	w, err1 := strconv.Atoi(tokens[1])
	h, err2 := strconv.Atoi(tokens[2])
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return nil, fmt.Errorf("%s: invalid image size", fileName)
	}
	b := newBilevel(w, h)
	if _, err := io.ReadFull(r, b.pix); err != nil {
		return nil, fmt.Errorf("%s: %w", fileName, err)
	}
	return b, nil
}

func writePBM(fileName string, b *bilevel) error {
	buf := fmt.Appendf(nil, "P4\n%d %d\n", b.w, b.h)
	buf = append(buf, b.pix...)
	return os.WriteFile(fileName, buf, 0o644)
}

// writeBMP writes b as an uncompressed 1-bit BMP file, in the layout
// used by the test data: bottom-up rows, palette entry 0 black and entry 1
// white.
func writeBMP(fileName string, b *bilevel) error {
	return writeBMPPalette(fileName, b, false)
}

// writeBMPPalette writes b as an uncompressed 1-bit BMP file with
// bottom-up rows.  If oneIsBlack is set, palette entry 0 is white and
// entry 1 is black, as in the sample images of the T.88 software;
// otherwise the palette is the other way round.
func writeBMPPalette(fileName string, b *bilevel, oneIsBlack bool) error {
	rowBytes := (b.w + 31) / 32 * 4
	const headerSize = 14 + 40 + 8
	imageSize := rowBytes * b.h

	buf := make([]byte, headerSize, headerSize+imageSize)
	copy(buf, "BM")
	binary.LittleEndian.PutUint32(buf[2:], uint32(headerSize+imageSize))
	binary.LittleEndian.PutUint32(buf[10:], headerSize)
	binary.LittleEndian.PutUint32(buf[14:], 40)
	binary.LittleEndian.PutUint32(buf[18:], uint32(b.w))
	binary.LittleEndian.PutUint32(buf[22:], uint32(b.h))
	binary.LittleEndian.PutUint16(buf[26:], 1) // planes
	binary.LittleEndian.PutUint16(buf[28:], 1) // bits per pixel
	binary.LittleEndian.PutUint32(buf[34:], uint32(imageSize))
	binary.LittleEndian.PutUint32(buf[38:], 3780) // 96 dpi
	binary.LittleEndian.PutUint32(buf[42:], 3780)
	binary.LittleEndian.PutUint32(buf[46:], 2) // colours used
	binary.LittleEndian.PutUint32(buf[50:], 2) // important colours
	if oneIsBlack {
		copy(buf[54:], []byte{0xFF, 0xFF, 0xFF, 0x00})
	} else {
		copy(buf[58:], []byte{0xFF, 0xFF, 0xFF, 0x00})
	}

	row := make([]byte, rowBytes)
	for y := b.h - 1; y >= 0; y-- {
		clear(row)
		src := b.pix[y*b.stride : (y+1)*b.stride]
		for i, v := range src {
			if oneIsBlack {
				row[i] = v
			} else {
				row[i] = ^v
			}
		}
		if b.w%8 != 0 {
			// padding bits after the last pixel stay zero
			row[b.stride-1] &= byte(0xFF << (8 - b.w%8))
		}
		buf = append(buf, row...)
	}
	return os.WriteFile(fileName, buf, 0o644)
}

// readBMP reads an uncompressed 1-bit BMP file, using the palette to
// decide which pixel value is black.
func readBMP(fileName string) (*bilevel, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	le := binary.LittleEndian
	if len(data) < 62 || string(data[:2]) != "BM" {
		return nil, fmt.Errorf("%s: not a BMP file", fileName)
	}
	offset := int(le.Uint32(data[10:]))
	infoSize := int(le.Uint32(data[14:]))
	w := int(int32(le.Uint32(data[18:])))
	h := int(int32(le.Uint32(data[22:])))
	bpp := le.Uint16(data[28:])
	compression := le.Uint32(data[30:])
	if bpp != 1 || compression != 0 || w <= 0 || h == 0 {
		return nil, fmt.Errorf("%s: unsupported BMP format", fileName)
	}
	topDown := h < 0
	if topDown {
		h = -h
	}
	pal := 14 + infoSize
	if pal+8 > len(data) {
		return nil, fmt.Errorf("%s: truncated BMP palette", fileName)
	}
	lum := func(i int) int {
		p := data[pal+4*i:]
		return int(p[0]) + int(p[1]) + int(p[2])
	}
	oneIsBlack := lum(1) < lum(0)

	rowBytes := (w + 31) / 32 * 4
	if offset+rowBytes*h > len(data) {
		return nil, fmt.Errorf("%s: truncated BMP data", fileName)
	}
	b := newBilevel(w, h)
	for y := range h {
		srcY := h - 1 - y
		if topDown {
			srcY = y
		}
		row := data[offset+srcY*rowBytes:]
		for x := range w {
			bit := row[x/8]&(0x80>>(x%8)) != 0
			if bit == oneIsBlack {
				b.set(x, y)
			}
		}
	}
	return b, nil
}

// readG4Strip returns the compressed data of a single-strip,
// CCITT Group 4 compressed, min-is-white, MSB-first TIFF file, as
// written by libtiff's ppm2tiff.
func readG4Strip(fileName string) ([]byte, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	if len(data) < 8 || string(data[:4]) != "II*\x00" {
		return nil, errors.New("not a little-endian classic TIFF file")
	}
	le := binary.LittleEndian
	ifd := int(le.Uint32(data[4:]))
	if ifd+2 > len(data) {
		return nil, errors.New("truncated TIFF file")
	}
	n := int(le.Uint16(data[ifd:]))
	tags := make(map[uint16]uint32)
	counts := make(map[uint16]uint32)
	for i := range n {
		e := ifd + 2 + 12*i
		if e+12 > len(data) {
			return nil, errors.New("truncated TIFF directory")
		}
		tag := le.Uint16(data[e:])
		typ := le.Uint16(data[e+2:])
		counts[tag] = le.Uint32(data[e+4:])
		switch typ {
		case 3: // SHORT
			tags[tag] = uint32(le.Uint16(data[e+8:]))
		case 4: // LONG
			tags[tag] = le.Uint32(data[e+8:])
		}
	}
	const (
		tagCompression  = 259
		tagPhotometric  = 262
		tagFillOrder    = 266
		tagStripOffsets = 273
		tagStripCounts  = 279
	)
	if tags[tagCompression] != 4 {
		return nil, errors.New("TIFF file is not Group 4 compressed")
	}
	if tags[tagPhotometric] != 0 {
		return nil, errors.New("TIFF file is not min-is-white")
	}
	if fo, ok := tags[tagFillOrder]; ok && fo != 1 {
		return nil, errors.New("TIFF file is not MSB-first")
	}
	if counts[tagStripOffsets] != 1 || counts[tagStripCounts] != 1 {
		return nil, errors.New("TIFF file has more than one strip")
	}
	off, size := int(tags[tagStripOffsets]), int(tags[tagStripCounts])
	if off+size > len(data) {
		return nil, errors.New("truncated TIFF strip")
	}
	return data[off : off+size], nil
}
