# Annotations which hold their size

`go run .` writes `test.pdf`, a four-page file for checking annotations which
keep their size whatever the magnification: those flagged NoZoom (§12.5.3)
and text annotations, which carry the flag implicitly (§12.5.6.4).

Two things are under test.

**Size.** Zoom in and out.  The red boxes and the icons should stay the same
size on screen; the blue box beside the first red one is the control and
should grow and shrink with the page.

**Anchor.** Such an annotation is placed so that the upper-left corner of its
rectangle *in default user space* does not move.  Each box has a white notch
cut out of that corner of its appearance, and a black cross is drawn in the
page content at the point the corner should hold.  Notch and cross should
coincide at every magnification.  The icons have a cross each as well.

The four pages carry the four values of `/Rotate`.  Each is laid out
pre-rotated, so the text reads upright once the viewer applies the rotation.
Rotation separates the two flags of §12.5.3:

- a **NoZoom** annotation turns with the page, so the anchored corner — and
  with it the notch and the cross — lands on a different corner of the box on
  screen under each rotation;
- a **NoRotate** annotation stays upright, so both stay at its top left
  throughout.

| page | `/Rotate` | notch and cross, on screen |
|------|-----------|----------------------------|
| 1    | 0         | top left                   |
| 2    | 90        | top right                  |
| 3    | 180       | bottom right               |
| 4    | 270       | bottom left                |

Text annotations carry both flags, so they follow the second rule.  The
NoRotate box is their control: if it and the icons disagree, the viewer
treats the implicit flags of §12.5.6.4 differently from the explicit ones.
A viewer which anchors the on-screen upper-left corner of a NoZoom annotation
instead will drift as you zoom.

The rows are:

- a **Square** with `/F NoZoom` and the control beside it,
- a **Link** with `/F NoZoom`, which is not a markup annotation and so
  survives a viewer's hide-markup setting where the square does not, and
  beside it a **Square** with `/F NoZoom NoRotate`,
- five **Text** annotations, one per standard icon.
