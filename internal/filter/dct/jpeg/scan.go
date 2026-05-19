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

// This file is based on code from Go's "image/jpeg" package (and then
// modified).  Use of the original source code is governed by a BSD-style
// license, which is reproduced here:
//
//     Copyright 2012 The Go Authors.
//
//     Redistribution and use in source and binary forms, with or without
//     modification, are permitted provided that the following conditions are
//     met:
//
//        * Redistributions of source code must retain the above copyright
//     notice, this list of conditions and the following disclaimer.
//        * Redistributions in binary form must reproduce the above
//     copyright notice, this list of conditions and the following disclaimer
//     in the documentation and/or other materials provided with the
//     distribution.
//        * Neither the name of Google LLC nor the names of its
//     contributors may be used to endorse or promote products derived from
//     this software without specific prior written permission.
//
//     THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
//     "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
//     LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
//     A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
//     OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
//     SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
//     LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
//     DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
//     THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
//     (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
//     OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package jpeg

// bytesPerProgBlock is the size in bytes of one entry in the
// per-component progressive coefficient buffer ([blockSize]int32).
const bytesPerProgBlock = blockSize * 4

// makeImg allocates the destination pixel planes.  When d.streaming is
// true only one MCU stripe is allocated (the storeMyy override below
// reduces the buffer height); otherwise full-image planes are
// allocated for the multi-scan baseline fallback.  All planes are
// dimensioned in MCU-aligned units, so 8*comp[i].h*mxx wide and
// 8*comp[i].v*storeMyy tall for component i.
func (d *decoder) makeImg(mxx, myy int) {
	storeMyy := myy
	if d.streaming {
		storeMyy = 1
	}

	h0 := d.comp[0].h
	v0 := d.comp[0].v
	d.yStride = 8 * h0 * mxx
	d.y = make([]byte, d.yStride*8*v0*storeMyy)
	if d.nComp == 1 {
		// d.hRatio / d.vRatio / d.cStride remain zero
		return
	}

	d.hRatio = h0 / d.comp[1].h
	d.vRatio = v0 / d.comp[1].v
	d.cStride = 8 * d.comp[1].h * mxx
	d.cb = make([]byte, d.cStride*8*d.comp[1].v*storeMyy)
	d.cr = make([]byte, d.cStride*8*d.comp[1].v*storeMyy)

	if d.nComp == 4 {
		d.blackStride = 8 * d.comp[3].h * mxx
		d.blackPix = make([]byte, d.blackStride*8*d.comp[3].v*storeMyy)
	}
}

