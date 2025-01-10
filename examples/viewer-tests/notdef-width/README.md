The CMAP in CIDFonts allows to specify an alternative glyph which is
to be used when original glyphs in a given range of character codes
is missing from the font.  From my reading of the spec it is not clear
whether in this case the glyph width of the original glyph or of the
alternative glyph should be used.

The code here constructs a test file which uses this mechanism to display
an alternative glyph for a missing glyph.  The position of the following
glyph allows to see which choice of width is made by the viewer.
