From the documentation, I am not quite sure how ranges in CMap files
are meant to work.  The code in this directory produces a PDF file
which tests the following special case:

```
1 begincodespacerange
<3030> <3232>
endcodespacerange

1 begincidrange
<3030> <3232> 34
endcidrange
```

Here, CID 34 is 'A' in the Adobe-Japan1 character collection.
The question is: Will this assign CIDs to all 9 possible codes?
Or just the codes <3030>, <3031>, <3032>?
Or is this invalid?

The PDF shows a 3x3 grid of glyphs, using the following codes:

```
<3030> <3031> <3032>
<3130> <3131> <3132>
<3230> <3231> <3232>
```

The font has the glyphs A to I at the CIDs 34 to 42, and the glyphs a to f
at the CIDs 290 to 292 and 546 to 548.  These are the CIDs which the second
and third row get if the codes in the range are counted as two-byte numbers.

Results (November 2026):
- Adobe Reader, Ghostscript: `ABC`, `DEF`, `GHI` (each byte varies independently)
- Firefox, Google Chrome, MuPDF, Poppler: `ABC`, `abc`, `def` (codes counted as two-byte numbers)
- macOS Preview: no codes are mapped
