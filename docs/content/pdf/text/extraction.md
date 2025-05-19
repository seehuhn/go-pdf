+++
title = 'Text Extraction'
date = 2025-05-19T10:16:36+01:00
+++

This page discusses ideas for extracting text from PDF files.
In particular, it is concerned with the reconstruction of white space
and line breaks in the original document.

## Relevant PDF Operators

The following PDF operators may contain information about white space and line breaks:

- `BT`/`ET`: Begin/end text object.
  This operator indicates the start and end of a text object.
  It is not clear whether these groupings contain information about
  white space and line breaks.

- `Td`: Move to the start of the next line.
  This operator gives a strong hint that the current line is finished.

- `TD`: Similar to `Td`, but also sets the text leading.
  This operator gives a strong hint that the current line is finished,
  and may indicate that the previous line was the first line of a paragraph.

- `Tm`: Set the text line matrix and text matrix.
  This may indicate the start of a new paragraph/column.

- `T*`: Move to the start of the next line.
  This operator indicates that the current line is finished.

- `Tj`: Show text.
  The text may contain embedded spaces, which should be preserved.
  No additional spaces or line breaks should be added within the text.

- `'`: Move to the start of the next line and show text.
  This operator indicates that the current line is finished.
  The text may contain embedded spaces, which should be preserved.
  No additional spaces or line breaks should be added within the text.

- `"`: Similar to `'`, but also sets the word/character spacing.
  In addition to the notes for `'`, this operator may hint
  that spaces are explicitly encoded in the text.

- `TJ`: Show text with kerning.
  The text may contain embedded spaces, which should be preserved.
  Negative kerning may be used to indicate spaces.
  No line breaks should be added within the text.

- `Tr`: Set the text rendering mode.
  This operator may indicate that the text is not visible.

- `Ts`: Set the text rise.
  This changes the vertical position of the text, without indicating a line break.
