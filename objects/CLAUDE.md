# objects/

Mapping between Go types and PDF object types, as YAML organized by PDF 2.0 spec chapter.

## Query tool

```bash
./query type AnnotText
./query types
```

Pipe to yq for filtering:
```bash
./query type AnnotText | yq '.fields[] | [.source, .goName, .pdfKey] | @tsv'
./query type AnnotText | yq '.fields[] | select(.goName == null)'
./query types | yq '.[] | select(.pdfType == "Annot") | .name'
```

yq quirks: `*` is a wildcard in `==` (use `test("^\\*pdf\\.Rectangle$")` for literal matching); object keys must be quoted `{"key": .val}`.

## Schema structure

- Types use `embeds: [AnnotCommon, AnnotMarkup]` for Go struct embedding
- `discriminator: true` marks PDF-only Type/Subtype fields
- `goName: null` means field not implemented in Go
- See SCHEMA.md for full format
