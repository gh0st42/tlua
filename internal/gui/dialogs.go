package gui

import (
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// buttonSets are what msgbox can offer: the labels shown, and the names it
// returns. Closing the box any other way answers with the last of them.
var buttonSets = map[string]struct{ labels, names []string }{
	"ok":          {[]string{"OK"}, []string{"ok"}},
	"okcancel":    {[]string{"OK", "Cancel"}, []string{"ok", "cancel"}},
	"yesno":       {[]string{"Yes", "No"}, []string{"yes", "no"}},
	"yesnocancel": {[]string{"Yes", "No", "Cancel"}, []string{"yes", "no", "cancel"}},
	"retrycancel": {[]string{"Retry", "Cancel"}, []string{"retry", "cancel"}},
}

// msgbox(message [, buttons [, title]]) shows a message and returns the name
// of the button pressed: "ok", "cancel", "yes", "no" or "retry".
func (a *app) msgbox(L *lua.LState) int {
	message := L.OptString(1, "")
	which := strings.ToLower(L.OptString(2, "ok"))
	title := L.OptString(3, "Message")
	set, ok := buttonSets[which]
	if !ok {
		names := make([]string, 0, len(buttonSets))
		for n := range buttonSets {
			names = append(names, n)
		}
		sort.Strings(names)
		L.ArgError(2, "buttons must be one of "+strings.Join(names, ", "))
	}
	pressed, _, err := dialog(title, message, set.labels, false, "")
	if err != nil {
		L.RaiseError("%s", err.Error())
	}
	if pressed < 0 {
		pressed = len(set.names) - 1
	}
	L.Push(lua.LString(set.names[pressed]))
	return 1
}

// inputbox(prompt [, title [, default]]) asks for a line of text and returns
// it, or nil when the box is cancelled.
func (a *app) inputbox(L *lua.LState) int {
	prompt := L.CheckString(1)
	title := L.OptString(2, "Input")
	deflt := L.OptString(3, "")
	pressed, text, err := dialog(title, prompt, []string{"OK", "Cancel"}, true, deflt)
	if err != nil {
		L.RaiseError("%s", err.Error())
	}
	if pressed != 0 {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LString(text))
	return 1
}

// clipboard() returns the text on the clipboard; clipboard(text) puts text
// there.
func (a *app) clipboard(L *lua.LState) int {
	if L.GetTop() > 0 {
		if err := setClipboard(L.CheckString(1)); err != nil {
			L.RaiseError("%s", err.Error())
		}
		return 0
	}
	text, err := getClipboard()
	if err != nil {
		L.RaiseError("%s", err.Error())
	}
	L.Push(lua.LString(text))
	return 1
}

// fileFunc makes openfile, savefile and choosedir. Each takes a title, or a
// table of title, filter, dir and file (and for openfile, multiple), and
// returns the path chosen or nil; openfile{multiple = true} returns a list.
func (a *app) fileFunc(mode string) lua.LGFunction {
	return func(L *lua.LState) int {
		var opts fileOptions
		multiple := false
		switch v := L.Get(1).(type) {
		case lua.LString:
			opts.title = string(v)
		case *lua.LTable:
			opts.title = lua.LVAsString(v.RawGetString("title"))
			opts.filter = lua.LVAsString(v.RawGetString("filter"))
			opts.dir = lua.LVAsString(v.RawGetString("dir"))
			opts.file = lua.LVAsString(v.RawGetString("file"))
			multiple = lua.LVAsBool(v.RawGetString("multiple"))
		case *lua.LNilType:
		default:
			L.ArgError(1, "title or table of options expected")
		}
		opts.mode = mode
		if mode == "open" && multiple {
			opts.mode = "openmulti"
		}
		paths, err := fileDialog(opts)
		if err != nil {
			L.RaiseError("%s", err.Error())
		}
		switch {
		case len(paths) == 0:
			L.Push(lua.LNil)
		case opts.mode == "openmulti":
			t := L.NewTable()
			for _, p := range paths {
				t.Append(lua.LString(p))
			}
			L.Push(t)
		default:
			L.Push(lua.LString(paths[0]))
		}
		return 1
	}
}

// choosecolor shows a colour chooser, starting at a colour, and returns the
// colour picked as "#rrggbb", or nil when it was cancelled. It takes a
// title, or a table of title and color.
func (a *app) choosecolor(L *lua.LState) int {
	title, start := "Colour", "#ffffff"
	switch v := L.Get(1).(type) {
	case lua.LString:
		title = string(v)
	case *lua.LTable:
		if t := lua.LVAsString(v.RawGetString("title")); t != "" {
			title = t
		}
		if c := lua.LVAsString(v.RawGetString("color")); c != "" {
			start = c
		}
	case *lua.LNilType:
	default:
		L.ArgError(1, "title or table of options expected")
	}
	r, g, b, err := parseColor(start)
	if err != nil {
		L.ArgError(1, err.Error())
	}
	color, ok, err := colorDialog(title, r, g, b)
	if err != nil {
		L.RaiseError("%s", err.Error())
	}
	if !ok {
		L.Push(lua.LNil)
	} else {
		L.Push(lua.LString(color))
	}
	return 1
}

type fileOptions struct {
	mode, title, filter, dir, file string
}

// timerFunc makes after(seconds, fn) and every(seconds, fn). Both return a
// timer whose stop() cancels it; fn returning false stops an every() too.
// Timers run while an event loop does: while a form is up.
func (a *app) timerFunc(repeat bool) lua.LGFunction {
	return func(L *lua.LState) int {
		secs := float64(L.CheckNumber(1))
		fn := L.CheckFunction(2)
		if secs < 0 {
			L.ArgError(1, "seconds must not be negative")
		}
		stopped := false
		t := L.NewTable()
		t.RawSetString("stop", L.NewFunction(func(L *lua.LState) int {
			stopped = true
			return 0
		}))
		var tick func()
		tick = func() {
			if stopped || a.err != nil {
				return
			}
			ret := a.call(fn, t)
			if repeat && !stopped && ret != lua.LFalse {
				addTimeout(secs, tick)
			}
		}
		if err := addTimeout(secs, tick); err != nil {
			L.RaiseError("%s", err.Error())
		}
		L.Push(t)
		return 1
	}
}
