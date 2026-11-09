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

package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

const afmBase = "https://raw.githubusercontent.com/apache/pdfbox/78be2c012e6b5f0c0b368dc9863fc01e9ec78f85/pdfbox/src/main/resources/org/apache/pdfbox/resources/afm/"

const foxitBase = "https://raw.githubusercontent.com/mozilla/pdf.js/v2.16.105/external/standard_fonts/"

// A source is a pinned input file of the generator.
type source struct {
	name string // base name of the file
	url  string
	sha  string // SHA-256, hex encoded
}

var (
	liberationSource = source{
		"liberation-fonts-ttf-2.1.5.tar.gz",
		"https://github.com/liberationfonts/liberation-fonts/files/7261482/liberation-fonts-ttf-2.1.5.tar.gz",
		"7191c669bf38899f73a2094ed00f7b800553364f90e2637010a69c0e268f25d0",
	}
	foxitSymbolSource = source{
		"FoxitSymbol.pfb", foxitBase + "FoxitSymbol.pfb",
		"47967d055530e7357088a08403115425643ec2cdfd6201ba8af0fbd7116c1539",
	}
	foxitDingbatsSource = source{
		"FoxitDingbats.pfb", foxitBase + "FoxitDingbats.pfb",
		"845c752392b6c914fb989c75a08b7792b88f542d2499042ef2889f8c814a16ed",
	}
	foxitLicenseSource = source{
		"LICENSE_FOXIT", foxitBase + "LICENSE_FOXIT",
		"b578cdd2345840ada550bd12519533812320d5f1d21cf4c1c7e1b1b0a31c98b7",
	}
	mustReadSource = source{
		"MustRead.html", afmBase + "MustRead.html",
		"b226bfc00e1b8b8a80c7b3cfbbc322d13b4b0401f94cdeafdd93b2210ad802eb",
	}
)

// outDir is the output directory, relative to the loader package.
var outDir string

func afmSource(name, sha string) source {
	return source{name, afmBase + name, sha}
}

// afmSources lists the Adobe Core 14 metrics files.
var afmSources = []source{
	afmSource("Courier-Bold.afm", "1e86f0a949e316c08df32adc2fc2e2a92ed9902ffd54e3516e5a66a32db856da"),
	afmSource("Courier-BoldOblique.afm", "e1349f5f306b16338972eaf7f43d7913535cc075a15189b5f3efc841aee3d02e"),
	afmSource("Courier-Oblique.afm", "9e3494b4abb127a6586c095a377924514dd6956973f8f3b0bd7d6d00fcc89ea6"),
	afmSource("Courier.afm", "e72833dda79161e3aaf385adfeb59c61c89a4a3e184f27d21c178056eea30e17"),
	afmSource("Helvetica-Bold.afm", "59ac93091bc7e76eb6baf60210526a4df88fe107dd5e13eb0ae70c9fc6d91fb9"),
	afmSource("Helvetica-BoldOblique.afm", "1f848276efd76c0e5020ef98ccb459005f5a022f56cb8579e52c98e33ffab03b"),
	afmSource("Helvetica-Oblique.afm", "8c00ac46ca5963df22d4fc1153ea1246c8778475820378640c8d54d4fd3a46fe"),
	afmSource("Helvetica.afm", "5654761eab868ccb3080122accf13b4fb79edf37af09e1c57a881c35c334ee03"),
	afmSource("Symbol.afm", "13eeef80def2f9a8d3a31a980731376823e8b48340ff3891ffc1cc0299c0b4fe"),
	afmSource("Times-Bold.afm", "37efa084183acfb06f45ad73bd75f6dd09b20d1e498c66ce1b9aa72b00db4ac7"),
	afmSource("Times-BoldItalic.afm", "1bf83cee5300d4de1ce90681a2a2d70fd1876c664e91c1c4c6b6b5b12873b6da"),
	afmSource("Times-Italic.afm", "2f34d3bc49530532daa690aec165b68368162d4c27d24fbe6a3fb24aae49a74a"),
	afmSource("Times-Roman.afm", "b6dbde33196ff61f051f8468fb81a0dece63997a80b7fba058e8295f49480026"),
	afmSource("ZapfDingbats.afm", "d3e7b9f6366ab0df06acbb217c8586c5962ff2efbd4e1b5566c7eaf5b745dd41"),
}

