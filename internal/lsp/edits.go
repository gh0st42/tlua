package lsp

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// ApplyEdits applies a server's edits to text and returns the result. Edits
// arrive in no particular order and their positions all refer to the original
// document, so they are applied from the last to the first.
func ApplyEdits(text string, edits []TextEdit) (string, error) {
	if len(edits) == 0 {
		return text, nil
	}

	type span struct {
		start, end int
		newText    string
	}
	spans := make([]span, 0, len(edits))
	for _, edit := range edits {
		start, err := Offset(text, edit.Range.Start)
		if err != nil {
			return "", fmt.Errorf("edit start: %w", err)
		}
		end, err := Offset(text, edit.Range.End)
		if err != nil {
			return "", fmt.Errorf("edit end: %w", err)
		}
		if end < start {
			start, end = end, start
		}
		spans = append(spans, span{start: start, end: end, newText: edit.NewText})
	}

	// Later edits first, and for edits at the same place the order they came in,
	// which is the order the protocol says to insert them in.
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].start > spans[j].start })

	var b strings.Builder
	b.Grow(len(text))
	out := text
	last := len(text) + 1
	for _, s := range spans {
		if s.end > last {
			return "", fmt.Errorf("overlapping edits at offset %d", s.start)
		}
		last = s.start
		out = out[:s.start] + s.newText + out[s.end:]
	}
	return out, nil
}

// Offset turns a protocol position into a byte offset into text. A line or
// character past the end of the document clamps to the end, which is what
// servers rely on when they address "the line after the last one".
func Offset(text string, pos Position) (int, error) {
	if pos.Line < 0 || pos.Character < 0 {
		return 0, fmt.Errorf("negative position %d:%d", pos.Line, pos.Character)
	}

	// Find the start of the line.
	offset, line := 0, 0
	for line < pos.Line {
		nl := strings.IndexByte(text[offset:], '\n')
		if nl < 0 {
			return len(text), nil // past the last line
		}
		offset += nl + 1
		line++
	}

	// Walk the line, counting UTF-16 code units rather than bytes: a character
	// outside the basic plane counts as two, and the protocol counts as the
	// server's editor would.
	rest := text[offset:]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[:nl]
	}
	units := 0
	for i := 0; i < len(rest); {
		if units >= pos.Character {
			return offset + i, nil
		}
		r, size := utf8.DecodeRuneInString(rest[i:])
		units += len(utf16.Encode([]rune{r}))
		i += size
	}
	return offset + len(rest), nil // past the end of the line
}
