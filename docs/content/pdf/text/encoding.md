+++
title = 'Encoding for Simple Fonts'
date = 2024-12-11T10:43:18Z
weight = 20
+++

# Encoding for Simple Fonts

For simple fonts, the interpretation of character codes in a PDF contents
stream is provided by an "encoding", which maps character codes to glyph names.

## Semantics

For each code, 0 to 255, the encoding can specify one of the following:

  - A glyph name to use for this code.

  - An indication that the font's built-in encoding should be used.  In this
    case, the code is mapped to a glyph name using the font's built-in
    encoding.

If a code maps to a glyph name, either directly or via the built-in
encoding, and if the font contains a glyph with the given name, this glyph is
shown.

If a code maps to a glyph name, but the font does not contain a glyph with the
given name, the `.notdef` glyph is shown.  (TODO(voss): what are the rules for
TrueType fonts?)

In case a predefined encoding is used but the code is not listed in the
corresponding tables in appendix D of ISO 32000-2:2020, a code may not
map to any glyph name.  According to
[PDF Issue 377](https://github.com/pdf-association/pdf-issues/issues/377#issuecomment-2097639506),
the `.notdef` glyph should be shown in this case.

## Type 1 Fonts

The encoding for a font is specified by the `/Encoding` field of the font
dictionary.  This field is interpreted as follows.   (See Table 112 in ISO
32000-2:2020 for details.)

  - If the `/Encoding` field is absent, the font’s built-in encoding is used.

  - If the `/Encoding` field is one of the following names, the corresponding
    predefined encoding is used:

    - `/MacRomanEncoding` (Table D.2 in ISO 32000-2:2020)
    - `/MacExpertEncoding` (Table D.4 in ISO 32000-2:2020)
    - `/WinAnsiEncoding` (Table D.2 in ISO 32000-2:2020)

  - If the `/Encoding` field is a dictionary, this dictionary specifies a "base
    encoding", which can then be modified using a "differences array".
    The format of an encoding dictionary is as follows:

      - If `/Type` is present, it must be `/Encoding`.

      - If the the `/BaseEncoding` field is present, it must be a one of the
        following names.  The "base encoding" is then the corresponding
        predefined encoding:

          - `/MacRomanEncoding` (Table D.2 in ISO 32000-2:2020)
          - `/MacExpertEncoding` (Table D.4 in ISO 32000-2:2020)
          - `/WinAnsiEncoding` (Table D.2 in ISO 32000-2:2020)

        If the `/BaseEncoding` field is absent, there are two cases:

          - If the font is embedded in the PDF file or the font is marked as
            symbolic in the font descriptor or the PDF version is <1.3, the
            "base encoding" is the font's built-in encoding.

          - If the font is not embedded in the PDF file and the font is marked
            as non-symbolic in the font descriptor and the PDF version is
            \>=1.3, the "base encoding" is the "standard encoding" (Table D.2
            in ISO 32000-2:2020).

        Note that an absent `/BaseEncoding` is interpreted slightly
        differently to an absent `/Encoding` field.

      - If the `/Differences` field is present, it describes the differences
        between the "base encoding" and the final encoding.  Otherwise the
        final encoding equals the "base encoding".  The differences array
        contains one or more sequences, each consisting of a character code
        followed by one or more glyph names.  The encoding is modified to map
        the code to the first glyph name in the sequence, and the subsequent
        codes to the subsequent glyph names.

## TrueType Fonts

The `Encoding` entry has the same format as for Type 1 font dictionaries,
but with some restrictions.
- Sometimes between PDF-1.4 and PDF-1.7, use of of `MacExpertEncoding`
  was disallowed or at least discouraged.
- Use of the `Differences` array is discouraged.

If a code is not otherwise specified, sometimes the Standard Encoding is used
as a fallback.  TrueType fonts don't have a built-in encoding, and normally
don't use glyph names.  Instead, the spec describes a variety of mechanisms to
map a single-byte code `c` to a glyph in a TrueType font. An overview can be
found on the
[page about glyph selection]({{< relref "glyph-selection.md#truetype-fonts" >}}).

## Type 3 Fonts

There is no base encoding, and not .notdef glyph.  The encoding is entirely
specified using a `/Difference` array within an encoding dictionary.