// Specified in section B.2.3.
func (d *decoder) processSOS(n int) error {
	if d.nComp == 0 {
		return FormatError("missing SOF marker")
	}
	if n < 6 || 4+2*d.nComp < n || n%2 != 0 {
		return FormatError("SOS has wrong length")
	}
	if err := d.readFull(d.tmp[:n]); err != nil {
		return err
	}
	nComp := int(d.tmp[0])
	if n != 4+2*nComp {
		return FormatError("SOS length inconsistent with number of components")
	}
	var scan [maxComponents]struct {
		compIndex uint8
		td        uint8 // DC table selector.
		ta        uint8 // AC table selector.
	}
	totalHV := 0
	for i := range nComp {
		cs := d.tmp[1+2*i] // Component selector.
		compIndex := -1
		for j, comp := range d.comp[:d.nComp] {
			if cs == comp.c {
				compIndex = j
			}
		}
		if compIndex < 0 {
			return FormatError("unknown component selector")
		}
		scan[i].compIndex = uint8(compIndex)
		// Section B.2.3 states that "the value of Cs_j shall be different from
		// the values of Cs_1 through Cs_(j-1)". Since we have previously
		// verified that a frame's component identifiers (C_i values in section
		// B.2.2) are unique, it suffices to check that the implicit indexes
		// into d.comp are unique.
		for j := range i {
			if scan[i].compIndex == scan[j].compIndex {
				return FormatError("repeated component selector")
			}
		}
		totalHV += d.comp[compIndex].h * d.comp[compIndex].v

		// The baseline t <= 1 restriction is specified in table B.3.
		scan[i].td = d.tmp[2+2*i] >> 4
		if t := scan[i].td; t > maxTh || (d.baseline && t > 1) {
			return FormatError("bad Td value")
		}
		scan[i].ta = d.tmp[2+2*i] & 0x0f
		if t := scan[i].ta; t > maxTh || (d.baseline && t > 1) {
			return FormatError("bad Ta value")
		}
	}
	// Section B.2.3 states that if there is more than one component then the
	// total H*V values in a scan must be <= 10.
	if d.nComp > 1 && totalHV > 10 {
		return FormatError("total sampling factors too large")
	}

	// zigStart and zigEnd are the spectral selection bounds.
	// ah and al are the successive approximation high and low values.
	// The spec calls these values Ss, Se, Ah and Al.
	//
	// For progressive JPEGs, these are the two more-or-less independent
	// aspects of progression. Spectral selection progression is when not
	// all of a block's 64 DCT coefficients are transmitted in one pass.
	// For example, three passes could transmit coefficient 0 (the DC
	// component), coefficients 1-5, and coefficients 6-63, in zig-zag
	// order. Successive approximation is when not all of the bits of a
	// band of coefficients are transmitted in one pass. For example,
	// three passes could transmit the 6 most significant bits, followed
	// by the second-least significant bit, followed by the least
	// significant bit.
	//
	// For sequential JPEGs, these parameters are hard-coded to 0/63/0/0, as
	// per table B.3.
	zigStart, zigEnd, ah, al := int32(0), int32(blockSize-1), uint32(0), uint32(0)
	if d.progressive {
		zigStart = int32(d.tmp[1+2*nComp])
		zigEnd = int32(d.tmp[2+2*nComp])
		ah = uint32(d.tmp[3+2*nComp] >> 4)
		al = uint32(d.tmp[3+2*nComp] & 0x0f)
		if (zigStart == 0 && zigEnd != 0) || zigStart > zigEnd || blockSize <= zigEnd {
			return FormatError("bad spectral selection bounds")
		}
		if zigStart != 0 && nComp != 1 {
			return FormatError("progressive AC coefficients for more than one component")
		}
		if ah != 0 && ah != al+1 {
			return FormatError("bad successive approximation values")
		}
	}

	// mxx and myy are the number of MCUs (Minimum Coded Units) in the image.
	h0, v0 := d.comp[0].h, d.comp[0].v // The h and v values from the Y components.
	mxx := (d.width + 8*h0 - 1) / (8 * h0)
	myy := (d.height + 8*v0 - 1) / (8 * v0)
	if d.y == nil {
		// streaming uses stripe-sized output buffers; for baseline the
		// stripe is filled and emitted during the scan, so it requires a
		// single interleaved scan (the SOS lists every component) — a
		// second SOS would try to refine MCU rows that have already been
		// emitted.  Progressive scans fill progCoeffs and emit during
		// reconstructProgressiveImage; they always tolerate stripes.
		// Multi-scan baseline (the rare non-interleaved encoding) falls
		// through to full-buffer mode and emits at the end of decode.
		if d.progressive || nComp == d.nComp {
			d.streaming = true
		}
		d.makeImg(mxx, myy)
		// bind the per-stripe emission function: stable from here on
		// since d.nComp, isRGB(), and useYCCK() are all set before SOS
		switch d.nComp {
		case 1:
			d.emit = d.emitGray
		case 3:
			if d.isRGB() {
				d.emit = d.emitYCbCrAsRGB
			} else {
				d.emit = d.emitYCbCr
			}
		default: // nComp == 4
			if d.useYCCK() {
				d.emit = d.emitYCCK
			} else {
				d.emit = d.emitRawCMYK
			}
		}
	} else if d.streaming && !d.progressive {
		// a second SOS appeared after the first baseline scan was
		// streamed; we cannot un-emit those rows, so reject the file
		// rather than silently producing wrong output
		return FormatError("multi-scan baseline not supported in streaming mode")
	}
	if d.progressive {
		// charge progressive coefficient buffers to the per-stream
		// budget; the per-image pixel/byte caps at SOF time admit a
		// ~4× amplification into the coefficient buffer
		for i := range nComp {
			compIndex := scan[i].compIndex
			if d.progCoeffs[compIndex] == nil {
				nBlocks := int64(mxx) * int64(myy) * int64(d.comp[compIndex].h) * int64(d.comp[compIndex].v)
				if err := d.budget.Charge(int(nBlocks) * bytesPerProgBlock); err != nil {
					return FormatError("progressive coefficient buffer exceeds budget")
				}
				d.progCoeffs[compIndex] = make([]block, nBlocks)
			}
		}
	}

	d.bits = bits{}
	mcu, expectedRST := 0, uint8(rst0Marker)
	var (
		// b is the decoded coefficients, in natural (not zig-zag) order.
		b  block
		dc [maxComponents]int32
		// bx and by are the location of the current block, in units of 8x8
		// blocks: the third block in the first row has (bx, by) = (2, 0).
		bx, by     int
		blockCount int
	)
	for my := range myy {
		if d.streaming {
			// the stripe buffer addresses block rows in [0, v_i); update
			// the per-component stripe origin so reconstructBlock can
			// translate global by to stripe-local by
			for i := range d.nComp {
				d.stripeYStart[i] = d.comp[i].v * my
			}
		}
		for mx := range mxx {
			for i := range nComp {
				compIndex := scan[i].compIndex
				hi := d.comp[compIndex].h
				vi := d.comp[compIndex].v
				for j := 0; j < hi*vi; j++ {
					// The blocks are traversed one MCU at a time. For 4:2:0 chroma
					// subsampling, there are four Y 8x8 blocks in every 16x16 MCU.
					//
					// For a sequential 32x16 pixel image, the Y blocks visiting order is:
					//	0 1 4 5
					//	2 3 6 7
					//
					// For progressive images, the interleaved scans (those with nComp > 1)
					// are traversed as above, but non-interleaved scans are traversed left
					// to right, top to bottom:
					//	0 1 2 3
					//	4 5 6 7
					// Only DC scans (zigStart == 0) can be interleaved. AC scans must have
					// only one component.
					//
					// To further complicate matters, for non-interleaved scans, there is no
					// data for any blocks that are inside the image at the MCU level but
					// outside the image at the pixel level. For example, a 24x16 pixel 4:2:0
					// progressive image consists of two 16x16 MCUs. The interleaved scans
					// will process 8 Y blocks:
					//	0 1 4 5
					//	2 3 6 7
					// The non-interleaved scans will process only 6 Y blocks:
					//	0 1 2
					//	3 4 5
					if nComp != 1 {
						bx = hi*mx + j%hi
						by = vi*my + j/hi
					} else {
						q := mxx * hi
						bx = blockCount % q
						by = blockCount / q
						blockCount++
						if bx*8 >= d.width || by*8 >= d.height {
							continue
						}
					}

					// Load the previous partially decoded coefficients, if applicable.
					if d.progressive {
						b = d.progCoeffs[compIndex][by*mxx*hi+bx]
					} else {
						b = block{}
					}

					if ah != 0 {
						if err := d.refine(&b, &d.huff[acTable][scan[i].ta], zigStart, zigEnd, 1<<al); err != nil {
							return err
						}
					} else {
						zig := zigStart
						if zig == 0 {
							zig++
							// Decode the DC coefficient, as specified in section F.2.2.1.
							value, err := d.decodeHuffman(&d.huff[dcTable][scan[i].td])
							if err != nil {
								return err
							}
							if value > 16 {
								return UnsupportedError("excessive DC component")
							}
							dcDelta, err := d.receiveExtend(value)
							if err != nil {
								return err
							}
							dc[compIndex] += dcDelta
							b[0] = dc[compIndex] << al
						}

						if zig <= zigEnd && d.eobRun > 0 {
							d.eobRun--
						} else {
							// Decode the AC coefficients, as specified in section F.2.2.2.
							huff := &d.huff[acTable][scan[i].ta]
							for ; zig <= zigEnd; zig++ {
								value, err := d.decodeHuffman(huff)
								if err != nil {
									return err
								}
								val0 := value >> 4
								val1 := value & 0x0f
								if val1 != 0 {
									zig += int32(val0)
									if zig > zigEnd {
										break
									}
									ac, err := d.receiveExtend(val1)
									if err != nil {
										return err
									}
									b[unzig[zig]] = ac << al
								} else {
									if val0 != 0x0f {
										d.eobRun = uint16(1 << val0)
										if val0 != 0 {
											bits, err := d.decodeBits(int32(val0))
											if err != nil {
												return err
											}
											d.eobRun |= uint16(bits)
										}
										d.eobRun--
										break
									}
									zig += 0x0f
								}
							}
						}
					}

					if d.progressive {
						// Save the coefficients.
						d.progCoeffs[compIndex][by*mxx*hi+bx] = b
						// We could call reconstructBlock here to dequantize and
						// perform the inverse DCT, materialising the partial
						// progressive image (the whole point of progressive
						// encoding), but since DecodeStream does not emit any
						// pixels until every scan has been consumed there is
						// nothing to do with the partial image.  Defer the
						// reconstruction to reconstructProgressiveImage after
						// all SOS markers have been processed.
						continue
					}
					if err := d.reconstructBlock(&b, bx, by, int(compIndex)); err != nil {
						return err
					}
				} // for j
			} // for i
			mcu++
			if d.ri > 0 && mcu%d.ri == 0 && mcu < mxx*myy {
				// For well-formed input, the RST[0-7] restart marker follows
				// immediately. For corrupt input, call findRST to try to
				// resynchronize.
				if err := d.readFull(d.tmp[:2]); err != nil {
					return err
				} else if d.tmp[0] != 0xff || d.tmp[1] != expectedRST {
					if err := d.findRST(expectedRST); err != nil {
						return err
					}
				}
				expectedRST++
				if expectedRST == rst7Marker+1 {
					expectedRST = rst0Marker
				}
				// Reset the Huffman decoder.
				d.bits = bits{}
				// Reset the DC components, as per section F.2.1.3.1.
				dc = [maxComponents]int32{}
				// Reset the progressive decoder state, as per section G.1.2.2.
				d.eobRun = 0
			}
		} // for mx
		if d.streaming && !d.progressive {
			// for baseline, the stripe holds the full pixel data for
			// this MCU row; for progressive we instead emit later from
			// reconstructProgressiveImage
			if err := d.emitStripe(my); err != nil {
				return err
			}
		}
	} // for my

	return nil
}

