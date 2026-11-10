# JBIG2 Test Data

Test vectors for the JBIG2 decoder and encoder.  All images in this
directory are original to this project: they are either synthetic
geometric patterns or a page of text written for this project, set in the
Go fonts and rendered by Ghostscript.  No sample images from ITU-T T.88 or
other third parties are included.

## Directory Structure

```
testdata/
  README.md          -- this file
  mq/                -- low-level arithmetic coder test vectors
    mq_test_vectors.txt
  decode/            -- segment-level decode test cases
    <name>.globals   -- JBIG2Globals stream (may be 0 bytes)
    <name>.page      -- JBIG2 page stream (image XObject data)
    <name>.bmp       -- expected decoded bitmap (1-bit BMP)
    <name>.json      -- segment manifest (types, sizes, structure)
  fuzz/              -- seed corpus of the Go fuzz tests
```

## Decoder Tests (decode/)

Each test case has four files sharing a base name.  `TestDecodeAllTestCases`
decodes `<name>.globals` (the JBIG2Globals stream; 0 bytes if there are no
global segments) together with `<name>.page` and compares the result pixel
by pixel against `<name>.bmp`.  The `.json` file lists the segments, which
helps with debugging; for generated cases it also records the provenance.

All streams use the PDF embedded organisation (ISO 32000-2, section
7.4.7): no file header and 1-byte page association fields.

Every stream was produced by an encoder independent of this package, and
every `.bmp` file was checked with a decoder independent of this package:
it is the output of jbig2dec for that stream, except for
`test_t88_gen_ext` (see below).  For the lossless cases, the generator
also checks that the decoders reproduce the encoder input exactly.

### Generated text-page cases

These cases are written by the program in `../generate`, which is run by
`go generate` in the package directory.  The program sets a page of text
written for this project (in `generate/source.go`) in the Go fonts, writes
it as a PDF file with this library, renders it with Ghostscript at 200 dpi
without anti-aliasing, and feeds the page, or a crop of it, to the encoder.
It then parses the encoder output and makes the header edits listed below;
all other header fields and the entropy-coded data are kept unchanged.  The
files in `decode/` are written only once every case has succeeded.

| Test Name | Input | Encoder | Header edits |
|-----------|-------|---------|--------------|
| test_enc_page_generic | full page, 1700x2200 | jbig2enc `-p` | region type 39, XOR, end of page |
| test_g4_page_mmr | full page, 1700x2200 | libtiff `ppm2tiff -c g4` | all JBIG2 headers |
| test_enc_text_tpgd | crop, 800x400 | jbig2enc `-p -d` | none |
| test_enc_text_defpixel | crop, 320x96 | jbig2enc `-p` | default pixel 1, XOR |
| test_enc_text_symbol_local | crop, 1000x360 | jbig2enc `-s` | page-local dictionary, region at (4,1) |

- In `test_g4_page_mmr`, the Group 4 strip of the TIFF file (min-is-white,
  MSB first, ending with EOFB) is used unchanged as the MMR data of an
  immediate lossless generic region, framed by a page information segment
  and an end of page segment written by the generator.
- In `test_enc_text_defpixel`, the page default pixel is 1 and the region
  is combined with XOR, so the expected page is the inverted crop.
- In `test_enc_text_symbol_local`, the file header of jbig2enc's
  stand-alone output is removed, the symbol dictionary is moved from page 0
  to page 1, and the page is enlarged so that the text region can be placed
  at (4,1).  Symbol coding is lossy, so the expected bitmap is the jbig2dec
  output.
- Where XOR is used, the page information segment sets the combination
  operator override flag.

The data were produced with jbig2enc 0.32 (leptonica 1.87.0), libtiff 4.7.2,
Ghostscript 10.08.0 and checked with jbig2dec 0.20; the generator warns if
the installed versions differ.  Regeneration with these versions is
byte-stable.

### T.88 sample software cases

The `test_t88_gen_*` cases cover generic region templates 1-3, AT pixels
away from their nominal positions and the extended template, for which no
other encoder is available.  Each stream was produced by the encoder of the
ITU-T T.88 sample software from a crop of the generated text page, so the
images are original to this project; the generator writes the parameter
files itself.  The generator makes these edits, each recorded in the
`.json` file:

