// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Generate writes the JBIG2 decoder test cases which are built from a page
// of text set by this library.
//
// The source page is a PDF file written with the Go fonts and rendered to
// a bi-level image by Ghostscript.  The compressed streams come from
// encoders independent of the jbig2 package (jbig2enc, and libtiff's Group 4
// coder for the MMR case); the generator only parses and rewrites segment
// headers around their output.  Every expected bitmap is the output of
// a decoder independent of the jbig2 package (jbig2dec, or the T.88 sample
// decoder where jbig2dec lacks a feature), and for lossless streams it is
// also checked against the source image.
//
// Usage, from the jbig2 package directory:
//
//	go generate
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// tool versions which produced the committed test data
var pinnedVersions = map[string]string{
	"jbig2enc":    "0.32",
	"jbig2dec":    "0.20",
	"ghostscript": "10.08.0",
	"libtiff":     "4.7.2",
}

const resolution = 200 // dpi of the rendered source page

func main() {
	outDir := flag.String("o", "testdata/decode", "output directory")
	keep := flag.Bool("k", false, "keep the temporary directory")
	flag.Parse()

	if err := run(*outDir, *keep); err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
}

func run(outDir string, keep bool) error {
	versions, err := toolVersions()
	if err != nil {
		return err
	}
	for tool, v := range versions {
		if pinnedVersions[tool] != v {
			fmt.Fprintf(os.Stderr, "warning: %s version %s differs from pinned version %s\n",
				tool, v, pinnedVersions[tool])
		}
	}

	// With JBIG2_KEEP set, all intermediate files (encoder inputs and
	// outputs, parameter files, decoder outputs) are kept in that
	// directory, also for failing cases.
	var tmp string
	if dir := os.Getenv("JBIG2_KEEP"); dir != "" {
		tmp = dir
		if err := os.MkdirAll(tmp, 0o755); err != nil {
			return err
		}
		keep = true
	} else {
		tmp, err = os.MkdirTemp("", "jbig2gen")
		if err != nil {
			return err
		}
	}
	if keep {
		fmt.Fprintln(os.Stderr, "intermediate files in", tmp)
	} else {
		defer os.RemoveAll(tmp)
	}

	// Output goes to a staging directory first, so that a failing case
	// leaves the committed test data untouched.
	stage := filepath.Join(tmp, "out")
	if err := os.RemoveAll(stage); err != nil {
		return err
	}
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return err
	}

	t88, err := t88Binary()
	if err != nil {
		return err
	}
	g := &generator{tmp: tmp, out: stage, versions: versions, t88: t88}

	pdfName := filepath.Join(tmp, "page.pdf")
	if err := writeSourcePDF(pdfName); err != nil {
		return err
	}
	pbmName := filepath.Join(tmp, "page.pbm")
	_, err = runTool("gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE",
		"-sDEVICE=pbmraw", fmt.Sprintf("-r%d", resolution),
		"-dTextAlphaBits=1", "-dGraphicsAlphaBits=1",
		"-o", pbmName, pdfName)
	if err != nil {
		return err
	}
	g.page, err = readPBM(pbmName)
	if err != nil {
		return err
	}
	wantW := pageWidth * resolution / 72
	wantH := pageHeight * resolution / 72
	if g.page.w != wantW || g.page.h != wantH {
		return fmt.Errorf("rendered page is %dx%d pixels, expected %dx%d",
			g.page.w, g.page.h, wantW, wantH)
	}

	all := cases
	if g.t88 == "" {
		fmt.Fprintf(os.Stderr, "%s not set: skipping %d cases which need the ITU-T T.88 sample software;\n"+
			"their committed files are left unchanged (see generate/t88.go)\n", t88EnvVar, len(t88Cases))
	} else {
		all = append(all[:len(all):len(all)], t88Cases...)
	}
	var failed []string
	for _, c := range all {
		if err := g.generate(c); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", c.name, err)
			failed = append(failed, c.name)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed cases: %s (%s left unchanged)", strings.Join(failed, ", "), outDir)
	}

	return copyFiles(outDir, stage)
}

