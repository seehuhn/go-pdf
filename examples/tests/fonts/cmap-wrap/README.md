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
Or just the CIDs <3030>, <3031>, <3032>?
Or is this invalid?

The PDF shows a 3x3 grid of glyps, using the following codes:

```
<3030> <3031> <3032>
<3130> <3131> <3132>
<3230> <3231> <3232>
```

Results (September 2024):
- Adobe Reader, Ghostscript: all codes are mapped
- Firefox, Google Chrome: only the first row is mapped
- MacOS Preview: no codes are mapped