// sources is the table of all pinned inputs.
var sources = func() []source {
	s := []source{liberationSource, foxitSymbolSource, foxitDingbatsSource, foxitLicenseSource}
	s = append(s, afmSources...)
	return append(s, mustReadSource)
}()

// load fetches and verifies a source.
func load(s source) ([]byte, error) {
	if path.Base(s.url) != s.name {
		return nil, fmt.Errorf("%s: URL does not end in the file name", s.name)
	}
	return fetch(s.url, s.sha)
}

func main() {
	flag.StringVar(&srcDir, "src", "", "read the source files from this directory instead of downloading them")
	flag.StringVar(&outDir, "out", "builtin", "output directory; its subdirectories quire/ and adobe/ are replaced")
	flag.Parse()
	log.SetFlags(0)
	log.SetPrefix("genbuiltin: ")

	if srcDir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			log.Fatal(err)
		}
		cacheDir = filepath.Join(base, "seehuhn-genbuiltin")
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	data := make(map[string][]byte)
	total := 0
	for _, s := range sources {
		d, err := load(s)
		if err != nil {
			return err
		}
		data[s.name] = d
		total += len(d)
	}
	fmt.Printf("loaded %d source files (%d bytes)\n", len(data), total)

	fonts, license, err := extractLiberation(data[liberationSource.name])
	if err != nil {
		return err
	}
	fmt.Printf("extracted %d Liberation fonts and LICENSE (%d bytes)\n", len(fonts), len(license))

	files := make(map[string][]byte) // keyed by path relative to outDir

	latin, err := makeQuireLatin(fonts, data)
	if err != nil {
		return err
	}
	for _, f := range latin {
		z, err := gzipBytes(f.data)
		if err != nil {
			return err
		}
		files["quire/"+f.name+".gz"] = z
	}

	for _, spec := range symbolSpecs {
		otf, err := makeQuireSymbol(spec, data[spec.src.name])
		if err != nil {
			return err
		}
		z, err := gzipBytes(otf)
		if err != nil {
			return err
		}
		files["quire/"+spec.fontName+".otf.gz"] = z
	}

	for _, s := range afmSources {
		z, err := gzipBytes(data[s.name])
		if err != nil {
			return err
		}
		files["adobe/"+s.name+".gz"] = z
	}
	files["adobe/"+mustReadSource.name] = data[mustReadSource.name]

	files["font.map"] = []byte(makeFontMap())
	files["Licenses.txt"] = []byte(makeLicenses(license, data[foxitLicenseSource.name], data[mustReadSource.name]))

	return writeOutputs(files)
}

// generatedDirs lists the subdirectories of outDir which are replaced
// completely.
var generatedDirs = []string{"quire", "adobe"}

// writeOutputs writes the files below outDir, replacing the generated
// directories quire and adobe.  The files are first written to a temporary
// directory next to outDir, so that a failure while writing leaves outDir
// unchanged.  A failure while the new files are moved into place can leave
// outDir partly updated.
func writeOutputs(files map[string][]byte) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(filepath.Clean(outDir)), ".genbuiltin-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	for _, dir := range generatedDirs {
		if err := os.Mkdir(filepath.Join(tmp, dir), 0o755); err != nil {
			return err
		}
	}

	paths := slices.Sorted(maps.Keys(files))
	total := 0
	for _, p := range paths {
		full := filepath.Join(tmp, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, files[p], 0o644); err != nil {
			return err
		}
		fmt.Printf("%8d %s\n", len(files[p]), p)
		if strings.HasSuffix(p, ".gz") {
			total += len(files[p])
		}
	}
	fmt.Printf("%8d total (gzipped files)\n", total)

	// swap in the new files; the old directories are moved into tmp and
	// removed together with it
	for _, dir := range generatedDirs {
		old := filepath.Join(outDir, dir)
		if err := os.Rename(old, filepath.Join(tmp, "old-"+dir)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.Rename(filepath.Join(tmp, dir), old); err != nil {
			return err
		}
	}
	for _, p := range paths {
		if strings.Contains(p, "/") {
			continue
		}
		if err := os.Rename(filepath.Join(tmp, p), filepath.Join(outDir, p)); err != nil {
			return err
		}
	}
	return nil
}
