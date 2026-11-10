// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The cases below use the encoder of the ITU-T T.88 sample software, which
// is the only available encoder for generic regions with templates 1-3,
// freely placed AT pixels and the extended template.  The software is not
// available from package managers; it is part of the ITU-T T.88 (08/2018)
// software package, file T-REC-T.88-201808-I!!SOFT-ZST-E.zip (sha256
// e2a3c458c4d28d5a835c0815fd9b23e197dbc1b83fe81d8e8d8b2ac413d8f22f), which
// contains Software/JBIG2_SampleSoftware-A20180829.zip (sha256
// c9e7fc74d4792653de258be4f77c05d639ced4518dd57cc5a6ddfda541e2d181).
// Build it with "make jbig2" in its source directory and set the
// environment variable T88_JBIG2 to the path of the resulting binary.
// Without the variable, these cases are skipped and the committed files
// are left unchanged.
//
// The sample encoder cannot be used with TPGDON=1: in MQ_EncImage the
// branch for typical prediction is empty, so the encoder sets the TPGDON
// flag but codes no pixels at all.  The cases therefore all use TPGDON=0.

const (
	t88EnvVar  = "T88_JBIG2"
	t88Version = "JBIG2_SampleSoftware-A20180829"
)

// t88FileHeader is the file header (T.88, section D.4) which the sample
// encoder writes: ID string, sequential organisation with a known number
// of pages, and one page.
const t88FileHeader = "\x97JB2\r\n\x1a\n\x01\x00\x00\x00\x01"

// t88Generic describes the parameters of a generic region coded by the
// T.88 sample software.
type t88Generic struct {
	template int
	ext      bool     // extended template (12 AT pixels, template 0 only)
	at       [][2]int // AT pixels (x, y)
}

// ini returns the parameter file for the T.88 sample software.
func (p *t88Generic) ini() string {
	b := &strings.Builder{}
	b.WriteString("-Gen -Seg 1\n")
	b.WriteString("-Gen -Param -Huff 0\n")
	fmt.Fprintf(b, "-Gen -Param -Template %d\n", p.template)
	if p.ext {
		b.WriteString("-Gen -Param -ExtTemplate 1\n")
	}
	b.WriteString("-Gen -Param -TpGDon 0\n")
	for i, a := range p.at {
		fmt.Fprintf(b, "-Gen -Param -ATX%d %d\n", i+1, a[0])
		fmt.Fprintf(b, "-Gen -Param -ATY%d %d\n", i+1, a[1])
	}
	return b.String()
}

func (p *t88Generic) settings() []string {
	res := []string{fmt.Sprintf("GBTEMPLATE=%d", p.template)}
	if p.ext {
		res = append(res, "EXTTEMPLATE=1")
	}
	res = append(res, "TPGDON=0")
	var at []string
	for _, a := range p.at {
		at = append(at, fmt.Sprintf("(%d,%d)", a[0], a[1]))
	}
	return append(res, "AT pixels "+strings.Join(at, " "))
}

// t88GenericCase returns a test case for a generic region coded by the
// T.88 sample software from a crop of the text page.
func t88GenericCase(name string, x, y, w, h int, p *t88Generic, jbig2decOK bool) *testCase {
	c := &testCase{
		name:     name,
		needsT88: true,
		input: func(page *bilevel) *bilevel {
			return page.crop(x, y, w, h)
		},
		expect:     same,
		verify:     t88Verify,
		encoder:    "ITU-T T.88 sample software (" + t88Version + "), immediate lossless generic region",
		settings:   p.settings(),
		noJbig2dec: !jbig2decOK,
	}
	c.encode = func(g *generator, in *bilevel, _ string) ([]byte, []byte, error) {
		raw, err := g.t88Encode(name, in, p.ini())
		if err != nil {
			return nil, nil, err
		}
		page, edits, err := t88Embedded(raw)
		c.edits = append([]string{"file header removed"}, edits...)
		return nil, page, err
	}
	if jbig2decOK {
		c.expected = "jbig2dec output and T.88 sample software decoder output, both identical to the encoder input"
	} else {
		c.expected = "T.88 sample software decoder output, identical to the encoder input " +
			"(jbig2dec 0.20 does not implement the extended template)"
	}
	return c
}

var t88Cases = []*testCase{
	t88GenericCase("test_t88_gen_t0_at", 200, 1380, 400, 128, &t88Generic{
		template: 0,
		at:       [][2]int{{4, -1}, {-4, -1}, {3, -2}, {-3, -2}},
	}, true),
	t88GenericCase("test_t88_gen_t1", 200, 312, 320, 112, &t88Generic{
		template: 1,
		at:       [][2]int{{-3, -1}},
	}, true),
	t88GenericCase("test_t88_gen_t2", 200, 545, 320, 96, &t88Generic{
		template: 2,
		at:       [][2]int{{2, -2}},
	}, true),
	t88GenericCase("test_t88_gen_t3", 200, 200, 320, 96, &t88Generic{
		template: 3,
		at:       [][2]int{{-1, -2}},
	}, true),
	t88GenericCase("test_t88_gen_ext", 700, 545, 400, 128, &t88Generic{
		template: 0,
		ext:      true,
		at: [][2]int{
			{-2, 0}, {0, -2}, {-2, -1}, {-1, -2}, {1, -2}, {2, -1},
			{-3, 0}, {-4, 0}, {2, -2}, {3, -1}, {-2, -2}, {-3, -1},
		},
	}, false),
}

func (g *generator) t88Dir() (string, error) {
	dir := filepath.Join(g.tmp, "t88")
	return dir, os.MkdirAll(dir, 0o755)
}

