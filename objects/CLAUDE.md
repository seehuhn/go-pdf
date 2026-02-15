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

## Graph tool

```bash
./graph stats            # summary statistics
./graph edges            # all edges (A -> B)
./graph loops            # all cycles
./graph file-boundary    # fi -> fd boundary edges
./graph field-collisions # PDF keys mapping to different Go names/types
./graph docs             # auto-generate documentation
```

## Schema notes

- `refTypes` lists valid PDF object types for a field (string or `{type, collection, since, deprecated}`)
- Internal map types use `_` prefix (e.g. `_FontMap`) with `refTypes` at the type level
- Interface names in `refTypes` expand to all implementations
- `embeds: [AnnotCommon, AnnotMarkup]` for Go struct embedding
- `discriminator: true` marks PDF-only Type/Subtype fields
- `goName: null` means field not implemented in Go
- `fileDependent: true` = Encode/Decode, `false` = Embed/Extract, omit if unknown
- `deprecated: "2.0"` marks types/fields deprecated in a PDF version
- See SCHEMA.md for full format
