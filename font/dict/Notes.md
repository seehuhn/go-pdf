Design Notes
============

- The different structures contain the information of a PDF font dict
  and all the structures referenced from there (font descriptor, CMaps, etc.).
  The only exception is the font file, which is not included in the font dict.

- The representation is one-to-one, writing the structure to a PDF file
  and reading it back should give the same structure.

Fields
======

X = required, O = optional

| Name           | T0 | T1 | T2 | T3 | TT |
|----------------|----|----|----|----|----|
| Ref            | X  | X  | X  | X  | X  |
| PostScriptName | X  | X  | X  |    | X  |
| SubsetTag      | O  | O  | O  |    | O  |
| Name           |    | O  |    | O  | O  |
| Descriptor     | X  | X  | X  | O  | X  |
| ROS            | X  |    | X  |    |    |
| Encoding       |    | X  |    | X  | X  |
| CMap           | X  |    | X  |    |    |
| ...            |    |    |    |    |    |
| FontType       | X  | X  | X  |    | X  |
| FontRef        | X  | X  | X  |    | X  |
