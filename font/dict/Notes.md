Design Notes
============

- The different structures contain the information of a PDF font dict
  and all the structures referenced from there (font descriptor, CMaps, etc.).
  The representation is one-to-one, writing the structure to a PDF file
  and reading it back should give the same structure.