- The file header is removed.
- The page information segment gets the combination operator override
  flag, since the encoder combines the region with XOR while the page
  default is OR (T.88, 7.4.8.5).
- The arithmetic-coded data are completed with the terminating marker
  0xFF 0xAC.  T.88 FLUSH (Figure E.11) ends the data with this marker, but
  the sample encoder uses a different flush, which shifts out the
  remaining bits of the C register and writes no marker, so that its data
  end either in 0xFF or in some other byte.  The generator appends 0xAC
  in the first case and 0xFF 0xAC in the second.  This does not change
  the decoded image: on meeting the marker, BYTEIN (Figure E.19) adds
  0xFF00 to C, and reading the appended 0xFF as data adds the same, so
  the decoder proceeds exactly as if the original data were padded with
  1-bits.  Without the marker, the stream does not conform to T.88 and
  jbig2dec warns about it.  The decoded images are still compared with
  the encoder input, and `TestGenericDecodeStreamEnd` checks that our
  decoder gives the same image for all three endings.

| Test Name | Input crop | Template | TPGDON | AT pixels |
|-----------|------------|----------|--------|-----------|
| test_t88_gen_t0_at | 400x128 | 0 | 0 | (4,-1) (-4,-1) (3,-2) (-3,-2) |
| test_t88_gen_t1 | 320x112 | 1 | 0 | (-3,-1) |
| test_t88_gen_t2 | 320x96 | 2 | 0 | (2,-2) |
| test_t88_gen_t3 | 320x96 | 3 | 0 | (-1,-2) |
| test_t88_gen_ext | 400x128 | 0, extended | 0 | 12 pixels, see the `.json` file |

All are lossless.  The generator checks that both jbig2dec and the T.88
sample decoder reproduce the encoder input exactly from the edited stream,
as committed; since the sample decoder reads only the file organisation,
the generator puts a file header in front of the stream for it.  jbig2dec
0.20 does not implement the extended template, so `test_t88_gen_ext` is
checked with the T.88 sample decoder only.

The sample software cannot be installed from a package manager.  It is in
the ITU-T T.88 (08/2018) software package
`T-REC-T.88-201808-I!!SOFT-ZST-E.zip` (sha256
`e2a3c458c4d28d5a835c0815fd9b23e197dbc1b83fe81d8e8d8b2ac413d8f22f`), as
`Software/JBIG2_SampleSoftware-A20180829.zip` (sha256
`c9e7fc74d4792653de258be4f77c05d639ced4518dd57cc5a6ddfda541e2d181`).
Build it with `make jbig2` in its `source` directory and run

```sh
T88_JBIG2=/path/to/source/jbig2 go generate
```

in the package directory.  Without `T88_JBIG2`, the generator skips these
cases and leaves their files unchanged.

The sample encoder does not implement typical prediction: with TPGDON=1
it sets the flag in the segment but codes no pixels (the TPGDON branch of
`MQ_EncImage` is empty), so all cases use TPGDON=0.

The sample software's symbol dictionaries are not usable as test vectors:
its output has no referred-to segments, its export flags are wrong, and
its Huffman-coded symbol dictionaries do not code the collective bitmap
as T.88 specifies.  Fixing this would mean re-encoding the coded data, so
no symbol dictionary or text region cases are made with it.

### jbig2enc cases with synthetic images

| Test Name | Image |
|-----------|-------|
| test_enc_generic_plain | 64x64 checkerboard pattern of filled squares |
| test_enc_generic_border | 64x64 nested rectangular frames |
| test_enc_generic_tpgd | 64x64 diagonal line |
| test_enc_symbol_globals, test_enc_symbol_tpgd, test_enc_multipage_p1, test_enc_multipage_p2 | 200x100 grid of filled and divided rectangles and filled ellipses |
| test_enc_symbol_large | 400x300 rows of simple shapes (filled squares, T, O, -, +, \|) |

