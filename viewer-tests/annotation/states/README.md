# Appearance states

`go run .` writes `test.pdf`: three annotations, each carrying all three
appearances of §12.5.5 — normal (`/N`), rollover (`/R`, shown while the
pointer is over the annotation) and down (`/D`, shown while the mouse button
is held down inside it).

The three appearances paint **disjoint** shapes inside the same bounding box:

| state | shape                | position     |
|-------|----------------------|--------------|
| `/N`  | blue square          | left third   |
| `/R`  | orange circle        | middle third |
| `/D`  | green triangle       | right third  |

so a state cannot hide the state it replaces.  Exactly one shape should be
visible at any moment.  Two or three at once mean the viewer paints a state
*over* its predecessor instead of *in place of* it: §12.5.5 says each
appearance shall be used in its own situation, which leaves no room for
compositing.  A dashed grey rectangle drawn in the page content marks each
annotation rectangle and stays visible throughout.

## The down appearance needs a highlighting mode

A widget's `/H` entry decides what happens while the mouse button is held
down, and "a highlighting mode other than P shall override any down
appearance defined for the annotation" (§12.5.6.19).  Its default is `I`
(invert).  So a widget carrying `/D` but no `/H` should show no down
appearance at all, and invert the rectangle instead.  The link and the
widget here therefore set `/H P`.

Only links (§12.5.6.5) and widgets (§12.5.6.19) have an `/H` entry at all.
A FreeText has none, and the link's own `/H P` is described as a push-down
effect without mentioning `/D`, so only the widget row is guaranteed a down
appearance by the spec.

## The rows

They are a Link, a FreeText and a push-button Widget, which differ in
how much of `/AP` a viewer honours:

- **Link** — the least likely to be honoured: a viewer may draw the link
  from its border and `/H` alone and ignore `/AP` altogether.
- **FreeText** — a markup annotation, so a viewer's hide-markup setting
  removes it.  Locked (`/F` Locked + LockedContents) so that editing cannot
  regenerate `/N` and discard the other two appearances.
- **Push button** — the usual consumer of `/D`.

Quire renders `/N` and `/R`, and does not implement `/D` at all: `go-render`
resolves only those two appearance kinds.  The down row is here for other
viewers, and as a target.
