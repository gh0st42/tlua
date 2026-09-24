package editor

import (
	"fmt"
	"strings"
)

// commentPrefix is what a commented line gains, and what it loses again.
const commentPrefix = "-- "

// toggleCommentBlock comments or uncomments a run of whole lines. It
// uncomments only when every line with anything on it is already commented,
// which is what makes one key work both ways; a block where some lines are
// commented and some are not gets commented, so a second press undoes it.
//
// The prefix goes in at the shallowest indentation in the block, so a comment
// lines up with the code rather than with the left margin, and blank lines are
// left alone.
func toggleCommentBlock(block string) (result string, commented bool) {
	hadNewline := strings.HasSuffix(block, "\n")
	lines := strings.Split(strings.TrimSuffix(block, "\n"), "\n")

	allCommented, anyCode, indent := true, false, -1
	for _, line := range lines {
		code := strings.TrimLeft(line, " \t")
		if code == "" {
			continue
		}
		anyCode = true
		if !strings.HasPrefix(code, "--") {
			allCommented = false
		}
		if width := len(line) - len(code); indent < 0 || width < indent {
			indent = width
		}
	}
	if !anyCode {
		return block, false // nothing but blank lines
	}

	out := make([]string, len(lines))
	for i, line := range lines {
		code := strings.TrimLeft(line, " \t")
		if code == "" {
			out[i] = line
			continue
		}
		if allCommented {
			rest := strings.TrimPrefix(code, "--")
			rest = strings.TrimPrefix(rest, " ") // the one space this put there
			out[i] = line[:len(line)-len(code)] + rest
			continue
		}
		out[i] = line[:indent] + commentPrefix + line[indent:]
	}

	result = strings.Join(out, "\n")
	if hadNewline {
		result += "\n"
	}
	return result, !allCommented
}

// lineSpan returns the byte range covering whole lines first to last, ending
// after the last line's newline when it has one.
func lineSpan(text string, first, last int) (start, end int) {
	start = offsetAt(text, first, 0)
	end = offsetAt(text, last, 0)
	if nl := strings.IndexByte(text[end:], '\n'); nl >= 0 {
		end += nl + 1
	} else {
		end = len(text)
	}
	return start, end
}

// toggleComment is Ctrl-B: it comments the line the cursor is on, or every line
// the selection touches.
func (e *Editor) toggleComment() {
	b := e.buf()
	if b == nil {
		return
	}
	text := b.area.GetText()
	if text == "" {
		return
	}
	_, selStart, selEnd := b.area.GetSelection()

	first := lineOf(text, selStart) - 1
	last := lineOf(text, selEnd) - 1
	// A selection that stops at the very start of a line does not reach into
	// that line, so it should not be commented.
	if last > first && selEnd > selStart && offsetAt(text, last, 0) == selEnd {
		last--
	}

	start, end := lineSpan(text, first, last)
	toggled, commented := toggleCommentBlock(text[start:end])
	if toggled == text[start:end] {
		e.setStatus("Nothing there to comment")
		return
	}

	row, column, _, _ := b.area.GetCursor()
	b.area.Replace(start, end, toggled)

	if selStart != selEnd {
		// Keep the same lines selected, so the key can be pressed again.
		b.area.Select(start, start+len(toggled))
	} else {
		// Keep the cursor where it was, moved along by what the line gained or
		// lost in front of it.
		delta := len(toggled) - (end - start)
		if column+delta < 0 {
			delta = -column
		}
		offset := offsetAt(b.area.GetText(), row, column+delta)
		b.area.Select(offset, offset)
	}

	lines := last - first + 1
	action := "Uncommented"
	if commented {
		action = "Commented"
	}
	if lines == 1 {
		e.setStatus(fmt.Sprintf("%s line %d", action, first+1))
		return
	}
	e.setStatus(fmt.Sprintf("%s %d lines", action, lines))
}
