// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
)

// page information flags (T.88, section 7.4.8.5)
const (
	pageLossless       = 0x01
	pageDefaultPixel1  = 0x04
	pageCombOpOverride = 0x40
)

const combOpXOR = 2

var cases = []*testCase{
	{
		name:  "test_enc_page_generic",
		input: wholePage,
		encode: func(g *generator, in *bilevel, inFile string) ([]byte, []byte, error) {
			segs, err := jbig2encGeneric(inFile)
			if err != nil {
				return nil, nil, err
			}
			pi, err := findSegment(segs, segPageInfo)
			if err != nil {
				return nil, nil, err
			}
			gr, err := findSegment(segs, segImmediateGeneric)
			if err != nil {
				return nil, nil, err
			}
			pi.data[16] |= pageCombOpOverride
			gr.typ = segLosslessGeneric
			gr.data[16] = combOpXOR
			segs, err = appendEndOfPage(segs)
			if err != nil {
				return nil, nil, err
			}
			return nil, encodeSegments(segs), nil
		},
		expect:  same,
		encoder: "jbig2enc: jbig2 -p (generic region, template 0, nominal AT pixels)",
		edits: []string{
			"page information: set the combination operator override flag",
			"generic region: segment type 38 changed to 39 (immediate lossless), combination operator XOR",
			"end of page segment appended",
		},
	},
	{
		name:  "test_g4_page_mmr",
		input: wholePage,
		encode: func(g *generator, in *bilevel, inFile string) ([]byte, []byte, error) {
			tiffFile := filepath.Join(g.tmp, "page.tif")
			_, err := runTool("ppm2tiff", "-c", "g4", "-r", strconv.Itoa(in.h), inFile, tiffFile)
			if err != nil {
				return nil, nil, err
			}
			g4, err := readG4Strip(tiffFile)
			if err != nil {
				return nil, nil, err
			}
			region := regionInfoData(in.w, in.h, 0, 0, combOpXOR)
			region = append(region, 0x01) // generic region flags: MMR=1
			segs := []*segment{
				{number: 0, typ: segPageInfo, page: 1,
					data: pageInfoData(in.w, in.h, pageLossless|pageCombOpOverride)},
				{number: 1, typ: segLosslessGeneric, page: 1, data: append(region, g4...)},
				{number: 2, typ: segEndOfPage, page: 1},
			}
			return nil, encodeSegments(segs), nil
		},
		expect:  same,
		encoder: "libtiff: ppm2tiff -c g4 (CCITT Group 4 strip, used unchanged as MMR data)",
		edits: []string{
			"page information, generic region (MMR=1, combination operator XOR) " +
				"and end of page segments written by the generator around the Group 4 data",
		},
	},
	{
		name:  "test_enc_text_tpgd",
		input: cropTPGD,
		encode: func(g *generator, in *bilevel, inFile string) ([]byte, []byte, error) {
			out, err := runTool("jbig2", "-p", "-d", inFile)
			if err != nil {
				return nil, nil, err
			}
			return nil, out, nil
		},
		expect:  same,
		encoder: "jbig2enc: jbig2 -p -d (generic region, template 0, TPGDON=1)",
	},
	{
		name:  "test_enc_text_defpixel",
		input: cropDefPixel,
		encode: func(g *generator, in *bilevel, inFile string) ([]byte, []byte, error) {
			segs, err := jbig2encGeneric(inFile)
			if err != nil {
				return nil, nil, err
			}
			pi, err := findSegment(segs, segPageInfo)
			if err != nil {
				return nil, nil, err
			}
			gr, err := findSegment(segs, segImmediateGeneric)
			if err != nil {
				return nil, nil, err
			}
			pi.data[16] |= pageDefaultPixel1 | pageCombOpOverride
			gr.data[16] = combOpXOR
			return nil, encodeSegments(segs), nil
		},
		expect:  inverted,
		encoder: "jbig2enc: jbig2 -p (generic region, template 0, nominal AT pixels)",
		edits: []string{
			"page information: default pixel value 1, combination operator override flag",
			"generic region: combination operator XOR (the page shows the inverted input)",
		},
	},
	{
		name:  "test_enc_text_symbol_local",
		input: cropSymbol,
		encode: func(g *generator, in *bilevel, inFile string) ([]byte, []byte, error) {
			file, err := runTool("jbig2", "-s", inFile)
			if err != nil {
				return nil, nil, err
			}
			// file header (T.88, section D.4): ID string, flags, page count
			const headerLen = 13
			if len(file) < headerLen || !bytes.HasPrefix(file, []byte("\x97JB2\r\n\x1a\n")) || file[8] != 0x01 {
				return nil, nil, errors.New("unexpected jbig2enc file header")
			}
			segs, err := parseSegments(file[headerLen:])
			if err != nil {
				return nil, nil, err
			}
			sd, err := findSegment(segs, segSymbolDict)
			if err != nil {
				return nil, nil, err
			}
			pi, err := findSegment(segs, segPageInfo)
			if err != nil {
				return nil, nil, err
			}
			tr, err := findSegment(segs, segImmediateText)
			if err != nil {
				return nil, nil, err
			}
			if sd.page != 0 {
				return nil, nil, fmt.Errorf("symbol dictionary on page %d", sd.page)
			}
			sd.page = 1
			copy(pi.data[0:8], pageInfoData(in.w+2*symbolDX, in.h+2*symbolDY, 0)[0:8])
			copy(tr.data[8:16], regionInfoData(0, 0, symbolDX, symbolDY, 0)[8:16])
			return nil, encodeSegments(segs), nil
		},
		encoder: "jbig2enc: jbig2 -s (arithmetic symbol dictionary and text region)",
		edits: []string{
			"file header removed",
			"symbol dictionary: page association 0 changed to 1 (page-local dictionary)",
			fmt.Sprintf("page information: page enlarged; text region moved to (%d,%d)", symbolDX, symbolDY),
		},
	},
}

// position of the text region in test_enc_text_symbol_local
const (
	symbolDX = 4
	symbolDY = 1
)

func wholePage(page *bilevel) *bilevel { return page }

func cropTPGD(page *bilevel) *bilevel { return page.crop(130, 280, 800, 400) }

func cropDefPixel(page *bilevel) *bilevel { return page.crop(190, 180, 320, 96) }

func cropSymbol(page *bilevel) *bilevel { return page.crop(190, 1000, 1000, 360) }

func same(in *bilevel) *bilevel { return in }

func inverted(in *bilevel) *bilevel {
	out := newBilevel(in.w, in.h)
	for y := range in.h {
		for x := range in.w {
			if !in.get(x, y) {
				out.set(x, y)
			}
		}
	}
	return out
}

// jbig2encGeneric runs jbig2enc in generic region mode with PDF output and
// returns the segments.
func jbig2encGeneric(inFile string) ([]*segment, error) {
	out, err := runTool("jbig2", "-p", inFile)
	if err != nil {
		return nil, err
	}
	return parseSegments(out)
}
