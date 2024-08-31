+++
title = 'Some design considerations for the font API'
date = 2024-08-31T23:13:27+01:00
draft = true
+++

Some design considerations for the font API
===========================================

Glyph ID Values
---------------

I normally address the glyphs in a font file by a Glyph ID (GID).
In Go, use the data type `seehuhn.de/go/sfnt/glyph.ID` for this,
which currently maps to `uint16`.  For Type1 and Type3 fonts,
I sort the glyph names, and use indices into this sorted list
as GIDs.  The GID 0 refers the the `.notdef` glyph (except for Type3 fonts,
where it does not refer to any glyph at all).

The map of GIDs to glyphs is fixed for any given font instance.

Character Codes
---------------

Inside content streams, the glyphs are addressed by a (single or multi-byte)
character code.  When typesetting new text, the library automatically allocates
codes to glyphs, and subsets fonts to the glyphs used.  In fonts extracted from
existing PDF files, the mapping between character codes and glyphs is fixed
in the font dictionary.

The map of character codes to glyphs can differ between PDF files, but is the
same for all content streams within a PDF file.

Typesetting
-----------

PDF fonts generated within the library contain all information required
for typesetting text.

Some of this information (leading, kerning tables, ligature maps, glyph
bounding boxes) is not always explicitly stored in PDF files, so may not
available for fonts extracted from PDF files.

Text Positioning
----------------

Some information is required to keep track of the current text position in
content streams.  This information is available for all fonts generated within
the library. Since this information is required for rendering PDF files, it is
also always available for fonts extracted from PDF files.

The required information is as follows:

  - glyph widths for each glyph.
    For fonts created within the library, this information is available
    for each GID within the font data.
    For fonts extracted from a PDF file, the glyph width for each
    character code is available in the font dictionary.

    Question: in this library, should glyph width be looked up by GID or by
    character code (or both)?

  - PDF writing mode (horizontal or vertical): This is "horizontal" for
    simple fonts, and defined in the "WMode" entry of the CMap for composite
    fonts.

  - For text extracted from a PDF file, the ability to split a PDF string
    into a sequence of character codes is also required.

Go API
------

The Go API needs to cater for the following use cases:

  - There must be font objects for typesetting text, which are not yet tied to
    a PDF file, so that fonts can be loaded ahead of time, and then used to
    generate multiple PDF files.

  - In order for the `pdf.ResourceManager` mechanism to work,
    fonts used for typesetting new text must embed the generic
    `pdf.Embedder[T]` interface, for some type `T`.
    The type `T` represents a font tied to a specific PDF file,
    and must be able to allocate character codes to glyphs and to keep
    track of which glyphs of the font are used in the PDF file.

  - The field `TextFont` in a PDF graphics state needs to be able to refer to
    fonts created within the library as well as to fonts extracted from a PDF
    file.

  - Fonts created within the library must expose a way to convert
    a Go string to a sequence of glyphs, taking kerning and ligature
    tables into account.  Fonts extracted from a PDF file do not
    expose this functionality.

The following table lists different operations on different types of fonts: A =
created font (users of the library), B = created font (PDF writer),
tied to a PDF file, C = extracted font (PDF reader).

| A | B | C | Operation
|---|---|---|-----------
| x |   |   | implement pdf.Embedder[T]
|   | x |   | implement pdf.Resource
| x | x | . | GID -> width
|   | . | x | character code -> width
| x | x | x | writing mode
| x | x | x | split PDF string into character codes
|   | x | x | GID <-> character code
| x | x |   | typesetting (ligatures, kerning, ...)
| x | x |   | glyphIsBlank()
