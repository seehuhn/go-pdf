package builtin

import (
	"sync"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt/glyph"
)

type Font string

func (f Font) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	info, err := getFontInfo(f)
	if err != nil {
		return nil, err
	}

	res := &embedded{
		fontInfo: info,
		w:        w,
		ref:      w.Alloc(),
		resName:  resName,
		enc:      cmap.NewSimpleEncoder(),
	}

	w.AutoClose(res)

	return res, nil
}

func (f Font) GetGeometry() *font.Geometry {
	info, _ := getFontInfo(f)
	return info.GetGeometry()
}

func (f Font) Layout(s string, ptSize float64) glyph.Seq {
	info, _ := getFontInfo(f)
	return info.Layout(s, ptSize)
}

type fontInfo struct {
	afm      *type1.Font
	names    []string
	geom     *font.Geometry
	encoding []glyph.ID
	cmap     map[rune]glyph.ID
	lig      map[pair]glyph.ID
	kern     map[pair]funit.Int16
}

type pair struct {
	left, right glyph.ID
}

func getFontInfo(f Font) (*fontInfo, error) {
	fontCacheLock.Lock()
	defer fontCacheLock.Unlock()

	if res, ok := fontCache[f]; ok {
		return res, nil
	}

	afm, err := f.Afm()
	if err != nil {
		return nil, err
	}

	glyphNames := afm.GlyphList()
	nameGid := make(map[string]glyph.ID, len(glyphNames))
	for i, name := range glyphNames {
		nameGid[name] = glyph.ID(i)
	}

	widths := make([]funit.Int16, len(glyphNames))
	extents := make([]funit.Rect16, len(glyphNames))
	for i, name := range glyphNames {
		gi := afm.GlyphInfo[name]
		widths[i] = gi.WidthX
		extents[i] = gi.Extent
	}

	geom := &font.Geometry{
		UnitsPerEm:   afm.UnitsPerEm,
		Widths:       widths,
		GlyphExtents: extents,

		Ascent:             afm.Ascent,
		Descent:            afm.Descent,
		BaseLineSkip:       1200, // TODO(voss): is this ok?
		UnderlinePosition:  afm.Info.UnderlinePosition,
		UnderlineThickness: afm.Info.UnderlineThickness,
	}

	encoding := make([]glyph.ID, 256)
	for i, name := range afm.Encoding {
		encoding[i] = nameGid[name]
	}

	cmap := make(map[rune]glyph.ID)
	isDingbats := f == "ZapfDingbats"
	for gid, name := range glyphNames {
		rr := names.ToUnicode(name, isDingbats)
		if len(rr) != 1 {
			continue
		}
		r := rr[0]

		if _, exists := cmap[r]; exists {
			continue
		}
		cmap[r] = glyph.ID(gid)
	}

	lig := make(map[pair]glyph.ID)
	for left, name := range glyphNames {
		gi := afm.GlyphInfo[name]
		for right, repl := range gi.Ligatures {
			lig[pair{left: glyph.ID(left), right: nameGid[right]}] = nameGid[repl]
		}
	}

	kern := make(map[pair]funit.Int16)
	for _, k := range afm.Kern {
		left, right := nameGid[k.Left], nameGid[k.Right]
		kern[pair{left: left, right: right}] = k.Adjust
	}

	res := &fontInfo{
		names:    glyphNames,
		afm:      afm,
		geom:     geom,
		encoding: encoding,
		cmap:     cmap,
		lig:      lig,
		kern:     kern,
	}
	fontCache[f] = res
	return res, nil
}

func (info *fontInfo) GetGeometry() *font.Geometry {
	return info.geom
}

func (info *fontInfo) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)

	gg := make(glyph.Seq, len(rr))
	var prev glyph.ID
	for i, r := range rr {
		gid := info.cmap[r]
		if i > 0 {
			if repl, ok := info.lig[pair{left: prev, right: gid}]; ok {
				gg[i-1].Gid = repl
				gg[i-1].Text = append(gg[i-1].Text, r)
				gg = gg[:len(gg)-1]
				prev = repl
				continue
			}
		}
		gg[i].Gid = glyph.ID(gid)
		gg[i].Text = []rune{r}
		prev = gid
	}

	for i, g := range gg {
		if i > 0 {
			if adj, ok := info.kern[pair{left: prev, right: g.Gid}]; ok {
				gg[i-1].Advance += adj
			}
		}
		gg[i].Advance = info.geom.Widths[g.Gid]
		prev = g.Gid
	}

	return gg
}

var (
	fontCache     = make(map[Font]*fontInfo)
	fontCacheLock sync.Mutex
)

// The 14 built-in PDF fonts.
const (
	Courier              Font = "Courier"
	CourierBold          Font = "Courier-Bold"
	CourierBoldOblique   Font = "Courier-BoldOblique"
	CourierOblique       Font = "Courier-Oblique"
	Helvetica            Font = "Helvetica"
	HelveticaBold        Font = "Helvetica-Bold"
	HelveticaBoldOblique Font = "Helvetica-BoldOblique"
	HelveticaOblique     Font = "Helvetica-Oblique"
	TimesRoman           Font = "Times-Roman"
	TimesBold            Font = "Times-Bold"
	TimesBoldItalic      Font = "Times-BoldItalic"
	TimesItalic          Font = "Times-Italic"
	Symbol               Font = "Symbol"
	ZapfDingbats         Font = "ZapfDingbats"
)

// All contains the 14 built-in PDF fonts.
var All = []Font{
	Courier,
	CourierBold,
	CourierBoldOblique,
	CourierOblique,
	Helvetica,
	HelveticaBold,
	HelveticaBoldOblique,
	HelveticaOblique,
	TimesRoman,
	TimesBold,
	TimesBoldItalic,
	TimesItalic,
	Symbol,
	ZapfDingbats,
}
