pdf-inspect manual
==================

Basic usage is to call `pdf-inspect` with a PDF file as the first argument,
and more arguments to specify what to inspect.

```sh
pdf-inspect [options] example.pdf [root] selector* [action]
```

The following options are defined:

- `-p password`: Use the given password to decrypt the PDF file.
- `-show-metadata`: Show information from the Document Information dictionary
  and the file metadata stream in Human-Readable form and exit.

Root can be one of:

- `catalog` (default): The Document Catalog dictionary
- `info`: The Document Information dictionary
- `trailer`: The file trailer dictionary
- An PDF object number in the form of `n.g` where `n` is the object number and
  `g` is the generation number, or just `n` if the generation number is 0.

If no root is given, `catalog` is assumed.

A sequence of selectors can be used to follow the logical structure of the PDF
file, starting from the root. The meaning of each selector depends on the type
of the current object:

- For dictionaries, the selector is interpreted as a dictionary key.
  The key name can optionally be prefixed with a slash `/`.
  The new current object is the corresponding value in the dictionary.

- As a special case, if the current object is the root of the page tree,
  the selector can be a page number.  In this case, the current object is the
  corresponding page dictionary.

- If the current object is an array, the selector is interpreted as an index
  (0-based).  The new current object is the array element at the index.

- If the current object is a stream, the selector is interpreted as a key in
  the stream dictionary. The key name can optionally be prefixed with a slash
  `/`. The new current object is the value of the key.  The special selector
  `dict` can be used to access the stream dict as a whole.

The optional action decides what to do with the current object:

- `@show` (default): Print the current object to stdout in a human-readable
  form.

- `@raw` (when the current object is a stream): Write the raw stream data to
  stdout.  This does not apply any stream filters.

- `@stream` (when the current object is a stream): Decode the stream data and
  write it to stdout.

- `@contents` (when the current object is a page dictionary): Write the
  complete content stream of the page to stdout.
