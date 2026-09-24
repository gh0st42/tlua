package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPlainText(t *testing.T) {
	markdown := "### print\n\n```lua\nfunction print(...)\n```\n\n" +
		"Prints **its arguments**, one `tab` apart.\n\n\n\nSee _also_ io.write.\n"

	got := PlainText(markdown)
	for _, unwanted := range []string{"```", "**", "###", "_also_"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("%q survived: %q", unwanted, got)
		}
	}
	for _, wanted := range []string{"print", "function print(...)", "its arguments", "tab", "also"} {
		if !strings.Contains(got, wanted) {
			t.Errorf("%q was lost: %q", wanted, got)
		}
	}
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("blank lines were not collapsed: %q", got)
	}
}

func TestPlainTextOfNothing(t *testing.T) {
	if got := PlainText(""); got != "" {
		t.Errorf("got %q", got)
	}
}

// Documentation and hover contents arrive in several shapes.
func TestMarkupText(t *testing.T) {
	cases := map[string]string{
		`"just a string"`:                                 "just a string",
		`{"kind":"markdown","value":"marked up"}`:         "marked up",
		`{"language":"lua","value":"function f()"}`:       "function f()",
		`["first",{"kind":"plaintext","value":"second"}]`: "first\n\nsecond",
		`null`: "",
		`{}`:   "",
	}
	for raw, want := range cases {
		if got := markupText(json.RawMessage(raw)); got != want {
			t.Errorf("markupText(%s) = %q, want %q", raw, got, want)
		}
	}
}

func TestPlainSnippet(t *testing.T) {
	cases := map[string]string{
		"for ${1:i} = 1, ${2:n} do\n\t$0\nend": "for i = 1, n do\n\t\nend",
		"print($1)":                            "print()",
		"${1|one,two|}":                        "one",
		"plain text":                           "plain text",
	}
	for snippet, want := range cases {
		if got := plainSnippet(snippet); got != want {
			t.Errorf("plainSnippet(%q) = %q, want %q", snippet, got, want)
		}
	}
}

func TestCompletionItemText(t *testing.T) {
	// The label is what goes in unless insertText says otherwise.
	if got := (CompletionItem{Label: "print"}).Text(); got != "print" {
		t.Errorf("got %q", got)
	}
	if got := (CompletionItem{Label: "insert", InsertText: "table.insert"}).Text(); got != "table.insert" {
		t.Errorf("got %q", got)
	}
	// A snippet loses its placeholders, since this editor has no tab stops.
	item := CompletionItem{Label: "for", InsertText: "for ${1:i} do $0 end", InsertTextFormat: 2}
	if got := item.Text(); got != "for i do  end" {
		t.Errorf("got %q", got)
	}
}

func TestCompletionItemHelp(t *testing.T) {
	item := CompletionItem{
		Label:         "pairs",
		Detail:        "function pairs(t)",
		Documentation: json.RawMessage(`{"kind":"markdown","value":"Iterates **a table**."}`),
	}
	got := item.Help()
	if !strings.Contains(got, "function pairs(t)") || !strings.Contains(got, "Iterates a table.") {
		t.Errorf("help = %q", got)
	}
	if strings.Contains(got, "**") {
		t.Errorf("markdown survived: %q", got)
	}
}

func TestCompletionItemKindNames(t *testing.T) {
	if got := KindFunction.String(); got != "function" {
		t.Errorf("got %q", got)
	}
	if got := CompletionItemKind(999).String(); got != "" {
		t.Errorf("an unknown kind gave %q", got)
	}
}

// Stripping emphasis must not touch Lua identifiers, which are full of
// underscores.
func TestPlainTextLeavesIdentifiersAlone(t *testing.T) {
	for _, code := range []string{
		"setmetatable(t, { __index = base })",
		"local some_var_name = 1",
		"_private and __gc and t.__index",
		"io.write(_VERSION)",
	} {
		if got := PlainText(code); got != code {
			t.Errorf("PlainText(%q) = %q", code, got)
		}
	}

	// Emphasis around a word still goes.
	if got := PlainText("see _also_ the __manual__ and *this*"); got != "see also the manual and this" {
		t.Errorf("got %q", got)
	}
}

// Real servers send links and rules; a text screen wants neither.
func TestPlainTextReducesLinksAndRules(t *testing.T) {
	markdown := "function print(...any)\n\n---\n\n" +
		"Converts each argument with [tostring](http://www.lua.org/manual/5.4/manual.html#pdf-tostring).\n\n" +
		"[View documents](http://www.lua.org/manual/5.4/manual.html#pdf-print)\n"

	got := PlainText(markdown)
	if strings.Contains(got, "http") || strings.Contains(got, "](") {
		t.Errorf("a URL survived: %q", got)
	}
	if strings.Contains(got, "---") {
		t.Errorf("a rule survived: %q", got)
	}
	for _, wanted := range []string{"function print(...any)", "tostring", "View documents"} {
		if !strings.Contains(got, wanted) {
			t.Errorf("%q was lost: %q", wanted, got)
		}
	}
}
