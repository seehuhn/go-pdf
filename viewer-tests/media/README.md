# Rendition actions

Two `go run .` programs write a `test.pdf` driving a 30-second 320x240
timestamp-counter clip (with a per-second audio tick) through a Screen
annotation (§13.4.5) and Rendition actions (§12.6.4.10).  Clicking the
screen plays the movie in place with the player's controller UI; a row of
buttons below exercises the operation codes — Play (0), Pause (2), Resume
(3), Stop (1), Play/Resume (4) — all targeting the screen annotation via
the action's `/AN` entry.  The timestamp and ticks let you judge whether
playback is smooth, in sync, and whether each button has an effect.

## The variants

- **embedded/** — the movie is embedded in the PDF as an `/EF` file
  specification inside the rendition's media clip.
- **external/** — the media clip names `movie.mp4` relative to the PDF,
  with no embedded data.  The committed symlink `movie.mp4` points at the
  shared copy in `../internal/testmovie`, so opening `test.pdf` from
  within this directory gives the viewer a fair chance of resolving it.

Renditions are sparsely implemented outside Adobe Acrobat: many viewers
ignore Screen annotations entirely, or play only embedded data.  The point
of the survey is to record which viewers support renditions at all, which
operations they honour, and whether they resolve the **external/**
variant.  The sibling `../movie/` tests probe the deprecated Movie
annotation with the same two delivery modes.

The clip itself lives once in `../internal/testmovie`; regenerate it there
with `go generate` (needs ffmpeg).
