+++
title = 'Notes about the API design'
date = 2024-08-31T23:09:46+01:00
weight = 10
+++

Notes about the API design
==========================

Here I will collect notes about the overall API design of the library.

Error Handling
--------------

- In case of malformed PDF files, the library should read as much information
  as possible and not return an error unless absolutely necessary.  Correct
  files shold be read as per the spec, incorrect files should be read on a
  best-effort basis.  This library is *not* a PDF checker.

- Where errors must be returned by readers, there should is a distinction
  between errors caused by malformed input files, and OS-level errors.
  Errors for malformed input wrap `pdf.MalformedFileError`.

- Functions for writing PDF data should refuse to write invalid PDF and
  should always abort with an error in case of invalid input.
  PDF written by this library should be 100% conformant to the
  PDF spec.


Deduplication
-------------

- The type
  [`ResourceManager`](https://pkg.go.dev/seehuhn.de/go/pdf@v0.5.1-0.20250106103048-1692b734110c#ResourceManager)
  helps to ensure that only a single copy of a given object is embedded in the
  PDF file.  This is useful for sharing resources (fonts, images, ...) between
  different pages.

- To be used with a `ResourceManager`, objects must implement the [`Embedder`
  interface](https://pkg.go.dev/seehuhn.de/go/pdf@v0.5.1-0.20250106103048-1692b734110c#Embedder),
  by providing a method `Embed(*pdf.EmbedHelper) (pdf.Native, error)`.
  The object can either embed the object immediately, or it can defer
  embedding by using the `EmbedHelper.Defer()` method to register a
  function that will be called when the resource manager is closed.


Naming
------

- I still need to decide on a consisten naming scheme for functions which
  extract data from a PDF file.  Currently I have the following:
  - Functions to extract basic types have names starting with `Get...`.
    Examples include:
    - [`GetName(r pdf.Getter, obj Object) (x pdf.Name, err error)`](https://pkg.go.dev/seehuhn.de/go/pdf#GetName)
    - [`GetString(r pdf.Getter, obj Object) (x pdf.String, err error)`](https://pkg.go.dev/seehuhn.de/go/pdf#GetString)
    - [`GetNumber(r pdf.Getter, obj Object) (x pdf.Number, err error)`](https://pkg.go.dev/seehuhn.de/go/pdf#GetNumber)
    - [`GetRectangle(r pdf.Getter, obj Object) (x *pdf.Rectangle, err error)`](https://pkg.go.dev/seehuhn.de/go/pdf#GetRectangle)
  - More complex structures often have names starting with `Extract...`.
    Examples include:
    - [`font.ExtractDescriptor(r pdf.Getter, obj pdf.Object) (*font.Descriptor, error)`](https://pkg.go.dev/seehuhn.de/go/pdf@v0.5.1-0.20240906163623-d591f7fad2df/font#ExtractDescriptor)
    - [`color.ExtractSpace(r pdf.Getter, desc pdf.Object) (color.Space, error)`](https://pkg.go.dev/seehuhn.de/go/pdf@v0.5.1-0.20240906163623-d591f7fad2df/graphics/color#ExtractSpace)
    - [`cff.ExtractSimple(r pdf.Getter, dicts *font.Dicts) (*cff.FontDictSimple, error)`](https://pkg.go.dev/seehuhn.de/go/pdf/font/cff#ExtractSimple)

  In all cases, the first argument is a `pdf.Getter`.

- Methods of the form `AsPDF(pdf.OutputOptions) pdf.Native` are used to convert
  objects to their PDF representation.  This is only used for relatively simple
  objects, which do not need to be written as indirect objects.

- Methods with signature `WriteToPDF(*pdf.ResourceManager) error`
  are used to write objects to a PDF file as one or more indirect objects.

- Methods with signature `Embed(*pdf.EmbedHelper) (pdf.Native, error)`
  are used to write objects to the PDF file, in cases when deduplication
  is required. Objects can use `EmbedHelper.Defer()` to register functions
  for delayed execution when the resource manager is closed.


Composite Objects
-----------------

No reader:
./go-pdf/function/function.go:func (f *Type2) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/font/cff/font.go:func (f *Instance) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/font/opentype/font.go:func (f *Instance) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/font/truetype/font.go:func (f *Instance) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/font/type1/font.go:func (f *Instance) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/font/type3/font.go:func (f *Instance) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/graphics/extgstate.go:func (s *ExtGState) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/graphics/form/form.go:func (f *Form) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/graphics/op-marks.go:func (mc *MarkedContent) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/graphics/pattern/type1.go:func (p *type1) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/graphics/pattern/type2.go:func (p *Type2) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/graphics/shading/type1.go:func (s *Type1) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/graphics/shading/type3.go:func (s *Type3) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/graphics/shading/type4.go:func (s *Type4) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

With reader:
cmap.Extract -> *cmap.File -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
cmap.ExtractToUnicode -> *cmap.ToUnicodeFile -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
color.ExtractSpace -> ...
  ... -> *color.SpaceCalGray -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
  ... -> *color.SpaceCalRGB -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
  ... -> *color.SpaceLab -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
  ... -> color.SpaceDeviceCMYK -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
  ... -> color.SpaceDeviceGray -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
  ... -> color.SpaceDeviceRGB -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
  ... -> *color.SpaceICCBased -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
  ... -> *color.spacePatternColored -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
  ... -> *color.spacePatternUncolored -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
  ... -> *color.SpaceDeviceN -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
  ... -> *color.SpaceIndexed -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
  ... -> *color.SpaceSeparation -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)
metadata.ExtractStream -> *metadata.Stream -> Embed(rm *pdf.EmbedHelper) (pdf.Native, error)

TODO:
./go-pdf/graphics/image/dict.go:func (d *Dict) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/graphics/image/indexed.go:func (im *Indexed) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/graphics/image/jpeg.go:func (im *jpegImage) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
./go-pdf/graphics/image/png.go:func (im *PNG) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
