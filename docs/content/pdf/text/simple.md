+++
title = 'Simple Fonts'
date = 2024-09-04T14:03:48+01:00
draft = true
+++

# Simple Fonts

Each byte is interpreted as a code, and then mapped to a glyph via the
“encoding”.  The encoding sometimes is defined relative to the “built-in
encoding” of the font.

## Type 1 Fonts

The encoding addresses glyphs by name.  The map from codes to glyph names is
described by an encoding dictionary (Table 112 in ISO 32000-2:2020).  This
dictionary specifies a “base encoding”, which then can be modified by a
“differences array”.  The following values of the /BaseEncoding field are
recognised:

* /MacRomanEncoding \- described in Table D.2 of ISO 32000-2:2020
* /MacExpertEncoding \- described in Table D.4 of ISO 32000-2:2020
* /WinAnsiEncoding \- described in Table D.2 of ISO 32000-2:2020
* (absent) \- If the font is non-symbolic and not embedded and the PDF version
  is \>=1.3 this is the “standard encoding”, otherwise this is the font’s
  built-in encoding.

If no encoding dictionary is present, the font’s built-in encoding is used.

If the font does not contain a glyph with the selected name, the “.notdef”
glyph is shown instead.  Similarly, codes not listed in tables D.2 and D.4
should be mapped to the “.notdef” glyph (according to
[https://github.com/pdf-association/pdf-issues/issues/377](https://github.com/pdf-association/pdf-issues/issues/377)).

## Type 3 Fonts

The encoding addresses glyphs by name.  The map from codes to glyph names is
described by the “differences array” of the encoding dictionary.  If the font
does not contain a glyph with the selected name, nothing is shown.

## TrueType Fonts

There are complicated rules in section 9.6.5.4 (Encodings for TrueType fonts)
of ISO 32000-2:2020.
