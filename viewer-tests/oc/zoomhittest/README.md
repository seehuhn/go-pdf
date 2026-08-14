# Hit testing under zoom-dependent optional content

`go run .` writes `test.pdf`: one framed link in an optional content group
whose usage dictionary carries `/Zoom << /min 2 /max 4 >>`, applied through a
`View` usage application in the default configuration's `/AS` array.

The group is ON when the magnification is "greater than or equal to min and
less than max" (§8.11.4.4), so the frame is drawn exactly on [200%, 400%).
Zoom across both boundaries.  Where the frame is drawn the link must show the
pointing hand and activate on a click; where it is not, the area must be empty
and dead.

L-shaped ticks drawn in the page content mark the corners of the link's
rectangle.  They sit outside the group, so they stay visible at every
magnification and give the pointer somewhere definite to aim while the frame
is gone.

When the usage application is re-evaluated is not settled.  §8.11.4.4 has the
usage settings applied "during viewing" and reads the Zoom category against
"the current magnification level", which points at re-evaluating as the
magnification moves, but nothing there says when an `/AS` entry is applied.
So zoom without reopening, and treat a frame that never moves as its own
result rather than as a wrong answer.

The group has no `/Order` entry, and so should not appear in the layers panel:
switching it by hand would count as a user override and stop the zoom from
driving it.  A viewer that lists it anyway is worth recording.

## Three failure shapes

**Open-time evaluation only.** The frame stays as the magnification at open
time left it, however far the zoom then moves.  Since the specification never
says when `/AS` is applied, this is a result to record rather than a defect to
report.

**Cached clickable regions.** The frame appears and disappears with the zoom,
but the cursor and the click do not — the viewer decided what was clickable
once, when the file was opened.  This is the same defect as in `../hittest`,
reached without touching a layers panel.

**Tile-scale evaluation.** A viewer that renders in tiles at power-of-two
levels may evaluate the usage dictionary at the tile scale rather than the
displayed magnification.  Near a boundary the pixels then disagree with the
cursor: the frame is drawn from a tile rendered at one level while hit testing
uses another.  Approach 200% and 400% slowly from both sides.

## A confound to note

The probe cannot by itself separate "the viewer ignores `/Zoom` usage" from
"this viewer's 100% is not magnification factor 1" — a difference that HiDPI
displays and screen-DPI corrections both produce.  If the frame never appears
at any zoom, the viewer is either ignoring the usage or reading it only at
open time; opening the file with the viewer already at 300% separates the
two.  If the frame appears but the boundaries sit somewhere other than the
reported 200% and 400%, record the magnifications at which it switches rather
than reading it as a failure.
Tiling the zoom axis with two more groups (`/max 2` and `/min 4`) would make
the boundaries self-diagnosing; that is not done here.

## Observed behaviour

One row per viewer.  For the three zoom bands, record whether the frame is
drawn; for the last column, whether the cursor and the click agree with it.

| viewer | below 200% | 200–400% | above 400% | cursor follows the frame? | switches at |
|--------|------------|----------|------------|---------------------------|-------------|
|        |            |          |            |                           |             |
