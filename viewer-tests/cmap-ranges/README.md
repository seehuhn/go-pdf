From the documentation, I am not quite sure how ranges in CMap files
are meant to work.  The code in this directory produces a PDF file
which tests the following special case:

```
3 begincodespacerange
  <00> <41>
  <4200> <42FF>
  <43> <FF>
endcodespacerange

1 begincidrange
  <41> <43> 34
endcidrange
```

Where CID 34 is 'A' in the Adobe-Japan1 character collection.
The question is: does `<43>` now map to 'B' (because `<02>` is not a valid code),
or to 'C' (because `<43>` is the third code in the range)?

The PDF file in this example shows two glyphs, with codes `<41>` and `<43>`.
The first glyph should be 'A'. The second one is the glyph encoded by `<43>`.

Result (Sept 2024):
Most viewers I tried showed 'A C'.
