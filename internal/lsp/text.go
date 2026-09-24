package lsp

import (
	"encoding/json"
	"regexp"
	"strings"
)

// markupText pulls the text out of the several shapes the protocol uses for
// documentation and hover contents: a bare string, a {language, value} pair, a
// {kind, value} pair, or an array of any of those.
func markupText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	var object struct {
		Kind     string `json:"kind"`
		Language string `json:"language"`
		Value    string `json:"value"`
	}
	if err := json.Unmarshal(raw, &object); err == nil && object.Value != "" {
		return object.Value
	}

	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		parts := make([]string, 0, len(list))
		for _, item := range list {
			if part := markupText(item); part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

var (
	fenceLine   = regexp.MustCompile("(?m)^\\s*```[\\w-]*\\s*$")
	headingLine = regexp.MustCompile(`(?m)^#{1,6}\s*`)
	codeSpan    = regexp.MustCompile("`([^`\n]+)`")
	// A link becomes its text: the URL beside it is noise on a text screen.
	link = regexp.MustCompile(`\[([^\]\n]+)\]\([^)\n]*\)`)
	// A horizontal rule is a paragraph break, which the blank lines already are.
	ruleLine     = regexp.MustCompile(`(?m)^\s*(?:-{3,}|\*{3,}|_{3,})\s*$`)
	starEmphasis = regexp.MustCompile(`\*\*([^*\n]+)\*\*|\*([^*\n]+)\*`)
	// Underscores need word boundaries around them, or an identifier such as
	// __index or some_var_name would come out with its middle eaten.
	underscoreEmphasis = regexp.MustCompile(`(^|[\s(\[{"'])_{1,2}([^_\n]+)_{1,2}([\s)\]}"'.,;:!?]|$)`)
	blankRun           = regexp.MustCompile(`\n{3,}`)
)

// PlainText reduces the markdown a server sends to something a text terminal
// can show: fences and emphasis markers go, the text inside them stays.
func PlainText(markdown string) string {
	if markdown == "" {
		return ""
	}
	text := fenceLine.ReplaceAllString(markdown, "")
	text = ruleLine.ReplaceAllString(text, "")
	text = headingLine.ReplaceAllString(text, "")
	text = link.ReplaceAllString(text, "$1")
	text = codeSpan.ReplaceAllString(text, "$1")
	text = starEmphasis.ReplaceAllStringFunc(text, func(match string) string {
		return strings.Trim(match, "*")
	})
	text = underscoreEmphasis.ReplaceAllString(text, "$1$2$3")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = blankRun.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

var (
	snippetDefault = regexp.MustCompile(`\$\{\d+:([^}]*)\}`)
	snippetChoice  = regexp.MustCompile(`\$\{\d+\|([^,|]*)[^}]*\}`)
	snippetPlain   = regexp.MustCompile(`\$\{?\d+\}?`)
)

// plainSnippet turns a snippet into the text it would hold with every
// placeholder left at its default, since this editor does not walk tab stops.
func plainSnippet(snippet string) string {
	text := snippetDefault.ReplaceAllString(snippet, "$1")
	text = snippetChoice.ReplaceAllString(text, "$1")
	text = snippetPlain.ReplaceAllString(text, "")
	return strings.ReplaceAll(text, `\$`, "$")
}
