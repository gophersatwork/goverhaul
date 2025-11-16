package lsp

// LSP protocol types for hover functionality
// Based on Language Server Protocol Specification

// Position represents a position in a text document
type Position struct {
	Line      int `json:"line"`      // Line position in a document (zero-based)
	Character int `json:"character"` // Character offset on a line in a document (zero-based)
}

// Range represents a range in a text document
type Range struct {
	Start Position `json:"start"` // The range's start position
	End   Position `json:"end"`   // The range's end position
}

// Location represents a location inside a resource
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier identifies a text document
type TextDocumentIdentifier struct {
	URI string `json:"uri"` // The text document's URI
}

// TextDocumentPositionParams represents parameters for text document position
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"` // The text document
	Position     Position               `json:"position"`     // The position inside the text document
}

// MarkupKind describes the content type that a client supports in various
// result literals like `Hover`, `ParameterInfo` or `CompletionItem`.
type MarkupKind string

const (
	PlainText MarkupKind = "plaintext"
	Markdown  MarkupKind = "markdown"
)

// MarkupContent represents a string value which content is interpreted based on its kind flag
type MarkupContent struct {
	Kind  MarkupKind `json:"kind"`  // The type of the Markup
	Value string     `json:"value"` // The content itself
}

// Hover represents the result of a hover request
type Hover struct {
	Contents MarkupContent `json:"contents"` // The hover's content
	Range    *Range        `json:"range,omitempty"`
}
