# Hit testing under optional content

`go run .` writes `test.pdf`, whose annotations sit inside and outside the
optional content group "Notes layer":

| annotation | in the layer | what it is                                  |
|------------|--------------|---------------------------------------------|
| Text       | yes          | a comment, with a reply (`/IRT`) behind it  |
| Link       | yes          | blue framed, opens `example.com`            |
| Sound      | no           | a speaker icon, one second of a square wave |
| Link       | no           | gray framed, toggles the layer              |

Switch the layer off, either in the viewer's layers panel or with the gray
link, and the layered annotations must stop being *drawn* and stop
*reacting*: no link cursor over the blue frame, no activation on a click,
no popup on the comment.  Switch it back on and they work again.  The
speaker sits outside the layer, so it stays visible and clickable whatever
the layer does; only a hide-annotations setting removes it.  The gray link
always works.

L-shaped ticks drawn in the page content mark the corners of the two layered
rectangles.  They sit outside the layer, so they stay visible throughout and
give the pointer somewhere definite to aim once the annotations are gone —
without them, "no link cursor over the blue frame" cannot be told apart from
hovering in the wrong place.

## What the specification settles, and what it does not

Only the drawing is settled.  An annotation's `/OC` decides its visibility
"before the annotation is drawn", and "if it is determined to be invisible,
the annotation shall not be drawn" (§12.5.2) — interaction is not mentioned.
Compare the Hidden flag, which is explicit: "do not render the annotation or
allow it to interact with the user" (§12.5.3).  The nearest the specification
comes for optional content is a note: "Tools to select and move annotations
need to honour the current on-screen visibility of annotations when
performing cursor tracking and mouse-click processing" (§8.11.3.1, NOTE 2),
which is informative rather than normative.

So the expectation above is what a reasonable viewer does, not what it is
required to do.  The failure shape to watch for is a viewer that computes its
clickable regions once when the file is opened: the hidden annotations keep
their cursor and stay clickable while invisible, which is how an invisible
link becomes a way to send a reader somewhere they cannot see they are going.

The comment sidebar is a genuinely open question rather than a defect either
way.  §8.11.3.1 leaves it to the processor, which "may choose whether to use
the document's current state of optional content groups ... or to supply
their own", so a sidebar may keep listing a hidden comment or may drop it.
Record which it does.

The layer is ON to start with, and listed in the default configuration's
`/Order`, without which it would not appear in the layers panel at all
(§8.11.4.3).

## Observed behaviour

With the layer off, one row per viewer:

| viewer | comment drawn? | comment reacts? | link drawn? | link reacts? | sidebar lists comment? |
|--------|----------------|-----------------|-------------|--------------|------------------------|
|        |                |                 |             |              |                        |

Quire draws neither the comment nor the blue link while the group is off
(`pdf-render -ocg-off "Notes layer" test.pdf`), and keeps the speaker and the
toggle.  Its hit testing lives in the viewers, not in `go-render`, so the
reacting columns have to be filled in from macQuire and lnxQuire by hand.
