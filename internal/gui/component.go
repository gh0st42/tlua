package gui

import (
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// A component is a control made of others, described by a layout: a Frame
// with parts in it, a few of its properties and events passed on to those
// parts. gui.component (lua/component.lua) defines one with gui.define; what
// it needs from Go is here:
//
//   - the names of the parts belong to the component, so that two of them
//     on a form do not clash, and a form cannot reach into one;
//   - a property of the component can be a property of a part.

// partProp is a property of a part that a property of a component stands for.
type partProp struct {
	part *guiObject
	prop string
}

// hostComponent adds to host what component.lua uses:
//
//	layout(path)                     reads a component's layout file
//	scope(frame)                     makes frame the keeper of its parts' names
//	expose(frame, name)              lets frame.<name> reach the part
//	forward(frame, prop, part, prop) frame.prop is part.prop
func (a *app) hostComponent(L *lua.LState, host *lua.LTable) {
	host.RawSetString("layout", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		if !strings.HasSuffix(path, ".lua") {
			path += ".comp.lua"
		}
		path = a.besideCaller(L, path)
		t, err := a.readLayout(L, path)
		if err != nil {
			L.RaiseError("gui.component: %v", err)
		}
		L.Push(t)
		return 1
	}))
	host.RawSetString("scope", L.NewFunction(func(L *lua.LState) int {
		checkObject(L, 1).scope = true
		return 0
	}))
	host.RawSetString("expose", L.NewFunction(func(L *lua.LState) int {
		o := checkObject(L, 1)
		if o.open == nil {
			o.open = map[string]bool{}
		}
		o.open[L.CheckString(2)] = true
		return 0
	}))
	host.RawSetString("forward", L.NewFunction(func(L *lua.LState) int {
		o := checkObject(L, 1)
		if o.link == nil {
			o.link = map[string]partProp{}
		}
		o.link[L.CheckString(2)] = partProp{part: checkObject(L, 3), prop: L.CheckString(4)}
		return 0
	}))
}
