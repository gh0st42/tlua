// Package lsp is a small Language Server Protocol client: enough of it to hand
// a document to a language server and get formatting edits back.
package lsp

import "encoding/json"

// Position is a place in a document. Character counts UTF-16 code units, which
// is what the protocol says, not bytes and not runes.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a half-open span of a document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// TextEdit replaces one range with new text.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// FormatOptions are the formatting preferences the protocol lets a client state.
type FormatOptions struct {
	TabSize                int  `json:"tabSize"`
	InsertSpaces           bool `json:"insertSpaces"`
	TrimTrailingWhitespace bool `json:"trimTrailingWhitespace"`
	InsertFinalNewline     bool `json:"insertFinalNewline"`
}

// DefaultFormatOptions match what the editor itself does with a tab.
func DefaultFormatOptions() FormatOptions {
	return FormatOptions{
		TabSize:                4,
		InsertSpaces:           true,
		TrimTrailingWhitespace: true,
		InsertFinalNewline:     true,
	}
}

/* --- JSON-RPC --- */

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *responseError) Error() string { return e.Message }

// serverCapabilities is the part of the initialize result this client reads.
type serverCapabilities struct {
	// The protocol allows either a plain true or an options object, so this
	// takes whatever comes and is checked for emptiness.
	DocumentFormattingProvider json.RawMessage `json:"documentFormattingProvider"`
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}