// refine decodes a successive approximation refinement block, as specified in
// section G.1.2.
func (d *decoder) refine(b *block, h *huffman, zigStart, zigEnd, delta int32) error {
	// Refining a DC component is trivial.
	if zigStart == 0 {
		if zigEnd != 0 {
			panic("unreachable")
		}
		bit, err := d.decodeBit()
		if err != nil {
			return err
		}
		if bit {
			b[0] |= delta
		}
		return nil
	}

	// Refining AC components is more complicated; see sections G.1.2.2 and G.1.2.3.
	zig := zigStart
	if d.eobRun == 0 {
	loop:
		for ; zig <= zigEnd; zig++ {
			z := int32(0)
			value, err := d.decodeHuffman(h)
			if err != nil {
				return err
			}
			val0 := value >> 4
			val1 := value & 0x0f

			switch val1 {
			case 0:
				if val0 != 0x0f {
					d.eobRun = uint16(1 << val0)
					if val0 != 0 {
						bits, err := d.decodeBits(int32(val0))
						if err != nil {
							return err
						}
						d.eobRun |= uint16(bits)
					}
					break loop
				}
			case 1:
				z = delta
				bit, err := d.decodeBit()
				if err != nil {
					return err
				}
				if !bit {
					z = -z
				}
			default:
				return FormatError("unexpected Huffman code")
			}

			zig, err = d.refineNonZeroes(b, zig, zigEnd, int32(val0), delta)
			if err != nil {
				return err
			}
			if zig > zigEnd {
				return FormatError("too many coefficients")
			}
			if z != 0 {
				b[unzig[zig]] = z
			}
		}
	}
	if d.eobRun > 0 {
		d.eobRun--
		if _, err := d.refineNonZeroes(b, zig, zigEnd, -1, delta); err != nil {
			return err
		}
	}
	return nil
}