// copyFiles copies the regular files in src to dst.
func copyFiles(dst, src string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

type generator struct {
	tmp      string
	out      string // the staging directory for the test data
	versions map[string]string
	page     *bilevel // the rendered source page
	t88      string   // path of the T.88 sample software binary, or ""
}

// A testCase describes how one test case is built.
type testCase struct {
	name string

	// input returns the input image for the encoder.
	input func(page *bilevel) *bilevel

	// encode runs the external encoder on the input image (stored as
	// inFile in PBM format) and returns the globals and page streams.
	encode func(g *generator, in *bilevel, inFile string) (globals, page []byte, err error)

	// expect returns the expected decoded image for lossless cases, or
	// nil if the expected image is taken from jbig2dec alone.
	expect func(in *bilevel) *bilevel

	// verify, if set, decodes the globals and page streams with a second
	// decoder; the result must equal the expected image.
	verify func(g *generator, c *testCase, globals, page []byte) (*bilevel, error)

	needsT88   bool // the case needs the T.88 sample software
	noJbig2dec bool // jbig2dec cannot decode the stream

	encoder  string   // which tool produced the entropy-coded data
	settings []string // encoder settings, if not evident from encoder
	edits    []string // changes made to the encoder's output
	expected string   // how the expected bitmap was verified, if non-default
}

func (g *generator) generate(c *testCase) error {
	in := c.input(g.page)
	if in.blank() {
		return errors.New("encoder input has no black pixels")
	}
	inFile := filepath.Join(g.tmp, c.name+".in.pbm")
	if err := writePBM(inFile, in); err != nil {
		return err
	}

	globals, page, err := c.encode(g, in, inFile)
	if err != nil {
		return err
	}

	globalsFile := filepath.Join(g.tmp, c.name+".globals")
	pageFile := filepath.Join(g.tmp, c.name+".page")
	outFile := filepath.Join(g.tmp, c.name+".out.pbm")
	if err := os.WriteFile(globalsFile, globals, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(pageFile, page, 0o644); err != nil {
		return err
	}

	// independent decoders
	var decoded *bilevel
	if !c.noJbig2dec {
		if _, err := runTool("jbig2dec", "-e", "-t", "pbm", "-o", outFile, globalsFile, pageFile); err != nil {
			return err
		}
		decoded, err = readPBM(outFile)
		if err != nil {
			return err
		}
		if c.expect != nil && !decoded.equal(c.expect(in)) {
			return errors.New("jbig2dec output differs from the expected image")
		}
	}
	if c.verify != nil {
		if c.expect == nil {
			return errors.New("second decoder needs an expected image")
		}
		other, err := c.verify(g, c, globals, page)
		if err != nil {
			return err
		}
		if !other.equal(c.expect(in)) {
			return errors.New("second decoder output differs from the expected image")
		}
		if decoded == nil {
			decoded = other
		}
	}
	if decoded == nil {
		return errors.New("no decoder checked the stream")
	}

	base := filepath.Join(g.out, c.name)
	if err := os.WriteFile(base+".globals", globals, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(base+".page", page, 0o644); err != nil {
		return err
	}
	if err := writeBMP(base+".bmp", decoded); err != nil {
		return err
	}
	return g.writeManifest(base+".json", c, globals, page, c.expect != nil)
}

// manifest is the content of the .json file for a test case.
type manifest struct {
	Segments        []manifestSegment `json:"segments"`
	GlobalsBytes    int               `json:"globals_bytes"`
	PageBytes       int               `json:"page_bytes"`
	GlobalsSegments int               `json:"globals_segments"`
	PageSegments    int               `json:"page_segments"`
	Source          manifestSource    `json:"source"`
}

type manifestSegment struct {
	Num         uint32 `json:"num"`
	Type        string `json:"type"`
	TypeID      int    `json:"type_id"`
	Page        uint32 `json:"page"`
	HeaderBytes int    `json:"header_bytes"`
	DataBytes   int    `json:"data_bytes"`
}

type manifestSource struct {
	Image    string            `json:"image"`
	Encoder  string            `json:"encoder"`
	Settings []string          `json:"settings,omitempty"`
	Edits    []string          `json:"edits,omitempty"`
	Expected string            `json:"expected_bitmap"`
	Tools    map[string]string `json:"tool_versions"`
}

func (g *generator) writeManifest(fileName string, c *testCase, globals, page []byte, lossless bool) error {
	gs, err := parseSegments(globals)
	if err != nil {
		return err
	}
	ps, err := parseSegments(page)
	if err != nil {
		return err
	}
	m := &manifest{
		GlobalsBytes:    len(globals),
		PageBytes:       len(page),
		GlobalsSegments: len(gs),
		PageSegments:    len(ps),
	}
	for _, s := range append(gs, ps...) {
		m.Segments = append(m.Segments, manifestSegment{
			Num:         s.number,
			Type:        segTypeNames[s.typ],
			TypeID:      s.typ,
			Page:        s.page,
			HeaderBytes: s.headerLen(),
			DataBytes:   len(s.data),
		})
	}
	in := c.input(g.page)
	m.Source = manifestSource{
		Image: fmt.Sprintf("%dx%d pixels of the generated text page (generate/source.go), rendered by Ghostscript at %d dpi",
			in.w, in.h, resolution),
		Encoder:  c.encoder,
		Settings: c.settings,
		Edits:    c.edits,
		Tools:    maps.Clone(g.versions),
	}
	if c.needsT88 {
		m.Source.Tools["t88_sample_software"] = t88Version
	}
	if c.expected != "" {
		m.Source.Expected = c.expected
	} else if lossless {
		m.Source.Expected = "jbig2dec output, identical to the expected image computed from the source"
	} else {
		m.Source.Expected = "jbig2dec output (lossy symbol coding)"
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fileName, append(data, '\n'), 0o644)
}

// runTool runs an external program and returns its standard output.
// Any output on standard error is treated as a failure, since jbig2dec
// reports problems with the stream there.
func runTool(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil || stderr.Len() > 0 {
		return nil, fmt.Errorf("%s %s: %v\n%s", name, strings.Join(args, " "), err, stderr.String())
	}
	return stdout.Bytes(), nil
}

var versionRe = regexp.MustCompile(`[0-9]+(\.[0-9]+)+`)

func toolVersions() (map[string]string, error) {
	res := make(map[string]string)
	probe := []struct {
		tool string
		cmd  []string
		pfx  string // the line containing the version starts with this
	}{
		{"jbig2enc", []string{"jbig2", "--version"}, "jbig2enc"},
		{"jbig2dec", []string{"jbig2dec", "--version"}, "jbig2dec"},
		{"ghostscript", []string{"gs", "--version"}, ""},
		{"libtiff", []string{"ppm2tiff"}, "LIBTIFF"},
	}
	for _, p := range probe {
		// Some of these print their version on stderr and exit with a
		// non-zero status, so only the output is inspected.
		out, _ := exec.Command(p.cmd[0], p.cmd[1:]...).CombinedOutput()
		var v string
		for l := range strings.Lines(string(out)) {
			if strings.HasPrefix(strings.TrimSpace(l), p.pfx) {
				v = versionRe.FindString(l)
				break
			}
		}
		if v == "" {
			return nil, fmt.Errorf("cannot determine the version of %s (is it installed?)", p.cmd[0])
		}
		res[p.tool] = v
	}
	return res, nil
}
