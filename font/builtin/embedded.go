package builtin

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/sfnt/glyph"
)

type embedded struct {
	*fontInfo
	w       pdf.Putter
	ref     pdf.Reference
	resName pdf.Name
	enc     cmap.SimpleEncoder
	closed  bool
}

func (f *embedded) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	res := &embedded{
		fontInfo: f.fontInfo,
		w:        w,
		ref:      w.Alloc(),
		resName:  resName,
		enc:      cmap.NewSimpleEncoder(),
	}

	w.AutoClose(res)

	return res, nil
}

func (e *embedded) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	return append(s, e.enc.Encode(gid, rr))
}

func (f *embedded) ResourceName() pdf.Name {
	return f.resName
}

func (f *embedded) Reference() pdf.Reference {
	return f.ref
}

func (f *embedded) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.enc.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			f.resName, f.afm.Info.FontName)
	}
	f.enc = cmap.NewFrozenSimpleEncoder(f.enc)

	// See section 9.6.2.1 of PDF 32000-1:2008.
	Font := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name(f.afm.Info.FontName),
	}

	isDingbats := f.afm.Info.FontName == "ZapfDingbats"

	enc := font.DescribeEncoding(f.enc.Encoding(), f.fontInfo.encoding,
		f.fontInfo.names, isDingbats)
	if enc != nil {
		Font["Encoding"] = enc
	}
	if f.w.GetMeta().Version == pdf.V1_0 {
		Font["Name"] = f.resName
	}

	err := f.w.Put(f.ref, Font)
	return err
}
