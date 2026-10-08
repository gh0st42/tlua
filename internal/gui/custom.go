package gui

import (
	"fmt"
	"strings"
	"unicode"

	lua "github.com/yuin/gopher-lua"
)

// A script makes controls of its own out of the built-in ones and gives them
// a name with gui.define:
//
//	gui.define{
//	  name = "Rating",
//	  events = {"onChange"},
//	  build = function(parent, opts) ... return control end,
//	}
//
// From then on frame:Rating{...} and gui.Rating{...} work as the built-in
// kinds do. build makes the control in parent (usually a Canvas, or a Frame
// holding others) and returns it. The new control answers to its name in
// errors and tostring, takes the events listed as handlers like any other,
// raises them with self:fire("onChange", ...), and is placed by the left,
// top, width and height it was made with, as the built-in kinds are.

// customKind is one control a script defined.
type customKind struct {
	events []string
	build  *lua.LFunction
}

func (a *app) define(L *lua.LState) int {
	spec := L.CheckTable(1)
	name := lua.LVAsString(spec.RawGetString("name"))
	if err := a.checkKindName(name); err != nil {
		L.RaiseError("%s", err.Error())
	}
	build, ok := spec.RawGetString("build").(*lua.LFunction)
	if !ok {
		L.RaiseError("gui.define: build must be a function that makes the control")
	}
	var events []string
	if t, ok := spec.RawGetString("events").(*lua.LTable); ok {
		for i := 1; i <= t.Len(); i++ {
			e := eventName(lua.LVAsString(t.RawGetInt(i)))
			if !looksLikeEvent(e) {
				L.RaiseError("gui.define: %q is not an event name", lua.LVAsString(t.RawGetInt(i)))
			}
			events = append(events, e)
		}
	}
	a.custom[name] = &customKind{events: events, build: build}
	// gui.Rating{...} goes where gui.Button{...} would.
	mod := L.Get(lua.UpvalueIndex(1)).(*lua.LTable)
	mod.RawSetString(name, L.NewFunction(func(L *lua.LState) int {
		opts := L.OptTable(1, L.NewTable())
		parent := a.lastForm
		if p, ok := toObject(opts.RawGetString("parent")); ok {
			parent = p
		}
		if parent == nil {
			L.RaiseError("gui.%s needs somewhere to go: make a Form first, or give parent =", name)
		}
		L.Push(a.makeCustom(L, name, parent, opts).ud)
		return 1
	}))
	return 0
}

func (a *app) checkKindName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("gui.define: a control needs a name")
	case !unicode.IsUpper(rune(name[0])):
		return fmt.Errorf("gui.define: %q should start with a capital, as Button and Label do", name)
	case strings.IndexFunc(name, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' }) >= 0:
		return fmt.Errorf("gui.define: %q is not a name", name)
	}
	if _, ok := kinds[name]; ok {
		return fmt.Errorf("gui.define: %s is one of the built-in kinds", name)
	}
	if _, ok := a.custom[name]; ok {
		return fmt.Errorf("gui.define: %s is already defined", name)
	}
	return nil
}

// makeCustom runs a defined control's build in parent, and dresses up what
// it returns as the named control.
func (a *app) makeCustom(L *lua.LState, name string, parent *guiObject, opts *lua.LTable) *guiObject {
	ck := a.custom[name]
	if !parent.holds("Label") {
		L.RaiseError("gui: a %s cannot hold a %s", parent.displayKind(), name)
	}
	L.Push(ck.build)
	L.Push(parent.ud)
	L.Push(opts)
	L.Call(2, 1)
	obj, ok := toObject(L.Get(-1))
	L.Pop(1)
	if !ok {
		L.RaiseError("gui: the build of %s must return the control it made", name)
	}
	obj.custom = name
	obj.extra = ck.events
	// Placement and the like, and the handlers, as given.
	var err error
	opts.ForEach(func(k, v lua.LValue) {
		key, ok := k.(lua.LString)
		if !ok || err != nil {
			return
		}
		_, isCommon := common[string(key)]
		switch {
		case key == "width" || key == "height" || isCommon:
			err = obj.set(L, string(key), v)
		case looksLikeEvent(string(key)) && obj.hasEvent(string(key)):
			err = obj.set(L, string(key), v)
		}
	})
	if err != nil {
		L.RaiseError("%s", err.Error())
	}
	return obj
}

// guiFire raises one of an object's events from Lua, the way a defined
// control tells whoever uses it that something happened. It returns what
// the handler returns; with no handler, nothing.
func guiFire(L *lua.LState) int {
	obj := checkObject(L, 1)
	name := eventName(L.CheckString(2))
	if !obj.hasEvent(name) {
		L.RaiseError("%s", obj.noSuchEvent(name).Error())
	}
	fn := obj.events[name]
	if fn == nil {
		return 0
	}
	top := L.GetTop()
	L.Push(fn)
	L.Push(obj.ud)
	for i := 3; i <= top; i++ {
		L.Push(L.Get(i))
	}
	L.Call(top-1, 1)
	return 1
}

// displayKind is what an object is called: its defined name, if it has one.
func (o *guiObject) displayKind() string {
	if o.custom != "" {
		return o.custom
	}
	return o.kind
}