// refineNonZeroes refines non-zero entries of b in zig-zag order. If nz >= 0,
// the first nz zero entries are skipped over.
func (d *decoder) refineNonZeroes(b *block, zig, zigEnd, nz, delta int32) (int32, error) {
	for ; zig <= zigEnd; zig++ {
		u := unzig[zig]
		if b[u] == 0 {
			if nz == 0 {
				break
			}
			nz--
			continue
		}
		bit, err := d.decodeBit()
		if err != nil {
			return 0, err
		}
		if !bit {
			continue
		}
		if b[u] >= 0 {
			b[u] += delta
		} else {
			b[u] -= delta
		}
	}
	return zig, nil
}

// reconstructProgressiveImage walks the per-component coefficient
// buffers built during the SOS marker loop and IDCT-reconstructs each
// block into the stripe-sized pixel planes, emitting one MCU row at a
// time.  Progressive decodes always use stripe mode (DecodeStream
// always sets streamOut, and the SOS detection enables streaming
// whenever d.progressive is true), so there is no full-buffer fallback
// path here.
func (d *decoder) reconstructProgressiveImage() error {
	h0 := d.comp[0].h
	v0 := d.comp[0].v
	mxx := (d.width + 8*h0 - 1) / (8 * h0)
	myy := (d.height + 8*v0 - 1) / (8 * v0)

	for my := range myy {
		for i := range d.nComp {
			d.stripeYStart[i] = d.comp[i].v * my
		}
		for i := 0; i < d.nComp; i++ {
			if d.progCoeffs[i] == nil {
				continue
			}
			v := 8 * v0 / d.comp[i].v
			h := 8 * h0 / d.comp[i].h
			stride := mxx * d.comp[i].h
			byStart := d.comp[i].v * my
			byEnd := byStart + d.comp[i].v
			for by := byStart; by < byEnd && by*v < d.height; by++ {
				for bx := 0; bx*h < d.width; bx++ {
					if err := d.reconstructBlock(&d.progCoeffs[i][by*stride+bx], bx, by, i); err != nil {
						return err
					}
				}
			}
		}
		if err := d.emitStripe(my); err != nil {
			return err
		}
	}
	return nil
}

