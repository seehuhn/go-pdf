Overview
========

This program creates a test file to check whether an external (not
embedded) composite font can change the position of the glyphs on a page.

The glyph widths of a composite font are given in the PDF file, so the
layout should not depend on the font which is installed.  The exception
tested here is a code whose CID has no glyph in the font, while the CMap
gives a notdef CID for the code.  The notdef glyph is then shown, and the
specification does not state whether the width of the original CID or the
width of the notdef CID applies.  A viewer which uses the width of the
notdef CID only makes this choice once it knows that the glyph is missing,
and for an external font this depends on the installed font file.

Mechanism
=========

The program writes "test.pdf" and the two font files "notdef-present.otf"
(font name `Test28411P`) and "notdef-missing.otf" (font name `Test28411M`).
Both fonts are CID-keyed and contain glyphs for the CIDs 0 to 3.  The first
font also contains a glyph for CID 4.

The page uses the two fonts as external fonts.  The two font
dictionaries have the same CMap and the same widths, and differ only in the
name of the font.  The CMap maps the code 'E' to CID 4 and gives CID 1 as
the notdef CID for this code.  The widths of CID 4 and CID 1 differ.

For each font the page shows one row of circles, drawn as in
`../notdef-width`: a cross in a circle on the left shows which glyph the
viewer used for the code 'E', and a cross in a circle on the right shows
which width.  A row without crosses means that the font was not loaded.
If the two rows have the cross on the right in different columns, the
installed font has changed the layout, although the PDF data for the two
rows differs only in the name of the font.

`../notdef-width` shows how a viewer treats the same case for an embedded
font.

Font Installation
=================

Both font files must be installed before the file is opened.
`../external-font/README.md` describes where Ghostscript and Adobe Acrobat
Reader find external fonts on macOS.
