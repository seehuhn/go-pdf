Character Encoding for TrueType Fonts
=====================================

This example generates a PDF file which can be used to explore how
a viewer maps character codes to glyphs for simple TrueType fonts.

Methods
-------

Given a single-byte code `c`, we consider the following method to map `c`
to a glyph in a TrueType font:

A. Use a (1,0) "cmap" subtable to map `c` to a glyph.

B. In a (3,0) "cmap" subtable, look up either `c`, `c+0xF000`, `c+0xF100`,
   or `c+0xF200` to get a glyph.

C. Use the encoding to map `c` to a name, use Mac OS Roman to map the name to
   a code, and use a (1,0) "cmap" subtable to map this code to a glyph.

D. Use the encoding to map `c` to a name, use the Adobe Glyph List to map the
   name to unicode, and use a (3,1) "cmap" subtable to map this character to a
   glyph.

E. Use the encoding to map `c` to a name, and use the "post" table to look
   up the glyph.

The program `ttf-encoding` constructs a sequence of TrueType fonts which allow
to determine which of these methods are supported in the viewer, and in which
order they are tried.


Results
-------

|           | Preview  |  Ghostscript  |  Acrobat Reader  |  FireFox  | Chrome  |
| :-------: | :------: | :-----------: | :--------------: | :-------: | :-----: |
| 2.0/+S/+E | DBC      |  BDA          |  DBA             |  BE       |  DBEA   | B
| 2.0/-S/+E | DBC      |  DC           |  DC              |  DC       |  DC     | C
| 2.0/+S/-E | BA       |  BA           |  BA              |  BA       |  BA     | BA
| 2.0/-S/-E | B        |  -            |  -               |  A        |  AB     | -
|           |          |               |                  |           |         |
| 1.2/+S/+E | DBC      |  BDA          |  DBA             |  BE       |  DBEA   |
| 1.2/-S/+E | DBC      |  DC           |  DC              |  DC       |  DC     |
| 1.2/+S/-E | BA       |  BA           |  BA              |  BA       |  BA     |
| 1.2/-S/-E | B        |  -            |  -               |  A        |  AB     |


Conclusions
-----------

- Good choices when writing a PDF file seem to be the following:
  - symbolic font, no encoding, use method B
  - non-symbolic font, encoding, use method D
- For readers, implementing the behaviour of Google Chrome seems a good choice.
- Existing viewers seem to ignore the PDF version.
- I found no evidence that Adobe Acrobat Reader implements method E.
