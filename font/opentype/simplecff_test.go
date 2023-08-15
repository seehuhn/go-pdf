package opentype

import (
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/sfnt/glyph"
)

func TestRoundTripSimpleCFF(t *testing.T) {
	otf, err := gofont.OpenType(gofont.GoRegular)
	if err != nil {
		t.Fatal(err)
	}

	encoding := make([]glyph.ID, 256)
	encoding[65] = otf.CMap.Lookup('A')
	encoding[66] = otf.CMap.Lookup('C')

	toUnicode := map[charcode.CharCode][]rune{
		65: {'A'},
		66: {'C'},
	}

	info1 := &EmbedInfoSimpleCFF{
		Font:      otf,
		SubsetTag: "UVWXYZ",
		Encoding:  encoding,
		ToUnicode: toUnicode,
	}

	rw := pdf.NewData(pdf.V1_7)
	ref := rw.Alloc()
	err = info1.Embed(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(rw, ref)
	if err != nil {
		t.Fatal(err)
	}
	info2, err := Extract(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	// Compare encodings:
	if len(info1.Encoding) != len(info2.Encoding) {
		t.Fatalf("len(info1.Encoding) != len(info2.Encoding): %d != %d", len(info1.Encoding), len(info2.Encoding))
	}
	for i, gid := range info1.Encoding {
		if gid != 0 && gid != info2.Encoding[i] {
			t.Errorf("info1.Encoding[%d] != info2.Encoding[%d]: %d != %d", i, i, gid, info2.Encoding[i])
		}
	}

	q := 1000 / float64(info1.Font.UnitsPerEm)
	// Compare ascent, descent, capHeight in PDF units, since they come from
	// the PDF font descriptor:
	if math.Round(info1.Font.Ascent.AsFloat(q)) != math.Round(info2.Font.Ascent.AsFloat(q)) {
		t.Errorf("info1.Font.Ascent != info2.Font.Ascent: %f != %f", info1.Font.Ascent.AsFloat(q), info2.Font.Ascent.AsFloat(q))
	}
	if math.Round(info1.Font.Descent.AsFloat(q)) != math.Round(info2.Font.Descent.AsFloat(q)) {
		t.Errorf("info1.Font.Descent != info2.Font.Descent: %f != %f", info1.Font.Descent.AsFloat(q), info2.Font.Descent.AsFloat(q))
	}
	if math.Round(info1.Font.CapHeight.AsFloat(q)) != math.Round(info2.Font.CapHeight.AsFloat(q)) {
		t.Errorf("info1.Font.CapHeight != info2.Font.CapHeight: %f != %f", info1.Font.CapHeight.AsFloat(q), info2.Font.CapHeight.AsFloat(q))
	}

	for _, info := range []*EmbedInfoSimpleCFF{info1, info2} {
		info.Encoding = nil     // already compared above
		info.Font.Ascent = 0    // already compared above
		info.Font.Descent = 0   // already compared above
		info.Font.CapHeight = 0 // already compared above

		info.Font.Width = 0                      // "OS/2" table is optional
		info.Font.IsRegular = false              // "OS/2" table is optional
		info.Font.CodePageRange = 0              // "OS/2" table is optional
		info.Font.CreationTime = time.Time{}     // "head" table is optional
		info.Font.ModificationTime = time.Time{} // "head" table is optional
		info.Font.Description = ""               // "name" table is optional
		info.Font.Trademark = ""                 // "name" table is optional
		info.Font.License = ""                   // "name" table is optional
		info.Font.LicenseURL = ""                // "name" table is optional
		info.Font.PermUse = 0                    // "OS/2" table is optional
		info.Font.LineGap = 0                    // "OS/2" and "hmtx" tables are optional
		info.Font.XHeight = 0                    // "OS/2" table is optional

		info.Font.Outlines = nil // TODO(voss): reenable this (but cmp.Diff hangs)
	}

	if d := cmp.Diff(info1, info2); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}
