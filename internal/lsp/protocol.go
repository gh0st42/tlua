// Package lsp is a small Language Server Protocol client: enough of it to hand
// a document to a language server and get formatting edits back.
package lsp

import (
	"encoding/json"
	"strings"
)

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
// The protocol allows each of these to be a plain true or an options object, so
// they arrive raw and are checked for emptiness.
type serverCapabilities struct {
	DocumentFormattingProvider json.RawMessage `json:"documentFormattingProvider"`
	CompletionProvider         json.RawMessage `json:"completionProvider"`
	HoverProvider              json.RawMessage `json:"hoverProvider"`
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

/* --- completion --- */

// CompletionItemKind is what a completion is, from the protocol's list. Only
// the ones worth telling apart on screen are named here.
type CompletionItemKind int

const (
	KindText        CompletionItemKind = 1
	KindMethod      CompletionItemKind = 2
	KindFunction    CompletionItemKind = 3
	KindConstructor CompletionItemKind = 4
	KindField       CompletionItemKind = 5
	KindVariable    CompletionItemKind = 6
	KindClass       CompletionItemKind = 7
	KindModule      CompletionItemKind = 9
	KindProperty    CompletionItemKind = 10
	KindValue       CompletionItemKind = 12
	KindEnum        CompletionItemKind = 13
	KindKeyword     CompletionItemKind = 14
	KindSnippet     CompletionItemKind = 15
	KindConstant    CompletionItemKind = 21
)

// String is the short word the editor shows beside a completion.
func (k CompletionItemKind) String() string {
	switch k {
	case KindMethod:
		return "method"
	case KindFunction:
		return "function"
	case KindConstructor:
		return "constructor"
	case KindField:
		return "field"
	case KindVariable:
		return "variable"
	case KindClass:
		return "class"
	case KindModule:
		return "module"
	case KindProperty:
		return "property"
	case KindValue, KindConstant:
		return "value"
	case KindEnum:
		return "enum"
	case KindKeyword:
		return "keyword"
	case KindSnippet:
		return "snippet"
	default:
		return ""
	}
}

// Insert text comes either literally or as a snippet with placeholders.
const (
	insertFormatPlain   = 1
	insertFormatSnippet = 2
)

// CompletionItem is one thing the server offers to insert.
type CompletionItem struct {
	Label            string             `json:"label"`
	Kind             CompletionItemKind `json:"kind"`
	Detail           string             `json:"detail"`
	Documentation    json.RawMessage    `json:"documentation"`
	InsertText       string             `json:"insertText"`
	InsertTextFormat int                `json:"insertTextFormat"`
	TextEdit         *TextEdit          `json:"textEdit"`
	FilterText       string             `json:"filterText"`
	SortText         string             `json:"sortText"`
}

// Text is what inserting this item should put in the document, with any snippet
// placeholders reduced to their default text.
func (item CompletionItem) Text() string {
	text := item.InsertText
	if text == "" {
		text = item.Label
	}
	if item.InsertTextFormat == insertFormatSnippet {
		text = plainSnippet(text)
	}
	return text
}

// Help is the item's detail and documentation as plain text, for the line under
// a completion list.
func (item CompletionItem) Help() string {
	parts := []string{}
	if item.Detail != "" {
		parts = append(parts, item.Detail)
	}
	if doc := PlainText(markupText(item.Documentation)); doc != "" {
		parts = append(parts, doc)
	}
	return strings.Join(parts, " — ")
}

// completionList is the other shape a completion response can take.
type completionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}
