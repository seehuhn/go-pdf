Overview
========

This program creates a test file to check whether PDF viewers make a difference
between Type 1 font dictionaries with no encoding dictionary and Type 1 font
dictionaries with an empty encoding dictionary, when external (not embedded)
fonts are used.

Mechanism
=========

The test string in the generated file "test.pdf" consists of three
codes: 'X', 'N', and 'O', mapped to the glyphs with the same names.
The generated font contains only the following glyphs:

  - "B": glyph name, code 'X' in the built-in encoding
  - "I": glyph name, code 'N' in the built-in encoding
  - "E": glyph name, code 'O' in the built-in encoding
  - "S": glyph name "X"
  - "T": glyph name "N"
  - "D": glyph name "O"

(plus the ".notdef" glyph).  The effect of this is that the test
string is displayed differently, depending on what the viewer does:

  - "XNO" indicates that the viewer did not load the external font
  - "BIE" indicates that the viewer loaded the font and used the
    built-in encoding.
  - "STD" indicates that the viewer loaded the font and used the
    standard encoding.

Font Installation
=================

On MacOS 15.3.1 I managed to install the font as follows:

- For GPL Ghostscript 10.04.0, I copied the font files "test.pfb" and
  "test.afm" into `$HOME/Library/Fonts/` .  The font was picked up from there
  without trouble.

- For Adobe Acrobat Reader 2024.003.20180, I copied the font files "test.pfb"
  and "test.afm" into `/Library/Application Support/Adobe/Fonts/`.
  (I had to create the `Fonts` directory myself.)
  After restarting the application, the glyphs were missing at first,
  but after a few seconds they appeared.  On following starts of the
  application, the glyphs were displayed immediately.

  (Suggestion from https://discussions.apple.com/thread/3874607 .)

  I also tried `$HOME/Library/Application Support/Adobe/Fonts/`, but
  this did not work.
