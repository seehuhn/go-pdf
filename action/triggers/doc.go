// Package triggers implements additional-actions dictionaries for PDF documents.
//
// Additional-actions dictionaries extend the set of events that can trigger
// action execution. There are four types of additional-actions dictionaries:
//
//   - [Annotation] for annotations (Table 197 in the PDF spec)
//   - [Page] for page objects (Table 198)
//   - [Form] for interactive form fields (Table 199)
//   - [Catalog] for document-level events (Table 200)
//
// Each type corresponds to the AA entry in its respective dictionary type.
package triggers
