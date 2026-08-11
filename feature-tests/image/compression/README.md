# Image compression

`go run .` writes two files:

- `test.pdf` — one image per (predictor, colour channels, bit depth)
  combination, eight predictors at a time.  Each group of eight should look
  identical; a group which does not is worth investigating.
- `data.csv` — the compressed length of every image, for comparing how well
  the predictors do on identical input.  It is regenerated on every run and
  is not committed.

`plot.R` draws `data.csv`, normalising each (channels, depth) group by its
smallest entry.  Run the program first.

## Reading the numbers

The test image is a high-frequency ripple, chosen so that the images differ
visibly when a predictor is decoded wrongly.  Differencing does not help on
data like that, so no predictor at all comes out smallest in 18 of the 20
groups.  The figures compare the predictors against each other on one
synthetic image; they do not say which predictor to use on real images.

Predictor 13 (PNG Average) is the largest of the seven fixed predictors in
all twelve groups below 8 bits per component, by up to a factor of two.
It is the only filter
which predicts from two neighbours at once, and `(left + up) / 2` is usually
not a value the image contains, so its residuals scatter over a far wider
alphabet: at three channels and 2 bits, 175 distinct byte values and 4.99
bits of entropy per byte, against 41 values and 2.88 bits in the raw data.
Sub, Up and Paeth each subtract an actual neighbouring byte, so a repeat
gives an exact zero.  At 8 and 16 bits the effect is gone, and predictor 14
(Paeth) is the smallest of the four differencing filters in every group.

Predictor 2 (TIFF horizontal differencing) is mid-field, third to sixth
smallest of the eight in every group.  At bit depths other than 1 and 8 it
is rare in practice, and some viewers have bugs in this area.

Predictor 15 (PNG Optimum) picks a filter per row, keeping the one whose
residuals have the lowest entropy.  Entropy counts the bits the Huffman stage
of the Flate filter needs, but says nothing about the matches its LZ77 stage
finds, so the score misses everything a repeat across rows would have saved.
On an image this close to noise that is the whole story: predictor 15 is not
the smallest of the six PNG predictors in any of the 20 groups, and at one
channel and 2 bits it is 32% larger than no filtering at all.

The library therefore does not reach for a predictor everywhere.  Per-row
selection earns its keep on images with 8 or 16 bits per component, but only
on the smoothly varying content real pictures hold, which the ripple
deliberately is not: on such data predictor 15 lands 30% to 45% below no
prediction, and this file cannot show it.  Below 8 bits a byte holds several
samples, and for an indexed colour space a sample is a palette position
rather than a brightness; in both cases prediction costs more than it saves,
and `graphics/image` writes the data unpredicted.  The pages here still cover
every combination, because the point of the file is to check that each one
decodes correctly.
