// Package pdf provides support for reading and writing PDF files.
//
// This package treats PDF files as containers containing a sequence of objects
// (typically Dictionaries and Streams).  Object are written sequentially, but
// can be read in any order.
//
// A `Reader` can be used to read an existing PDF file:
//
//      r, err := pdf.Open("in.pdf")
//      if err != nil {
//          log.Fatal(err)
//      }
//      defer r.Close()
//      catalog, err := r.Catalog()
//      if err != nil {
//          log.Fatal(err)
//      }
//      ... use catalog to locate objects in the file ...
//
// A `Writer` can be used to write a new PDF file:
//
//     w, err := pdf.Create("out.pdf")
//     if err != nil {
//         log.Fatal(err)
//     }
//
//     ... add pages to the document using w.Write() and w.OpenStream() ...
//
//     err = w.SetCatalog(pdf.Struct(&pdf.Catalog{
//         Pages: pages,
//     }))
//     if err != nil {
//         log.Fatal(err)
//     }
//
//     err = out.Close()
//     if err != nil {
//         log.Fatal(err)
//     }
//
// The following classes implement native PDF objects which can be stored in
// PDF files.  All of these implement the `pdf.Object` interface:
//
//     Array
//     Bool
//     Dict
//     Integer
//     Name
//     Real
//     Reference
//     Stream
//     String
//
// Subpackages implement support to produce PDF files representing pages of
// text and images.
package pdf
