package lsp

// Position represents a position in a text document
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a range in a text document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location inside a resource
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Diagnostic represents a diagnostic message
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
	Code     string `json:"code,omitempty"`
}

// DiagnosticSeverity defines the severity levels
const (
	DiagnosticSeverityError       = 1
	DiagnosticSeverityWarning     = 2
	DiagnosticSeverityInformation = 3
	DiagnosticSeverityHint        = 4
)

// TextEdit represents a textual edit
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// WorkspaceEdit represents changes to many resources
type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes,omitempty"`
}

// CodeActionKind defines the kind of code action
type CodeActionKind string

const (
	QuickFix                 CodeActionKind = "quickfix"
	Refactor                 CodeActionKind = "refactor"
	RefactorExtract          CodeActionKind = "refactor.extract"
	RefactorInline           CodeActionKind = "refactor.inline"
	RefactorRewrite          CodeActionKind = "refactor.rewrite"
	Source                   CodeActionKind = "source"
	SourceOrganizeImports    CodeActionKind = "source.organizeImports"
	SourceFixAll             CodeActionKind = "source.fixAll"
)

// CodeAction represents a change that can be performed in code
type CodeAction struct {
	Title       string          `json:"title"`
	Kind        CodeActionKind  `json:"kind,omitempty"`
	Diagnostics []Diagnostic    `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit  `json:"edit,omitempty"`
	Command     *Command        `json:"command,omitempty"`
	IsPreferred bool            `json:"isPreferred,omitempty"`
}

// Command represents a reference to a command
type Command struct {
	Title     string        `json:"title"`
	Command   string        `json:"command"`
	Arguments []interface{} `json:"arguments,omitempty"`
}

// CodeActionContext contains additional diagnostic information
type CodeActionContext struct {
	Diagnostics []Diagnostic     `json:"diagnostics"`
	Only        []CodeActionKind `json:"only,omitempty"`
}

// TextDocumentIdentifier identifies a text document
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// CodeActionParams parameters for code action request
type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

// PublishDiagnosticsParams parameters for publishing diagnostics
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
