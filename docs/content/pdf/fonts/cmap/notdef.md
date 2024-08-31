+++
title = 'Notdef'
date = 2024-08-31T22:31:54+01:00
draft = true
+++

# Interpreting Character Codes in PDF Strings

## Composite Fonts

*\[The following information is from section 9.7.6.3 (Handling undefined characters) of ISO 32000-2:2020.\]*

Valid codes are defined by the codespace ranges in the CMap.  If a code is invalid, the following things happen:

1. CID 0 is returned.
2. The number of input bytes consumed is the length of the shortest code which contains the longest possible prefix of the given code.  In particular, if no valid code starts with the first byte of the given code, the number of bytes consumed is the length of the shortest code.

If the code is valid, maps to a CID, and a glyph exists for this CID, the CID is returned.

Otherwise, the notdef mappings are consulted.  If there is a notdef mapping for the code, the corresponding CID is considered: if the font contains a glyph for this CID, this CID is returned.

Otherwise CID 0 is returned.

## Simple Fonts

Each byte is interpreted as a code, and then mapped to a glyph via the “encoding”.  The encoding sometimes is defined relative to the “built-in encoding” of the font.

### Type 1 Fonts

The encoding addresses glyphs by name.  The map from codes to glyph names is described by an encoding dictionary (Table 112 in ISO 32000-2:2020).  This dictionary specifies a “base encoding”, which then can be modified by a “differences array”.  The following values of the /BaseEncoding field are recognised:

* /MacRomanEncoding \- described in Table D.2 of ISO 32000-2:2020
* /MacExpertEncoding \- described in Table D.4 of ISO 32000-2:2020
* /WinAnsiEncoding \- described in Table D.2 of ISO 32000-2:2020
* (absent) \- If the font is non-symbolic and not embedded and the PDF version is \>=1.3 this is the “standard encoding”, otherwise this is the font’s built-in encoding.

If no encoding dictionary is present, the font’s built-in encoding is used.

If the font does not contain a glyph with the selected name, the “.notdef” glyph is shown instead.  Similarly, codes not listed in tables D.2 and D.4 should be mapped to the “.notdef” glyph (according to [https://github.com/pdf-association/pdf-issues/issues/377](https://github.com/pdf-association/pdf-issues/issues/377)).

### Type 3 Fonts

The encoding addresses glyphs by name.  The map from codes to glyph names is described by the “differences array” of the encoding dictionary.  If the font does not contain a glyph with the selected name, nothing is shown.

### TrueType Fonts

There are complicated rules in section 9.6.5.4 (Encodings for TrueType fonts) of ISO 32000-2:2020.
