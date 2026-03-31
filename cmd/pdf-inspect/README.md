pdf-inspect manual
==================

Basic usage is to call `pdf-inspect` with a PDF file as the first argument,
and more arguments to specify what to inspect.

```sh
pdf-inspect [options] file.pdf [path]
```

The following options are defined:

- `-p password`: Use the given password to decrypt the PDF file.

The path argument is a sequence of selectors that traverse the PDF document
structure, starting at file level.

The first element of the path, if any, must be one of
- `meta`: File metadata (name, size, modification time)
- `catalog`: The Document Catalog dictionary
- `info`: The Document Information dictionary
- `trailer`: The file trailer dictionary
- an object reference: go to the corresponding indirect object
  (e.g., `1` for object `1 0 R`)
- a catalog key: go to the corresponding value in the catalog dictionary
  (e.g., `Pages` for the page tree root).


Along the path, the meaning of each selector depends on the type of the current
object. Available navigation options are shown at the bottom of each output for
easy discovery.

- For dictionaries, the selector is interpreted as a dictionary key.
  The key name can optionally be prefixed with a slash `/`.
  The next object is the corresponding value in the dictionary.

- If the current object is a page dictionary, the selector can be one of
  * Dictionary keys, optionally prefixed with a slash `/`.
    The next object is the value of the key.
  * `@contents`: Write the complete content stream of the page to stdout.

- If the current object is a font dictionary:
  * Dictionary keys, optionally prefixed with a slash `/`.
    The next object is the value of the key.
  * `@font`: Display detailed font information including PostScript name, descriptor, and font type.

- If the current object is a font dictionary with an embedded font program:
  * `@raw`: Output the binary font program to stdout.
  * `load`: go to the embedded font program (TrueType or Type1), if any.

- If the current object is a loaded font:
  * `glyphs`: Display the glyph list with character mappings and bounding boxes.

- As a special case, if the current object is the root of the page tree, the
  selector can be a 1-based page number. In this case, the next object is
  the corresponding page dictionary.

- If the current object is an array, the selector is interpreted as a 0-based index.
  Negative indices count from the end of the array, e.g., `-1` for the last element.
  The next object is the array element at the index.

- If the current object is a stream, following selectors are allowed:
  * Keys in the stream dictionary, optionally prefixed with a slash `/`.
    The next object is the value of the key.
  * `dict` goes to the stream dictionary itself.
  * `@raw` decodes the stream data and writes it to stdout.
  * `@encoded` writes the encoded stream data to stdout without decoding any
    stream filters.
