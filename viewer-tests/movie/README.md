# Movie annotations

Two `go run .` programs write a `test.pdf` playing a 30-second 320x240
timestamp-counter clip (with a per-second audio tick) through the deprecated
Movie annotation (§13.4.5, removed in PDF 2.0), using the default
activation parameters: clicking the annotation rectangle plays the movie in
place.

## The variants

- **embedded/** — the movie is embedded in the PDF as an `/EF` file
  specification; every conformant viewer has the bytes.
- **external/** — the file specification names `movie.mp4` relative to the
  PDF, with no embedded data.  The committed symlink `movie.mp4` points at
  the shared copy in `../internal/testmovie`, so opening `test.pdf` from
  within this directory gives the viewer a fair chance of resolving it.

Movie support is rare outside Adobe's viewers; the point of the survey is
to record which viewers still honour the annotation at all, and whether
they resolve the **external/** variant.  The sibling `../media/` tests
probe the same two delivery modes through Screen annotations and Rendition
actions, which give finer control over playback.

The clip itself lives once in `../internal/testmovie`; regenerate it there
with `go generate` (needs ffmpeg).
