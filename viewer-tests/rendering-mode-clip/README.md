Text rendering modes 4 to 7 intersect the clipping region with the
outlines of the text shown in a text object.  Special rules apply when
no glyph with an outline is shown, and to Type 3 fonts.  Viewers
disagree on these cases.

The code here constructs a test file with one row per case.  Each row
runs a text object in a clipping mode and then paints a blue bar from
the glyph origin to the right.  The Type 3 glyph declares a bounding
box larger than the area it paints, so the label under the right end
of the bar says whether the viewer clips to the painted marks, to the
declared bounding box, or not at all; no bar means the viewer clipped
to an empty path.  Black above and below the bar means the glyph was
painted.  Case 0 shows the same box from a Type 1 font built at run
time, as the regular case.

Results so far:

| viewer         | case 1 (space) | case 2 (no text) | case 3 (mode 7, Type 3) | case 4 (mode 4, Type 3) |
|----------------|----------------|------------------|-------------------------|-------------------------|
| Quire          | no clip        | no clip          | no clip, not painted    | no clip, painted        |
| Ghostscript    | empty clip     | empty clip       | empty clip, painted     | empty clip, painted     |
| poppler        | empty clip     | no clip          | no clip, painted        | no clip, painted        |
| Acrobat Reader | no clip        | not tested       | marks or bbox (*)       | marks or bbox (*)       |

Ghostscript 10.05.1, poppler 25.03.0 (pdftoppm).

(*) seen with an earlier version of the file, whose glyph painted its
whole bounding box, so marks and bbox could not be told apart, and it
did not show whether case 4 painted the glyph.
