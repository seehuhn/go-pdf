+++
title = 'The Embedder Interface'
date = 2025-06-18T13:49:16+01:00
+++

# The Embedder Interface

In the library, file-independent objects contain only data that can be shared
across PDF files, such as font metrics, color space parameters, or image data.
They cannot contain any references to file-specific object, and also no
`pdf.Reference` fields since these are tied to specific files. File-independent
objects should implement the [Embedder] interface, which allows them to be
embedded into a PDF file, transforming them into file-specific representations:

```go
type Embedder[T any] interface {
	// Embed converts the Go representation of the object into a PDF object,
	// corresponding to the PDF version of the output file.
	//
	// The first return value is the PDF representation of the object.
	// If the object is embedded in the PDF file, this may be a reference.
	//
	// The second return value is a Go representation of the embedded object.
	// In most cases, this value is not used and T can be set to [pdf.Unused].
	Embed(rm *ResourceManager) (Native, T, error)
}
```

Users should not call the `Embed` method directly. Instead, the
`pdf.ResourceManagerEmbed` function should be used to embed objects into a PDF
file.


## Resource Managers

The ResourceManager coordinates the process of embedding objects into a PDF
file. It ensures that objects are not duplicated and that dependencies between
objects are properly resolved.

```go
type ResourceManager struct {
	Out *Writer
}

func NewResourceManager(w *pdf.Writer) *pdf.ResourceManager
func (rm *pdf.ResourceManager) Close() error
```

Normally, there will only be one ResourceManager per PDF file.  The resource
manager must be closed before the PDF file is closed.  New objects are added
to the PDF file using the `pdf.ResourceManagerEmbed` function:

```go
func ResourceManagerEmbed[T any](rm *ResourceManager, obj Embedder[T]) (pdf.Native, T, error)
```

The return values are:
- The PDF representation of the embedded object.  This is the value which might
  for example be used in a resource dictionary to represent the object. The
  value is typically a `pdf.Reference`, but can also be a `pdf.Name` or other
  PDF primitive.
- The Go representation of the embedded, file-specific object. If this is not
  needed, the type parameter `T` can be set to [pdf.Unused].
- An error if the embedding failed.

Depending on the type of the embedded object, the actual embedding may
be delayed until the ResourceManager is closed.

The resource manager keeps track of which objects have already been
embedded. If an object is embedded multiple times, the resource manager
will return the same PDF representation for later calls, without calling
the `Embed` method again. For this to work, embedders must be "comparable".
An easy way to ensure this is to implement embedders as pointer types.

### Alternative: Function-Driven Embedding

For cases where objects don't implement the Embedder interface, or when you
need more control over the embedding process, the `ResourceManagerEmbedFunc`
function provides an alternative approach:

```go
func ResourceManagerEmbedFunc[T any](rm *ResourceManager, f func(*ResourceManager, T) (Object, error), obj T) (Object, error)
```

This function embeds a resource using a custom embedding function instead of
the Embedder interface. Like `ResourceManagerEmbed`, it prevents duplicate
embedding by tracking already embedded objects.

Example usage:

```go
embedFunc := func(rm *ResourceManager, data MyCustomData) (Object, error) {
    ref := rm.Out.Alloc()
    dict := pdf.Dict{
        "Type": pdf.Name("CustomType"),
        "Data": pdf.String(data.Value),
    }
    err := rm.Out.Put(ref, dict)
    return ref, err
}

obj, err := pdf.ResourceManagerEmbedFunc(rm, embedFunc, myData)
```

## Embedders

File-independent objects must implement the [Embedder] interface to
allow embedding into a PDF file.  The `Embed` method receives a
`ResourceManager` instance, which can be used to embed sub-objects.
The `rm.Out` field in the `ResourceManager` is a `pdf.Writer` that
should be used to write any PDF data structures, such as dictionaries or streams.

Example 1: Simple objects like device color spaces return PDF primitives
directly. For example, DeviceGray color space returns the PDF name
`/DeviceGray` without requiring any file-specific resources:

```go
func (s spaceDeviceGray) Embed(rm *ResourceManager) (Native, Unused, error) {
    return pdf.Name("DeviceGray"), zero, nil
}
```

Complex objects that require PDF streams or dictionaries typically allocate a
new object reference and create the necessary file structures. A TrueType font
implementation demonstrates this pattern:

Example 2: More complex objects use `rm.Out` to allocate a new PDF reference
where they then write the PDF representation of the object. For example,
a method to write JPEG images might look like this:

```go
func (img *Image) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	ref := rm.Out.Alloc()
	stream, err := rm.Out.OpenStream(ref, pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(img.Bounds().Dx()),
		"Height":           pdf.Integer(img.Bounds().Dy()),
		"ColorSpace":       pdf.Name(color.FamilyDeviceRGB),
		"BitsPerComponent": pdf.Integer(8),
		"Filter":           pdf.Name("DCTDecode"),
	})

	// handle errors and write the JPEG image data to the stream

	return ref, zero, nil
}
```

Example 3: When objects depend on other embeddable resources, they use
`pdf.ResourceManagerEmbed` to recursively embed their dependencies before
embedding the main object. For example, when a PNG image embeds its color space
dependency:

```go
func (img *Image) Embed(rm *ResourceManager) (Native, Unused, error) {
    csEmbedded, _, err := pdf.ResourceManagerEmbed(rm, img.cs)
    if err != nil {
        return nil, zero, err
    }

    imDict := pdf.Dict{
        "Type":       pdf.Name("XObject"),
        "Subtype":    pdf.Name("Image"),
        "ColorSpace": csEmbedded,  // Using embedded result
        // ... other fields
    }

    ref := rm.Out.Alloc()

    // ... write image stream at ref

    return ref, zero, nil
}
```

## Delayed Embedding

Some objects can still be updated after they have been embedded.
The main example of this is font embedding, where a reference to the font
dictionary is required the first time the font is used on a page, but
the list of glyphs used in the font (required for subsetting) is not
known until the last use of the font has occurred.  In these cases,
the second return value from the `Embed` method is a Go object which is
used to update the state of the embedded object.  This Go object
must implement the `pdf.Finisher` interface, and the resource manager
will automatically call the `Finish` method on this object when the
ResourceManager is closed.

For example, file-independent fonts in this library implement the `font.Font`
interface.

```go
type Font interface {
	// PostScriptName returns the PostScript name of the font.
	PostScriptName() string

	pdf.Embedder[font.Embedded]
}
```

When using the `pdf.ResourceManagerEmbed` function to embed a font, the
call returns a `font.Embedded` object which can be used typeset text.

For objects with delayed embedding, normally the `Embed` method
is very simple.  It just allocates a new PDF reference and returns
the embedded object.  All the actual work is done in the `Finish` method
of the embedded object.

Example: the code to embed a simple PDF font might look like this:

```go
func (f *Instance) Embed(rm *ResourceManager) (Native, font.Embedded, error) {
    ref := rm.Out.Alloc()
    embedded := newEmbeddedSimple(ref, f.Font)
    return ref, embedded, nil
}

type embeddedSimple struct {
    Ref      pdf.Reference
    Font     *sfnt.Font
    ...
}

func (e *embeddedSimple) Finish(rm *ResourceManager) error {
    // subset the font and write to rm.Out
}
```
