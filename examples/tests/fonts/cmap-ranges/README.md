From the documentation, I am not quite sure how ranges in CMap files
are meant to work.  The code in this directory produces a PDF file
which tests the following special case:

```
3 begincodespacerange
  <00> <01>
  <0200> <02FF>
  <03> <FF>
endcodespacerange

1 begincidrange
  <01> <03> <34>
endcidrange
```

Where CID 34 is 'A' in the Adobe-Japan1 character collection.
The question is: does <03> now map to 'B' (because <02> is not a valid code),
or to 'C' (because <03> is the third code in the range)?
