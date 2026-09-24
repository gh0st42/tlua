package editor

import (
	"context"
	"strings"
	"time"

	"github.com/rivo/tview"

	"tlua/internal/lsp"
)

// signatureTriggers are the characters after which this editor asks what a call
// takes: the one that opens a call, and the one that moves to the next argument.
// As with completions, servers ask for more than that and are narrowed down.
const signatureTriggers = "(,"

const lspSignatureTimeout = 700 * time.Millisecond

// signaturePage is the name of the hint's page. It is not a dialog: it takes no
// focus, so typing carries on underneath it.
const signaturePage = "signature"

// isSignatureTrigger reports whether typing r is reason enough to ask what the
// call being typed takes.
func (e *Editor) isSignatureTrigger(r rune) bool {
	if e.lsp == nil || !e.lsp.CanSignature() || !strings.ContainsRune(signatureTriggers, r) {
		return false
	}
	for _, trigger := range e.lsp.SignatureTriggerCharacters() {
		if trigger == string(r) {
			return true
		}
	}
	return false
}

// signatureHelp asks what the call at the cursor takes and shows it. Asked for
// by hand it explains itself; asked because a bracket was typed it keeps quiet.
func (e *Editor) signatureHelp(automatic bool) {
	b := e.buf()
	if b == nil {
		return
	}
	switch {
	case e.lsp == nil:
		if !automatic {
			e.setStatus("No language server; put one on PATH, or set " + lsp.EnvServer)
		}
		return
	case !e.lsp.CanSignature():
		if !automatic {
			e.setStatus(e.lsp.Name() + " does not offer signature help")
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), lspSignatureTimeout)
	defer cancel()

	help, err := e.lsp.Signature(ctx, b.lspPath(), b.area.GetText(), e.cursorPosition(b))
	if err != nil {
		e.hideSignature()
		if !automatic {
			e.setStatus("Parameters: " + err.Error())
		}
		return
	}
	signature, active, ok := help.Active()
	if !ok {
		e.hideSignature()
		if !automatic {
			e.setStatus("Nothing is being called here")
		}
		return
	}
	e.showSignature(b, signature, active)
}

// refreshSignature keeps the hint in step with the cursor while it is up, and
// takes it down once the cursor leaves the call. It runs from the text area's
// own moved handler, where the text and the cursor are already up to date.
func (e *Editor) refreshSignature() {
	if !e.signatureShown || e.modals > 0 || e.openMenu >= 0 {
		return
	}
	e.signatureHelp(true)
}

// showSignature draws the hint, with the parameter the cursor is at picked out.
func (e *Editor) showSignature(b *buffer, signature lsp.SignatureInformation, active int) {
	label := signature.Label
	text := tview.Escape(label)

	// Pick out the active parameter, when the server said which one it is.
	if active >= 0 && active < len(signature.Parameters) {
		if start, end, ok := signature.Parameters[active].Span(label); ok {
			text = tview.Escape(label[:start]) +
				"[black:green:b]" + tview.Escape(label[start:end]) + "[black:silver:-]" +
				tview.Escape(label[end:])
		}
	}

	if e.signatureInStatus {
		e.hidePanel()
		e.setTaggedStatus(text)
		e.signatureShown = true
		return
	}
	e.showSignaturePanel(b, "[black:silver]"+text)
}

// showSignaturePanel floats the hint on the line above the cursor, or below it
// at the top of the window, where it covers no line being typed.
func (e *Editor) showSignaturePanel(b *buffer, text string) {
	hint := tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	hint.SetBackgroundColor(egaLightGray)
	hint.SetTextStyle(dialogTextStyle)
	hint.SetText(" " + text)

	screenWidth, _ := e.screenSize()
	width := len(stripTags(text)) + 2
	if width > screenWidth {
		width = screenWidth
	}

	at := e.cursorScreenPosition(b)
	col := at.col
	if col+width > screenWidth {
		col = screenWidth - width
	}
	if col < 0 {
		col = 0
	}

	// The line above the call is the natural place, but the window's frame is
	// not: on the first line of a buffer the hint goes below instead.
	_, textTop, _, textHeight := b.area.GetInnerRect()
	row := at.row - 1
	if row < textTop {
		row = at.row + 1
	}
	if row >= textTop+textHeight {
		row = textTop
	}

	e.pages.RemovePage(signaturePage)
	e.pages.AddPage(signaturePage, place(hint, col, row, width, 1), true, true)
	e.signatureShown = true
	// The hint takes no focus: the keyboard stays with the text.
	if b.area != nil {
		e.app.SetFocus(b.area)
	}
}

// hideSignature takes the hint down, wherever it was showing.
func (e *Editor) hideSignature() {
	if !e.signatureShown {
		return
	}
	e.signatureShown = false
	e.hidePanel()
	if e.signatureInStatus {
		e.clearTaggedStatus()
	}
}

func (e *Editor) hidePanel() {
	if e.pages.HasPage(signaturePage) {
		e.pages.RemovePage(signaturePage)
	}
}

// toggleSignaturePlace moves the hint between a panel at the cursor and the
// status line, since either reads well and people differ.
func (e *Editor) toggleSignaturePlace() {
	e.signatureInStatus = !e.signatureInStatus
	showing := e.signatureShown
	e.hideSignature()
	if e.signatureInStatus {
		e.setStatus("Parameters will show on the status line")
	} else {
		e.setStatus("Parameters will show at the cursor")
	}
	e.menus = e.buildMenus()
	if showing {
		e.signatureHelp(true)
	}
}

// signaturePlaceLabel is what the menu says about the setting.
func (e *Editor) signaturePlaceLabel() string {
	if e.signatureInStatus {
		return "Parameters on status line [on]"
	}
	return "Parameters on status line [off]"
}

// stripTags measures text as it will be drawn, without its colour tags.
func stripTags(text string) string {
	var b strings.Builder
	depth := 0
	for _, r := range text {
		switch {
		case r == '[':
			depth++
		case r == ']' && depth > 0:
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}