These were made with jbig2enc in PDF mode (`-p`, with `-s` for the symbol
cases) from synthetic images drawn for this project; the script which drew
the images and the jbig2enc version are not recorded.  The `.bmp` files are
jbig2dec output, not the encoder input, since symbol classification is
lossy.  In the symbol cases, `.globals` holds a symbol dictionary with page
association 0 and `.page` holds a page information segment and a text
region which refers to it.  `test_enc_multipage_p1` and
`test_enc_multipage_p2` share identical globals, as the pages of a PDF file
which use one global symbol dictionary would.

### Feature coverage

| Test Name | SymDict | TextRegion | GenericRegion | Other |
|-----------|---------|------------|---------------|-------|
| test_enc_generic_plain | - | - | Arith (T0) | - |
| test_enc_generic_border | - | - | Arith (T0) | - |
| test_enc_generic_tpgd | - | - | Arith (T0, TPGDON) | - |
| test_enc_page_generic | - | - | Arith (T0), large, lossless | XOR, end of page |
| test_g4_page_mmr | - | - | MMR, large, lossless | XOR, end of page, EOFB |
| test_enc_text_tpgd | - | - | Arith (T0, TPGDON) | - |
| test_enc_text_defpixel | - | - | Arith (T0) | default pixel 1, XOR |
| test_enc_text_symbol_local | Arith, page-local | Arith, at (4,1) | - | end of page |
| test_enc_symbol_globals | Arith (globals) | Arith | - | - |
| test_enc_symbol_large | Arith (globals) | Arith | - | - |
| test_enc_symbol_tpgd | Arith (globals) | Arith | - | - |
| test_enc_multipage_p1 | Arith (globals) | Arith | - | - |
| test_enc_multipage_p2 | Arith (globals) | Arith | - | - |

The `test_t88_gen_*` cases (see the table above) add generic regions with
templates 1-3, AT pixels away from their nominal positions, and the
extended template, all with TPGDON=0.  All other arithmetic generic
regions use template 0 with the nominal AT pixels.

### Known gaps

No independent encoder is available for the following features, so they
have no decoder test vectors here.  Except for the colour extension, which
PDF does not allow, they are covered by round-trip and fuzz tests, and the
MQ vectors below check refinement templates 0 and 1 at the coder level.

- Generic regions with TPGDON=1 and templates 1-3 (TPGDON is covered for
  template 0 only)
- Huffman-coded symbol dictionaries and text regions
- Refinement/aggregate symbol dictionaries, text-region refinement,
  refinement region segments
- Halftone region and pattern dictionary segments
- Custom Huffman table segments
- Striped pages (end of stripe segments), pages with unknown height
- Colour extension

## Arithmetic Coder Tests (mq/)

`mq_test_vectors.txt` contains MQ coder test vectors: synthetic inputs and
the bytes which the MQ coder of the ITU-T T.88 reference software produced
for them.  The inputs are self-described in each block: alternating
decisions, lists of integers, and small synthetic bitmaps (diagonal line,
checkerboard, stripes, centred block, all zeros) which the tests build with
helper functions such as `makeDiagonal`.  The file contains no ITU sample
data; the program which wrote it is not part of the repository.

Each test block has the format:
```
=== TEST: <name> ===
<parameters>
encoded: <hex bytes>
RESULT: PASS
```

Test categories:

- `mq_binary`: raw binary decisions (50 decisions, 16 contexts)
- `mq_integer`, `mq_integer_edge_cases`: integer encoding via the IADT,
  IADH, IADW, IADS, IAFS, IAIT and IARI procedures, including edge values
- `mq_iaid`: symbol ID encoding (IAID procedure, SBSYMCODELEN=2)
- `mq_image_*`: bitmaps coded with templates 0-3
- `mq_refinement_*`: refinement coding with templates 0 and 1,
  including the identical-bitmap case

The original output did not include the terminating marker byte 0xAC at
the end of streams whose final coded byte is 0xFF.  Per Figure E.11 of
ITU-T T.88 (2018), the FLUSH procedure appends 0xAC after the trailing
0xFF; the `encoded` values have been updated to include this byte.