// t88Encode runs the T.88 sample encoder on the image and returns the
// file it writes.
func (g *generator) t88Encode(name string, in *bilevel, ini string) ([]byte, error) {
	dir, err := g.t88Dir()
	if err != nil {
		return nil, err
	}
	if err := writeBMPPalette(filepath.Join(dir, name+".bmp"), in, true); err != nil {
		return nil, err
	}
	iniName := name + ".ini"
	if err := os.WriteFile(filepath.Join(dir, iniName), []byte(ini), 0o644); err != nil {
		return nil, err
	}
	out := name + "_enc"
	if err := g.runT88(dir, "-i", name, "-f", "bmp", "-o", out, "-F", "jb2", "-ini", iniName); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, out+".jb2"))
}

// t88Binary returns the path of the T.88 sample software binary given by
// the environment variable t88EnvVar, or "" if the variable is not set.
// The program runs in a different directory, so a relative path is made
// absolute.
func t88Binary() (string, error) {
	name := os.Getenv(t88EnvVar)
	if name == "" {
		return "", nil
	}
	if strings.ContainsRune(name, os.PathSeparator) {
		abs, err := filepath.Abs(name)
		if err != nil {
			return "", err
		}
		name = abs
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s: %w", t88EnvVar, err)
	}
	return path, nil
}

// t88Verify decodes the edited streams with the T.88 sample decoder.  The
// decoder reads only the file organisation, so the page stream is given
// the file header of a single-page sequential file (T.88, section D.4).
func t88Verify(g *generator, c *testCase, globals, page []byte) (*bilevel, error) {
	if len(globals) > 0 {
		return nil, errors.New("T.88 sample decoder cannot read a globals stream")
	}
	dir, err := g.t88Dir()
	if err != nil {
		return nil, err
	}
	emb := c.name + "_emb"
	dec := c.name + "_dec"
	file := append([]byte(t88FileHeader), page...)
	if err := os.WriteFile(filepath.Join(dir, emb+".jb2"), file, 0o644); err != nil {
		return nil, err
	}
	if err := g.runT88(dir, "-i", emb, "-f", "jb2", "-o", dec, "-F", "bmp"); err != nil {
		return nil, err
	}
	// the decoder numbers its output pages from 00
	return readBMP(filepath.Join(dir, dec+"00.bmp"))
}

// runT88 runs the T.88 sample software in dir.  The program reports
// errors on standard output and mostly exits with status 0, so success is
// recognised by its completion message.
func (g *generator) runT88(dir string, args ...string) error {
	cmd := exec.Command(g.t88, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil || !strings.Contains(out.String(), "===>complete") {
		return fmt.Errorf("%s %s: %v\n%s", g.t88, strings.Join(args, " "), err, out.String())
	}
	return nil
}

// t88Embedded converts the output of the T.88 sample encoder to the PDF
// embedded organisation, by removing the file header, and returns a
// description of the further changes made.
//
// The arithmetic-coded data of a generic region must end with the
// terminating marker 0xFF 0xAC (T.88, Figure E.11: FLUSH appends 0xFF
// unless the last byte already is 0xFF, and then 0xAC).  The sample
// encoder's flush follows a different scheme: it shifts out the remaining
// bits of the C register and writes no marker, so its data end with
// either 0xFF or some other byte.  The missing part of the marker is
// appended.  This does not change the decoded image: a decoder which
// meets the marker supplies 1-bits from then on (BYTEIN, Figure E.19: C is
// incremented by 0xFF00), and reading the appended 0xFF as a data byte
// adds the same 0xFF00, so the decoder registers evolve exactly as they
// would if the data ended at the original last byte and were padded with
// 1-bits.  The generator still compares the decoded images with the
// encoder input.
func t88Embedded(file []byte) (page []byte, edits []string, err error) {
	if !bytes.HasPrefix(file, []byte(t88FileHeader)) {
		return nil, nil, errors.New("unexpected T.88 sample software file header")
	}
	segs, err := parseSegments(file[len(t88FileHeader):])
	if err != nil {
		return nil, nil, err
	}
	gr, err := findSegment(segs, segLosslessGeneric)
	if err != nil {
		return nil, nil, err
	}
	if len(gr.data) < 19 || gr.data[17]&0x01 != 0 {
		return nil, nil, errors.New("unexpected generic region segment")
	}
	pi, err := findSegment(segs, segPageInfo)
	if err != nil {
		return nil, nil, err
	}
	if len(pi.data) < 19 {
		return nil, nil, errors.New("unexpected page information segment")
	}
	// T.88, 7.4.8.5: a region may only use a combination operator other
	// than the page default if the page sets the override flag
	defaultOp := (pi.data[16] >> 3) & 0x03
	if gr.data[16]&0x07 != defaultOp && pi.data[16]&pageCombOpOverride == 0 {
		pi.data[16] |= pageCombOpOverride
		edits = append(edits, "page information: combination operator override flag set, "+
			"since the generic region uses a combination operator other than the page default")
	}

	data := gr.data[:len(gr.data):len(gr.data)]
	n := len(data)
	switch {
	case n >= 2 && data[n-2] == 0xFF && data[n-1] > 0x8F:
		// already terminated
	case data[n-1] == 0xFF:
		gr.data = append(data, 0xAC)
		edits = append(edits, "terminating marker completed: 0xAC appended after the final 0xFF of the arithmetic-coded data")
	default:
		gr.data = append(data, 0xFF, 0xAC)
		edits = append(edits, "terminating marker appended: 0xFF 0xAC after the final byte of the arithmetic-coded data")
	}
	return encodeSegments(segs), edits, nil
}
