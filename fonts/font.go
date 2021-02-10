package fonts

// Box represents a rectangle in the PDF coordinate space.
type Box struct {
	LLx, LLy, URx, URy float64
}

// IsPrint returns whether the glyph makes marks on the page.
func (box *Box) IsPrint() bool {
	return box.LLx != 0 || box.LLy != 0 || box.URx != 0 || box.URy != 0
}

// Font represents information about a PDF font.
type Font struct {
	FontName  string
	FullName  string
	CapHeight float64
	XHeight   float64
	Ascender  float64
	Descender float64
	Encoding  Encoding
	Width     map[byte]float64
	BBox      map[byte]*Box
	Ligatures map[GlyphPair]byte
	Kerning   map[GlyphPair]float64
}

// GlyphPair represents two consecutive glyphs, specified by a pair of
// character codes.  This is used for detecting ligatures and computing kerning
// information.
type GlyphPair [2]byte
