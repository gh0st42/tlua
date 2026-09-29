package editor

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// searchState is what the Find and Replace dialogs last asked for, so that
// "find next" has something to repeat.
type searchState struct {
	what      string
	with      string
	matchCase bool
}

// findIn locates needle in text at or after from, wrapping round to the top
// when it runs off the end. The offsets it returns are into text.
func findIn(text, needle string, from int, matchCase bool) (start, end int, wrapped, ok bool) {
	if needle == "" {
		return 0, 0, false, false
	}
	haystack, what := searchable(text, needle, matchCase)
	if from < 0 {
		from = 0
	}
	if from > len(haystack) {
		from = len(haystack)
	}

	if i := strings.Index(haystack[from:], what); i >= 0 {
		return from + i, from + i + len(what), false, true
	}
	if i := strings.Index(haystack, what); i >= 0 {
		return i, i + len(what), true, true
	}
	return 0, 0, false, false
}

// findBefore locates the last match that starts before from, wrapping round to
// the bottom.
func findBefore(text, needle string, from int, matchCase bool) (start, end int, wrapped, ok bool) {
	if needle == "" {
		return 0, 0, false, false
	}
	haystack, what := searchable(text, needle, matchCase)
	if from < 0 {
		from = 0
	}
	if from > len(haystack) {
		from = len(haystack)
	}

	if i := strings.LastIndex(haystack[:from], what); i >= 0 {
		return i, i + len(what), false, true
	}
	if i := strings.LastIndex(haystack, what); i >= 0 {
		return i, i + len(what), true, true
	}
	return 0, 0, false, false
}

// searchable folds case when asked to, but only while that leaves every offset
// where it was: a few letters change length in lower case, and a shifted offset
// would select the wrong text.
func searchable(text, needle string, matchCase bool) (string, string) {
	if matchCase {
		return text, needle
	}
	lowerText, lowerNeedle := strings.ToLower(text), strings.ToLower(needle)
	if len(lowerText) == len(text) && len(lowerNeedle) == len(needle) {
		return lowerText, lowerNeedle
	}
	return text, needle
}

// matchesAll lists every match in text, left to right.
func matchesAll(text, needle string, matchCase bool) [][2]int {
	if needle == "" {
		return nil
	}
	haystack, what := searchable(text, needle, matchCase)
	var out [][2]int
	for at := 0; at+len(what) <= len(haystack); {
		i := strings.Index(haystack[at:], what)
		if i < 0 {
			break
		}
		start := at + i
		out = append(out, [2]int{start, start + len(what)})
		at = start + len(what)
	}
	return out
}

// lineOf reports the 1-based line an offset falls on.
func lineOf(text string, offset int) int {
	if offset > len(text) {
		offset = len(text)
	}
	return strings.Count(text[:offset], "\n") + 1
}

/* --- the commands --- */

// findNext repeats the last search, from wherever the cursor is.
func (e *Editor) findNext() { e.findFrom(false) }

// findPrevious repeats it in the other direction.
func (e *Editor) findPrevious() { e.findFrom(true) }

func (e *Editor) findFrom(backwards bool) {
	b := e.buf()
	if b == nil {
		return
	}
	if e.search.what == "" {
		e.findDialog()
		return
	}
	text := b.area.GetText()
	_, selStart, selEnd := b.area.GetSelection()

	var start, end int
	var wrapped, ok bool
	if backwards {
		start, end, wrapped, ok = findBefore(text, e.search.what, selStart, e.search.matchCase)
	} else {
		start, end, wrapped, ok = findIn(text, e.search.what, selEnd, e.search.matchCase)
	}
	if !ok {
		e.setStatus(fmt.Sprintf("%q not found", e.search.what))
		return
	}
	e.showAndSelect(b, start, end)
	note := ""
	if wrapped {
		note = ", wrapped round"
	}
	e.setStatus(fmt.Sprintf("Found %q on line %d%s", e.search.what, lineOf(text, start), note))
}

// replaceCurrent replaces the match under the cursor, or finds the next one to
// replace if the cursor is not on one.
func (e *Editor) replaceCurrent() {
	b := e.buf()
	if b == nil || e.search.what == "" {
		e.replaceDialog()
		return
	}
	selected, start, end := b.area.GetSelection()
	onMatch := start != end && sameText(selected, e.search.what, e.search.matchCase)
	if !onMatch {
		e.findNext()
		if selected, start, end = b.area.GetSelection(); start == end ||
			!sameText(selected, e.search.what, e.search.matchCase) {
			return // nothing to replace; findNext has said so
		}
	}
	b.area.Replace(start, end, e.search.with)
	after := start + len(e.search.with)
	e.showAndSelect(b, after, after)
	e.setStatus(fmt.Sprintf("Replaced one occurrence of %q", e.search.what))
	e.findNext()
}

