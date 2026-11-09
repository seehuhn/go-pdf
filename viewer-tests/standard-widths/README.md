# Glyph-widths

For PDF versions until 1.7, the widths of the glyphs in the 14 standard fonts
can be omitted from the PDF file and must then be supplied by the PDF viewer.
Unfortunately, the PDF specification does not specify these widths, and the
values used by Adobe Acrobat Reader are not published.

The PDF file produced by this example can be used to test whether a
PDF viewer uses the same glyph widths as the go-pdf library.
It covers all 14 standard fonts.
Each row of the generated file contains a number of copies of a glyph,
followed by two black horizontal lines:
- The position of the first line on the background grid can be used to infer
  the glyph width used by the PDF viewer.
- If the second line is printed over the red background-line near the right
  margin, the viewer's glyph width is the same as the library's.  If the black
  line is to the left/right of the red line, the viewer's glyph width is
  smaller/larger than the library's.

The library uses the widths from Adobe's Core14 AFM files.  Adobe Acrobat
Reader 24.003 was found to agree with these, except for the following glyphs:

| font                  | glyph    | AFM width | viewer width |
|-----------------------|----------|----------:|-------------:|
| Helvetica             | mu       |       556 |          578 |
| Times-Roman           | mu       |       500 |          510 |
| Times-Bold            | mu       |       556 |          601 |
| Helvetica-Oblique     | fraction |       167 |          278 |
| Helvetica-BoldOblique | fraction |       167 |          278 |
