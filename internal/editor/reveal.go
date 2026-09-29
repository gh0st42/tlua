package editor

// Moving the cursor about a file is not as simple as it looks, because of how
// tview's text area finds a position.
//
// It can only place the cursor at an offset that falls within the lines it has
// already broken over, and it breaks over the window it last drew. Asked for an
// offset further down than that, its search walks into the sentinel that stands
// for "the rest of the text" and treats all of it as one long row: the cursor
// ends up at the end of the last drawn line, thousands of columns along. Jumping
// to line 600 of an 842 line file put the cursor on line 26.
//
// So a jump out of view moves the view first and places the cursor on the next
// turn of the loop, once the drawing has broken the lines over that far.

// revealLine puts a line at the top of the window with the cursor at its start.
// This is what a jump to a definition wants: the thing jumped to, and what
// follows it, rather than the thing jumped to at the bottom of the screen.
func (e *Editor) revealLine(b *buffer, line int) {
	if line < 1 {
		line = 1
	}
	row := line - 1
	b.area.SetOffset(row, 0)
	e.afterDraw(func() {
		offset := offsetAt(b.area.GetText(), row, 0)
		b.area.Select(offset, offset)
		b.area.SetOffset(row, 0) // the cursor move must not scroll it away again
	})
}

// showAndSelect selects a stretch of text and makes sure it can be seen, moving
// the view as little as it can: a match or a mistake is best read with the lines
// around it, unlike a definition.
func (e *Editor) showAndSelect(b *buffer, start, end int) {
	text := b.area.GetText()
	row := lineOf(text, start) - 1

	viewRow, _ := b.area.GetOffset()
	_, _, _, height := b.area.GetInnerRect()
	if height <= 0 {
		height = 1
	}
	if row >= viewRow && row < viewRow+height {
		// Already on screen, so the text area knows where it is.
		b.area.Select(start, end)
		return
	}

	// Out of view: bring it a third of the way down, which leaves a little of
	// what comes before it, and place the cursor once the view has caught up.
	top := row - height/3
	if top < 0 {
		top = 0
	}
	b.area.SetOffset(top, 0)
	e.afterDraw(func() { b.area.Select(start, end) })
}

// afterDraw runs fn once the screen has caught up with what has been asked for.
//
// It draws first and then runs fn, rather than trusting that a draw will happen
// in between: it usually would, since tview draws after each key, but a caller
// that is itself queued work would otherwise place the cursor against a line
// index that had not been extended yet. It goes through a goroutine because
// tview's queue waits for the work it is given, so the loop cannot queue work
// for itself.
func (e *Editor) afterDraw(fn func()) {
	go func() {
		e.app.QueueUpdateDraw(func() {}) // let the drawing reach the new view
		e.app.QueueUpdateDraw(fn)
	}()
}
