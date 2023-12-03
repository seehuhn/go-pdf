PDF Graphics State Parameters
=============================

Notes
-----

Parameters are used in the following parts of this library:
- Setter functions of Page (e.g. SetLineWidth):  This
  (a) updates the graphics state, and
  (b) writes the corresponding PDF operator to the content stream.
- Reading/Writing `ExtGState`` objects:
  This needs to know the dictionary keys and encodings for each parameter.
- Applying ExtGState objects (gs operator):
  This updates the graphics state.
- Reading the content stream:
  This decodes the PDF operators and updates the graphics state.


List of all parameters
----------------------

|state bit                   | PDF operator       | Dict key       | Notes                             |
|----------------------------|--------------------|----------------|-----------------------------------|
|                            | cm                 | -              | CTM                               |
|                            | W, W*              |                | clipping path                     |
|StateStrokeColor            | ...                | -              | various operators                 |
|StateFillColor              | ...                | -              | various operators                 |
|StateTc                     | Tc                 | -              |                                   |
|StateTw                     | Tw                 | -              |                                   |
|StateTh                     | Tz                 | -              |                                   |
|StateTl                     | TL, TD, T*         | -              |                                   |
|StateFont                   | Tf                 | Font           |                                   |
|StateTmode                  | Tr                 | -              |                                   |
|StateTextRise               | Ts                 | -              |                                   |
|StateTextKnockout           | -                  | TK             |                                   |
|StateTm                     | Td, TD, Tm, T*, ...| -              | updated by text showing operators |
|StateTlm                    | Td, TD, Tm, T*     | -              |                                   |
|StateLineWidth              | w                  | LW             |                                   |
|StateLineCap                | J                  | LC             |                                   |
|StateLineJoin               | j                  | LJ             |                                   |
|StateMiterLimit             | M                  | ML             |                                   |
|StateDash                   | d                  | D              |                                   |
|StateRenderingIntent        | ri                 | RI             |                                   |
|StateStrokeAdjustment       | -                  | SA             |                                   |
|StateBlendMode              | -                  | BM             |                                   |
|StateSoftMask               | -                  | SMask          |                                   |
|StateStrokeAlpha            | -                  | CA             |                                   |
|StateFillAlpha              | -                  | ca             |                                   |
|StateAlphaSourceFlag        | -                  | AIS            |                                   |
|StateBlackPointCompensation | -                  | UseBlackPtComp |                                   |
|                            |                    |                |                                   |
|StateOverprint              | -                  | OP, op         |                                   |
|StateOverprintMode          | -                  | OPM            |                                   |
|StateBlackGeneration        | -                  | BG BG2         |                                   |
|StateUndercolorRemoval      | -                  | UCR, UCR2      |                                   |
|StateTransferFunction       | -                  | TR, TR2        |                                   |
|StateHalftone               | -                  | HT             |                                   |
|StateHalftoneOrigin         | -                  | HTO            |                                   |
|StateFlatnessTolerance      | i                  | FL             |                                   |
|StateSmoothnessTolerance    | -                  | SM             |                                   |