// replaceAll rewrites every match in the buffer. The replacements go in back to
// front so that the offsets found up front stay valid, and each one goes
// through the text area, which keeps them all on the undo stack.
func (e *Editor) replaceAll() {
	b := e.buf()
	if b == nil || e.search.what == "" {
		return
	}
	matches := matchesAll(b.area.GetText(), e.search.what, e.search.matchCase)
	if len(matches) == 0 {
		e.setStatus(fmt.Sprintf("%q not found", e.search.what))
		return
	}
	for i := len(matches) - 1; i >= 0; i-- {
		b.area.Replace(matches[i][0], matches[i][1], e.search.with)
	}
	first := matches[0][0]
	e.showAndSelect(b, first, first+len(e.search.with))
	e.setStatus(fmt.Sprintf("Replaced %d occurrences of %q", len(matches), e.search.what))
}

func sameText(a, b string, matchCase bool) bool {
	if matchCase {
		return a == b
	}
	return strings.EqualFold(a, b)
}

/* --- the dialogs --- */

// selectedOrLastSearch prefills a search box the way an editor should: with
// what is selected, else with what was searched for last.
func (e *Editor) selectedOrLastSearch() string {
	if b := e.buf(); b != nil {
		if selected, start, end := b.area.GetSelection(); start != end &&
			!strings.ContainsRune(selected, '\n') {
			return selected
		}
	}
	return e.search.what
}

func (e *Editor) findDialog() {
	const name = "find"
	form := tview.NewForm()
	form.AddInputField("Find: ", e.selectedOrLastSearch(), 40, nil, nil)
	form.AddCheckbox("Case sensitive ", e.search.matchCase, nil)

	read := func() {
		e.search.what = form.GetFormItem(0).(*tview.InputField).GetText()
		e.search.matchCase = form.GetFormItem(1).(*tview.Checkbox).IsChecked()
	}
	form.AddButton("Find", func() {
		read()
		e.closeModal(name)
		if e.search.what != "" {
			e.findNext()
		}
	})
	form.AddButton("Cancel", func() { e.closeModal(name) })

	find := func() {
		read()
		e.closeModal(name)
		if e.search.what != "" {
			e.findNext()
		}
	}
	e.styleForm(form, "Find")
	form.SetInputCapture(dialogKeys(e, name, form, find))
	e.showModal(name, form, 60, 9)
}

func (e *Editor) replaceDialog() {
	const name = "replace"
	form := tview.NewForm()
	form.AddInputField("Find: ", e.selectedOrLastSearch(), 40, nil, nil)
	form.AddInputField("Replace with: ", e.search.with, 40, nil, nil)
	form.AddCheckbox("Case sensitive ", e.search.matchCase, nil)

	read := func() {
		e.search.what = form.GetFormItem(0).(*tview.InputField).GetText()
		e.search.with = form.GetFormItem(1).(*tview.InputField).GetText()
		e.search.matchCase = form.GetFormItem(2).(*tview.Checkbox).IsChecked()
	}
	form.AddButton("Replace", func() {
		read()
		e.closeModal(name)
		if e.search.what != "" {
			e.replaceCurrent()
		}
	})
	form.AddButton("Replace all", func() {
		read()
		e.closeModal(name)
		if e.search.what != "" {
			e.replaceAll()
		}
	})
	form.AddButton("Cancel", func() { e.closeModal(name) })

	replace := func() {
		read()
		e.closeModal(name)
		if e.search.what != "" {
			e.replaceCurrent()
		}
	}
	e.styleForm(form, "Replace")
	form.SetInputCapture(dialogKeys(e, name, form, replace))
	e.showModal(name, form, 60, 11)
}

// dialogKeys gives a form the two keys every DOS dialog had: Escape leaves,
// and Enter anywhere but on a button does what the first button does, so a
// search is "Ctrl-F, type, Enter".
func dialogKeys(e *Editor, name string, form *tview.Form, primary func()) func(*tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			e.closeModal(name)
			return nil
		case tcell.KeyEnter:
			if _, button := form.GetFocusedItemIndex(); button < 0 {
				primary()
				return nil
			}
		}
		return event
	}
}
