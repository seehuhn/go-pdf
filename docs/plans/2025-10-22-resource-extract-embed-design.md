# Resource Extract and Embed Design

## Purpose

Implement `Extract` and `Embed` functions for the `Resource` struct to enable reading resource dictionaries from PDF files and writing them back.

## Requirements

### Extraction (Permissive)
- Skip invalid entries rather than fail
- Accept malformed resource dictionaries silently
- Return empty Resource struct for missing/nil dictionaries
- Extract succeeds even if individual resources fail to extract

### Embedding (Strict)
- Return errors for individual embed failures (cannot produce invalid PDF)
- Validate PDF version constraints
- Omit empty subdictionaries from output
- Preserve direct/indirect structure via SingleUse flag

## Design

### SingleUse Field

Add `SingleUse bool` field to the Resource struct. This field controls output format:
- `true`: Embed returns dictionary directly
- `false`: Embed allocates reference and returns it

Extract sets this field based on whether the original object was direct or indirect, preserving the structure in round-trip operations.

### Extract Implementation

Extract follows this sequence:

1. Check if object is indirect (before resolving) to set SingleUse
2. Resolve object and verify it is a dictionary
3. Create empty Resource struct with SingleUse set
4. Process each resource type sequentially:
   - ExtGState: call `graphics.ExtractExtGState`
   - ColorSpace: call `color.ExtractSpace`
   - Pattern: call `color.ExtractPattern`
   - Shading: call `graphics.ExtractShading`
   - XObject: call `graphics.ExtractXObject`
   - Font: call `font/dict.ExtractFont`
   - Properties: call `property.Extract`
5. Handle ProcSet (array format): read names, set boolean fields
6. Return populated Resource

For each subdictionary:
- Get subdictionary via `x.GetDict()`
- If missing or wrong type, skip with continue
- Iterate name-object pairs
- Call appropriate extractor via `pdf.ExtractorGet`
- If extraction fails, skip that entry with continue (permissive)
- Only initialize maps when entries exist (lazy initialization)

### Embed Implementation

Embed follows this sequence:

1. Validate PDF version:
   - Error if Shading non-empty and version < 1.3
   - Error if Properties non-empty and version < 1.2
   - Error if any ProcSet bits set and version >= 2.0
2. Create empty result dictionary
3. For each resource type:
   - Skip if map is nil or empty
   - Create subdictionary
   - Iterate map entries
   - Call `rm.Embed()` on each resource
   - If embed fails, return error (strict)
   - Add name-to-reference mapping to subdictionary
   - Add subdictionary to result
4. Handle ProcSet: convert booleans to array of names
5. Return based on SingleUse:
   - If true: return dictionary
   - If false: allocate reference, call `rm.Out().Put()`, return reference

Processing order matches Table 34 in PDF spec section 7.8.3:
1. ExtGState
2. ColorSpace
3. Pattern
4. Shading
5. XObject
6. Font
7. ProcSet
8. Properties

### ProcSet Conversion

Extract: Read array of names (`/PDF`, `/Text`, `/ImageB`, `/ImageC`, `/ImageI`), set corresponding boolean fields. Unknown names are ignored.

Embed: Build array from true boolean fields in same order.

## Testing

### Table-Driven TestRoundTrip
- One test case per resource type
- One test case with multiple types combined
- One test case with empty resource dictionary
- One test case for SingleUse (direct vs indirect)
- Compare with `cmp.Diff`

### FuzzRoundTrip
- Seed corpus with test cases
- Perform read-write-read cycle
- Verify malformed PDFs are skipped during extraction

### Version Validation Tests
- Verify Embed returns errors for:
  - Shading with PDF < 1.3
  - Properties with PDF < 1.2
  - ProcSet with PDF >= 2.0

### ProcSet Conversion Tests
- Test all combinations of five boolean fields
- Verify unknown names ignored during extraction
