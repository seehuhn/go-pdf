// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"html"
	"regexp"
	"strings"
)

// standardFonts maps the standard font names to the Quire fonts which
// replace them.  The Adobe metrics file of each font is named after the
// standard font.
var standardFonts = []struct{ name, quire string }{
	{"Courier", "QuireMono-Regular"},
	{"Courier-Bold", "QuireMono-Bold"},
	{"Courier-BoldOblique", "QuireMono-BoldItalic"},
	{"Courier-Oblique", "QuireMono-Italic"},
	{"Helvetica", "QuireSans-Regular"},
	{"Helvetica-Bold", "QuireSans-Bold"},
	{"Helvetica-BoldOblique", "QuireSans-BoldItalic"},
	{"Helvetica-Oblique", "QuireSans-Italic"},
	{"Symbol", "QuireSymbol"},
	{"Times-Bold", "QuireSerif-Bold"},
	{"Times-BoldItalic", "QuireSerif-BoldItalic"},
	{"Times-Italic", "QuireSerif-Italic"},
	{"Times-Roman", "QuireSerif-Regular"},
	{"ZapfDingbats", "QuireDingbats"},
}

// standardName returns the name of the standard font which the given Quire
// font replaces, or "" if there is none.
func standardName(quire string) string {
	for _, f := range standardFonts {
		if f.quire == quire {
			return f.name
		}
	}
	return ""
}

// quireFontFile returns the path of a Quire font, relative to the loader
// package.
func quireFontFile(name string) string {
	if strings.HasPrefix(name, "QuireSymbol") || strings.HasPrefix(name, "QuireDingbats") {
		return "builtin/quire/" + name + ".otf.gz"
	}
	return "builtin/quire/" + name + ".ttf.gz"
}

// makeFontMap returns the content of builtin/font.map.
func makeFontMap() string {
	var b strings.Builder
	for _, f := range standardFonts {
		b.WriteString(f.name + " sfnt " + quireFontFile(f.quire) + "\n")
		b.WriteString(f.name + " afm builtin/adobe/" + f.name + ".afm.gz\n")
	}
	for _, family := range quireFamilies {
		for _, style := range quireStyles {
			name := "Quire" + family + "-" + style
			b.WriteString(name + " sfnt " + quireFontFile(name) + "\n")
		}
	}
	for _, spec := range symbolSpecs {
		b.WriteString(spec.fontName + " sfnt " + quireFontFile(spec.fontName) + "\n")
	}
	return b.String()
}

var (
	reHidden = regexp.MustCompile(`(?s)<font color="white">.*?</font>`)
	reTag    = regexp.MustCompile(`(?s)<[^>]*>`)
	reSpace  = regexp.MustCompile(`\s+`)
)

// mustReadText extracts the visible notice paragraph from MustRead.html.
func mustReadText(page []byte) string {
	body := string(page)
	if i := strings.Index(body, "<body"); i >= 0 {
		body = body[i:]
	}
	body = reHidden.ReplaceAllString(body, "")
	body = reTag.ReplaceAllString(body, " ")
	return strings.TrimSpace(reSpace.ReplaceAllString(html.UnescapeString(body), " "))
}

// makeLicenses returns the content of Licenses.txt.
func makeLicenses(liberation, foxit, mustRead []byte) string {
	section := func(title string, paras ...string) string {
		var s strings.Builder
		s.WriteString(title + "\n" + strings.Repeat("-", len(title)) + "\n\n")
		for _, p := range paras {
			s.WriteString(strings.TrimRight(p, "\n") + "\n\n")
		}
		return s.String()
	}
	var b strings.Builder
	b.WriteString("The fonts and font metrics in this directory are third-party material.\n" +
		"They remain under the licences stated below, independently of the licence\n" +
		"of the go-pdf code (see the file LICENSE in the root of the repository).\n" +
		"Anyone who redistributes these files must comply with these licences.\n\n")
	b.WriteString(section("builtin/quire (QuireSans, QuireSerif, QuireMono)",
		"These fonts are derived from Liberation Fonts 2.1.5, downloaded from\n"+
			"https://github.com/liberationfonts/liberation-fonts/files/7261482/liberation-fonts-ttf-2.1.5.tar.gz .\n"+
			"They are subset to Latin, Greek and Cyrillic and renamed, as the Reserved\n"+
			"Font Names of the license require.  The license text of the original is:",
		string(liberation)))
	b.WriteString(section("builtin/quire (QuireSymbol, QuireDingbats)",
		"These fonts are derived from FoxitSymbol and FoxitDingbats, downloaded from\n"+
			"https://github.com/mozilla/pdf.js/tree/v2.16.105/external/standard_fonts .\n"+
			"They are renamed and wrapped as OpenType.  The PDFium project, where\n"+
			"the originals come from, marks them with the lines\n\n"+
			"  Copyright 2014 PDFium Authors. All rights reserved.\n"+
			"  Original code copyright 2014 Foxit Software Inc. http://www.foxitsoftware.com\n\n"+
			"The license text of the originals is:",
		string(foxit)))
	b.WriteString(section("builtin/adobe",
		"These files were downloaded from\n"+
			"https://github.com/apache/pdfbox/tree/78be2c012e6b5f0c0b368dc9863fc01e9ec78f85/pdfbox/src/main/resources/org/apache/pdfbox/resources/afm .\n"+
			"The files are stored gzip-compressed; their content is unmodified.\n"+
			"The accompanying notice (MustRead.html) reads:",
		mustReadText(mustRead)))
	return strings.TrimRight(b.String(), "\n") + "\n"
}
