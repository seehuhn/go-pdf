PDF 32000-2:2020 §9.3.1 requires a content stream to set the font and
font size with a Tf operator before any text-showing operator runs.
The specification does not say what a conforming reader must do if Tf
is absent: it is undefined behaviour.

This file tests how different PDF viewers handle that case.  The page
contains two cells, each with a labelled rectangle, a crosshair at the
text origin set by Tm, and a horizontal baseline tick.

  - The TEST cell on the left contains a BT / Tm / Tj (Hello!) / ET
    sequence with no Tf anywhere in its content stream.

  - The CONTROL cell on the right contains the same string set with a
    valid Tf for Times-Roman 24pt.

Open `test.pdf` in the viewer under test and compare the cells.

Open questions for each viewer:

  - Does the test cell render any text at all?
  - If it does, what font and size does it use?
  - Where is the text positioned relative to the crosshair?
  - Does copy-pasting from the test cell expose any text?
  - Does searching the document for "Hello!" match the test cell?

Observed behaviour:

| Viewer | Renders test text? | Fallback font/size | Notes |
|--------|--------------------|--------------------|-------|
|        |                    |                    |       |