// reconstructBlock dequantizes, performs the inverse DCT and stores the block
// to the image.
func (d *decoder) reconstructBlock(b *block, bx, by, compIndex int) error {
	qt := &d.quant[d.comp[compIndex].tq]
	for zig := range blockSize {
		b[unzig[zig]] *= qt[zig]
	}
	idct(b)
	// stripeYStart[i] is v_i*my in stripe mode and zero in full-buffer
	// mode (where the buffer covers all MCU rows), so unconditionally
	// subtracting it gives the correct local block-y in both cases
	localBy := by - d.stripeYStart[compIndex]
	var dst []byte
	var stride int
	switch compIndex {
	case 0:
		dst, stride = d.y[8*(localBy*d.yStride+bx):], d.yStride
	case 1:
		dst, stride = d.cb[8*(localBy*d.cStride+bx):], d.cStride
	case 2:
		dst, stride = d.cr[8*(localBy*d.cStride+bx):], d.cStride
	case 3:
		dst, stride = d.blackPix[8*(localBy*d.blackStride+bx):], d.blackStride
	default:
		return UnsupportedError("too many components")
	}
	// Level shift by +128, clip to [0, 255], and write to dst.
	for y := range 8 {
		y8 := y * 8
		yStride := y * stride
		for x := range 8 {
			c := b[y8+x]
			if c < -128 {
				c = 0
			} else if c > 127 {
				c = 255
			} else {
				c += 128
			}
			dst[yStride+x] = uint8(c)
		}
	}
	return nil
}

// findRST advances past the next RST restart marker that matches expectedRST.
// Other than I/O errors, it is also an error if we encounter an {0xFF, M}
// two-byte marker sequence where M is not 0x00, 0xFF or the expectedRST.
//
// This is similar to libjpeg's jdmarker.c's next_marker function.
// https://github.com/libjpeg-turbo/libjpeg-turbo/blob/2dfe6c0fe9e18671105e94f7cbf044d4a1d157e6/jdmarker.c#L892-L935
//
// Precondition: d.tmp[:2] holds the next two bytes of JPEG-encoded input
// (input in the d.readFull sense).
func (d *decoder) findRST(expectedRST uint8) error {
	for {
		// i is the index such that, at the bottom of the loop, we read 2-i
		// bytes into d.tmp[i:2], maintaining the invariant that d.tmp[:2]
		// holds the next two bytes of JPEG-encoded input. It is either 0 or 1,
		// so that each iteration advances by 1 or 2 bytes (or returns).
		i := 0

		if d.tmp[0] == 0xff {
			if d.tmp[1] == expectedRST {
				return nil
			} else if d.tmp[1] == 0xff {
				i = 1
			} else if d.tmp[1] != 0x00 {
				// libjpeg's jdmarker.c's jpeg_resync_to_restart does something
				// fancy here, treating RST markers within two (modulo 8) of
				// expectedRST differently from RST markers that are 'more
				// distant'. Until we see evidence that recovering from such
				// cases is frequent enough to be worth the complexity, we take
				// a simpler approach for now. Any marker that's not 0x00, 0xff
				// or expectedRST is a fatal FormatError.
				return FormatError("bad RST marker")
			}

		} else if d.tmp[1] == 0xff {
			d.tmp[0] = 0xff
			i = 1
		}

		if err := d.readFull(d.tmp[i:2]); err != nil {
			return err
		}
	}
}
