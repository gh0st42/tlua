package gui

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// A form can be written down as a layout: a Lua table with the form's kind
// and properties, and its controls, each written the same way, in its array
// part. gui.load builds a form from one, gui.dump makes one from a form, and
// gui.save writes one to a file in a steady order, so that a designer can
// rewrite a layout whole without the file churning. Layout files are data:
// they run with nothing in scope.
//
//	return {
//	  kind = "Form", name = "Main", caption = "Hello", width = 320, height = 160,
//	  { kind = "Label", name = "lblName", caption = "Name", left = 16, top = 16 },
//	  { kind = "Button", name = "cmdGreet", caption = "Greet", left = 16, top = 60 },
//	}
//
// Controls with a name can be reached from their form by it: frm.cmdGreet.

// ---------------------------------------------------------------- properties

// propInfo is what a property holds, for gui.kinds and the designer that
// will read it: a Lua type, or one of a few richer ones.
type propInfo struct {
	typ     string // string, number, integer, boolean, color, choice, file, name, list, rows, tree, menu
	choices []string
}

var propSchema = map[string]propInfo{
	"name": {typ: "name"}, "caption": {typ: "string"}, "tooltip": {typ: "string"},
	"visible": {typ: "boolean"}, "enabled": {typ: "boolean"}, "grow": {typ: "boolean"},
	"left": {typ: "integer"}, "top": {typ: "integer"}, "width": {typ: "integer"}, "height": {typ: "integer"},
	"color": {typ: "color"}, "textColor": {typ: "color"}, "fontSize": {typ: "integer"},
	"font":  {typ: "choice", choices: []string{"sans", "serif", "mono"}},
	"align": {typ: "choice", choices: []string{"left", "center", "right"}},
	"text":  {typ: "string"}, "path": {typ: "string"}, "file": {typ: "file"}, "image": {typ: "file"},
	"transparent": {typ: "boolean"}, "lineNumbers": {typ: "boolean"}, "acceptsTab": {typ: "boolean"},
	"syntax": {typ: "choice", choices: []string{"lua"}}, "tabIndex": {typ: "integer"},
	"multiLine": {typ: "boolean"}, "password": {typ: "boolean"}, "readOnly": {typ: "boolean"},
	"default": {typ: "boolean"}, "vertical": {typ: "boolean"}, "resizable": {typ: "boolean"},
	"checked": {typ: "boolean"}, "fit": {typ: "boolean"}, "editable": {typ: "boolean"},
	"selected": {typ: "integer"}, "min": {typ: "number"}, "max": {typ: "number"},
	"step": {typ: "number"}, "value": {typ: "number"},
	"items": {typ: "list"}, "columns": {typ: "list"}, "rows": {typ: "rows"}, "columnWidths": {typ: "list"},
}

// styleProps every kind takes, with no default of their own.
var styleProps = []string{"color", "textColor", "font", "fontSize"}

// listProps are the table properties a kind has, which start empty.
var listProps = map[string][]string{
	"ComboBox": {"items"}, "ListBox": {"items"}, "Tree": {"items"}, "Menu": {"items"},
	"Table": {"columns", "rows", "columnWidths"},
}

// propsOf lists a kind's properties, in the order a layout writes them.
func propsOf(kind string) []string {
	spec := kinds[kind]
	names := []string{"name"}
	for k := range common {
		names = append(names, k)
	}
	names = append(names, "width", "height")
	names = append(names, styleProps...)
	for k := range spec.props {
		if k != "color" { // a Canvas's background, already among the styles
			names = append(names, k)
		}
	}
	names = append(names, listProps[kind]...)
	return orderProps(names)
}

// leading are the properties a layout writes first, in this order; the rest
// follow alphabetically.
var leading = []string{"kind", "name", "caption", "text", "left", "top", "width", "height"}

func orderProps(names []string) []string {
	rank := func(n string) int {
		for i, l := range leading {
			if l == n {
				return i
			}
		}
		return len(leading)
	}
	sort.SliceStable(names, func(i, j int) bool {
		ri, rj := rank(names[i]), rank(names[j])
		if ri != rj {
			return ri < rj
		}
		return names[i] < names[j]
	})
	out := names[:0]
	for i, n := range names {
		if i == 0 || n != names[i-1] {
			out = append(out, n)
		}
	}
	return out
}

// propType is a property's type, falling back to its default's.
func propType(name string, def lua.LValue) propInfo {
	if p, ok := propSchema[name]; ok {
		return p
	}
	switch def.(type) {
	case lua.LBool:
		return propInfo{typ: "boolean"}
	case lua.LNumber:
		return propInfo{typ: "number"}
	case *lua.LTable:
		return propInfo{typ: "list"}
	}
	return propInfo{typ: "string"}
}

// defaultOf is a kind's default for a property: nil for the styles, which
// have none.
func defaultOf(kind, name string) lua.LValue {
	spec := kinds[kind]
	if name == "width" {
		return lua.LNumber(spec.w)
	}
	if name == "height" {
		return lua.LNumber(spec.h)
	}
	if v, ok := spec.props[name]; ok {
		return v
	}
	if v, ok := common[name]; ok {
		return v
	}
	if name == "name" {
		return lua.LString("")
	}
	return lua.LNil
}

// ---------------------------------------------------------------- names

var luaPosition = regexp.MustCompile(`^[^:\n]*:\d+: `)

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var luaKeywords = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true, "end": true,
	"false": true, "for": true, "function": true, "if": true, "in": true, "local": true,
	"nil": true, "not": true, "or": true, "repeat": true, "return": true, "then": true,
	"true": true, "until": true, "while": true,
}

// checkName refuses a name a control cannot go by: one that is not an
// identifier, or that frm.<name> would read as something else.
func (o *guiObject) checkName(name string) error {
	switch {
	case name == "":
		return nil
	case !identifier.MatchString(name) || luaKeywords[name]:
		return fmt.Errorf("gui: %q cannot be a name: a name is a Lua identifier, as cmdOK is", name)
	case o.hasMethod(name):
		return fmt.Errorf("gui: %q cannot be a name: it is a method", name)
	case looksLikeEvent(name):
		return fmt.Errorf("gui: %q cannot be a name: it reads as an event", name)
	}
	if _, ok := propSchema[name]; ok {
		return fmt.Errorf("gui: %q cannot be a name: it is a property", name)
	}
	if name == "kind" || name == "parent" {
		return fmt.Errorf("gui: %q cannot be a name: layouts use it", name)
	}
	return nil
}

// named lists the names in a subtree, for checking and registering together.
func named(o *guiObject, out map[string]*guiObject) map[string]*guiObject {
	if n := propString(o, "name"); n != "" && o.kind != "Form" {
		out[n] = o
	}
	for _, c := range o.children {
		named(c, out)
	}
	return out
}

// fitNames checks that a subtree's names are free on form f.
func fitNames(f *guiObject, sub *guiObject) error {
	if f == nil {
		return nil
	}
	for n, o := range named(sub, map[string]*guiObject{}) {
		if other, ok := f.names[n]; ok && other != o {
			return fmt.Errorf("gui: the form already has a control named %s", n)
		}
	}
	return nil
}

func register(f *guiObject, sub *guiObject) {
	if f == nil {
		return
	}
	if f.names == nil {
		f.names = map[string]*guiObject{}
	}
	for n, o := range named(sub, map[string]*guiObject{}) {
		f.names[n] = o
	}
}

func unregister(f *guiObject, sub *guiObject) {
	if f == nil {
		return
	}
	for n, o := range named(sub, map[string]*guiObject{}) {
		if f.names[n] == o {
			delete(f.names, n)
		}
	}
}

// rename is setting a control's name: free on its form, and moved there.
func (o *guiObject) rename(name string) error {
	if err := o.checkName(name); err != nil {
		return err
	}
	f := o.form()
	if f == nil || f == o {
		return nil
	}
	if other, ok := f.names[name]; ok && other != o && name != "" {
		return fmt.Errorf("gui: the form already has a control named %s", name)
	}
	if old := propString(o, "name"); old != "" && f.names[old] == o {
		delete(f.names, old)
	}
	if name != "" {
		if f.names == nil {
			f.names = map[string]*guiObject{}
		}
		f.names[name] = o
	}
	return nil
}

// guiFind is form:find(name): the control of that name on the object's
// form, or nil.
func guiFind(L *lua.LState) int {
	obj := checkObject(L, 1)
	name := L.CheckString(2)
	if f := obj.form(); f != nil {
		if c, ok := f.names[name]; ok {
			L.Push(c.ud)
			return 1
		}
	}
	L.Push(lua.LNil)
	return 1
}

// ---------------------------------------------------------------- load

// load is gui.load(layout [, parent]): a layout table, or the path of a file
// returning one. A path without .lua at the end means its .form.lua file;
// either is found next to the script that names it, or in a fused
// program's archive. A layout whose top is not a Form is built in parent.
func (a *app) load(L *lua.LState) int {
	var parent *guiObject
	if L.GetTop() >= 2 && L.Get(2) != lua.LNil {
		parent = checkObject(L, 2)
	}
	var t *lua.LTable
	where := "layout"
	switch v := L.Get(1).(type) {
	case *lua.LTable:
		t = v
	case lua.LString:
		path := string(v)
		if !strings.HasSuffix(path, ".lua") {
			path += ".form.lua"
		}
		path = a.besideCaller(L, path)
		where = path
		var err error
		if t, err = a.readLayout(L, path); err != nil {
			L.RaiseError("gui.load: %v", err)
		}
	default:
		L.ArgError(1, "a layout table, or the path of a layout file")
	}
	// Building raises errors of its own from deep inside; they are caught
	// here to say which layout and which part of it they came from.
	var obj *guiObject
	var buildErr error
	at := ""
	build := L.NewFunction(func(L *lua.LState) int {
		obj, buildErr = a.buildLayout(L, t, parent, top, &at)
		return 0
	})
	err := L.CallByParam(lua.P{Fn: build, NRet: 0, Protect: true})
	if err == nil {
		err = buildErr
	}
	if err != nil {
		msg := err.Error()
		if apiErr, ok := err.(*lua.ApiError); ok {
			msg = lua.LVAsString(apiErr.Object)
		}
		// The error is about the layout, not about the Lua line that
		// called gui.load, which the message is about to say anyway.
		msg = luaPosition.ReplaceAllString(msg, "")
		msg = strings.TrimPrefix(msg, "gui: ")
		if at != "" {
			L.RaiseError("gui.load %s, at %s: %s", where, at, msg)
		}
		L.RaiseError("gui.load %s: %s", where, msg)
	}
	L.Push(obj.ud)
	return 1
}

// readLayout runs a layout file with nothing in scope, and returns the
// table it gives back.
func (a *app) readLayout(L *lua.LState, path string) (*lua.LTable, error) {
	var data []byte
	var err error
	if a.read != nil {
		data, err = a.read(path)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	fn, err := L.Load(strings.NewReader(string(data)), "@"+path)
	if err != nil {
		return nil, err
	}
	L.SetFEnv(fn, L.NewTable())
	if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}); err != nil {
		return nil, err
	}
	ret := L.Get(-1)
	L.Pop(1)
	t, ok := ret.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("%s returns a %s, not a layout table", path, ret.Type())
	}
	return t, nil
}

// top is the path buildLayout starts from: the top of a layout, which
// errors leave unnamed for the parts under it.
const top = "\x00"

// buildLayout makes one object of a layout, and then its controls in turn.
// at keeps track of where in the layout it is, for errors.
func (a *app) buildLayout(L *lua.LState, t *lua.LTable, parent *guiObject, path string, at *string) (*guiObject, error) {
	kind, ok := t.RawGetString("kind").(lua.LString)
	label := string(kind)
	if n, ok := t.RawGetString("name").(lua.LString); ok && n != "" {
		label = string(n)
	}
	// The top of the layout goes without saying.
	here := ""
	switch {
	case path == top:
		*at = label
	case path == "":
		here, *at = label, label
	default:
		here = path + "." + label
		*at = here
	}
	if !ok || kind == "" {
		return nil, fmt.Errorf("gui: every part of a layout says its kind, as kind = \"Button\"")
	}
	opts := L.NewTable()
	t.ForEach(func(k, v lua.LValue) {
		if s, ok := k.(lua.LString); ok && s != "kind" && s != "parent" {
			opts.RawSetString(string(s), v)
		}
	})

	if _, builtIn := kinds[string(kind)]; !builtIn {
		if err := a.findControl(L, string(kind)); err != nil {
			return nil, err
		}
	}

	var obj *guiObject
	if _, ok := a.custom[string(kind)]; ok {
		if parent == nil {
			return nil, fmt.Errorf("gui: a layout's top is a Form")
		}
		if t.Len() > 0 {
			return nil, fmt.Errorf("gui: a %s builds its own controls; its layout has none", kind)
		}
		obj = a.makeCustom(L, string(kind), parent, opts)
	} else {
		spec, ok := kinds[string(kind)]
		if !ok {
			return nil, fmt.Errorf("gui: there is no kind %q", string(kind))
		}
		switch {
		case kind == "Form" && parent != nil:
			return nil, fmt.Errorf("gui: a Form cannot be inside anything")
		case kind != "Form" && parent == nil:
			return nil, fmt.Errorf("gui: a layout's top is a Form, unless gui.load is given a container to build it in")
		}
		obj = a.make(L, string(kind), spec, opts, parent)
	}

	for i := 1; i <= t.Len(); i++ {
		child, ok := t.RawGetInt(i).(*lua.LTable)
		if !ok {
			*at = here
			if here == "" {
				*at = label
			}
			return nil, fmt.Errorf("gui: item %d of %s is a %s, not a control", i, label, t.RawGetInt(i).Type())
		}
		if _, err := a.buildLayout(L, child, obj, here, at); err != nil {
			return nil, err
		}
	}
	*at = ""
	return obj, nil
}

// findControl looks for a control a layout uses that is not defined yet, as
// a project keeps them: controls/<kind>.lua, which says gui.define. It is
// required the first time it is needed, so a program whose forms use a
// control of its own needs nothing else to find it, fused or not. A file
// there that fails to run says why; no file is no error, and the layout's
// kind is then reported as unknown.
func (a *app) findControl(L *lua.LState, kind string) error {
	if _, ok := a.custom[kind]; ok || !identifier.MatchString(kind) {
		return nil
	}
	module := "controls." + kind
	err := L.CallByParam(lua.P{Fn: L.GetGlobal("require"), NRet: 0, Protect: true}, lua.LString(module))
	if err == nil {
		return nil
	}
	msg := err.Error()
	if apiErr, ok := err.(*lua.ApiError); ok {
		msg = lua.LVAsString(apiErr.Object)
	}
	if strings.Contains(msg, "module "+module+" not found") {
		return nil
	}
	return fmt.Errorf("gui: %s: %s", module, strings.TrimPrefix(luaPosition.ReplaceAllString(msg, ""), "gui: "))
}

// ---------------------------------------------------------------- dump

// dump is gui.dump(obj): the layout that would build obj as it is now,
// with what is on screen read back and only what differs from the defaults
// written down. Handlers are code and are left out, as are fields of the
// script's own.
func (a *app) dump(L *lua.LState) int {
	L.Push(dumpObject(L, checkObject(L, 1)))
	return 1
}

func dumpObject(L *lua.LState, o *guiObject) *lua.LTable {
	t := L.NewTable()
	t.RawSetString("kind", lua.LString(o.displayKind()))
	for _, name := range dumpedProps(o) {
		v := o.get(name)
		if g, ok := o.given[name]; ok {
			v = lua.LString(g) // as the script wrote it, not as found
		}
		if same(v, o.defaultFor(name)) {
			continue
		}
		t.RawSetString(name, copyData(L, v))
	}
	if o.custom == "" {
		for _, c := range o.children {
			t.Append(dumpObject(L, c))
		}
	}
	return t
}

// dumpedProps are the properties a layout keeps for an object: a defined
// control's are its placement and the props it was defined with.
func dumpedProps(o *guiObject) []string {
	if o.custom != "" {
		names := []string{"name", "left", "top", "width", "height", "visible", "enabled", "tooltip", "caption"}
		if ck := o.app.custom[o.custom]; ck != nil {
			for n := range ck.props {
				names = append(names, n)
			}
		}
		return orderProps(names)
	}
	var out []string
	for _, n := range propsOf(o.kind) {
		switch {
		case o.kind == "Form" && n == "visible":
			// Whether it is up is the program's business.
		case o.kind == "Form" && (n == "left" || n == "top") && !o.placed:
			// Left alone, it opens in the middle of the screen.
		case o.kind == "Page" && (n == "left" || n == "top" || n == "width" || n == "height"):
			// A page is wherever its Tabs puts it.
		default:
			out = append(out, n)
		}
	}
	return out
}

func (o *guiObject) defaultFor(name string) lua.LValue {
	if o.custom != "" {
		if ck := o.app.custom[o.custom]; ck != nil {
			if p, ok := ck.props[name]; ok {
				return p.def
			}
		}
	}
	return defaultOf(o.kind, name)
}

// same says whether a value goes without saying: its default, nothing, or
// an empty list.
func same(v, def lua.LValue) bool {
	if v == lua.LNil {
		return true
	}
	if t, ok := v.(*lua.LTable); ok {
		k, _ := t.Next(lua.LNil)
		return k == lua.LNil
	}
	return v == def
}

// copyData copies a property's value for a layout, leaving out functions,
// which are code.
func copyData(L *lua.LState, v lua.LValue) lua.LValue {
	t, ok := v.(*lua.LTable)
	if !ok {
		return v
	}
	c := L.NewTable()
	t.ForEach(func(k, x lua.LValue) {
		if _, isFn := x.(*lua.LFunction); isFn {
			return
		}
		c.RawSet(k, copyData(L, x))
	})
	return c
}

// ---------------------------------------------------------------- save

// save is gui.save(obj or layout, path): the layout of obj, or a layout
// table as it is, written as a Lua file in a steady order.
func (a *app) save(L *lua.LState) int {
	var layout *lua.LTable
	if t, ok := L.Get(1).(*lua.LTable); ok {
		layout = t
	} else {
		layout = dumpObject(L, checkObject(L, 1))
	}
	path := L.CheckString(2)
	text := "-- A layout, written by gui.save: gui.load builds it.\nreturn " + formatLayout(layout, "") + "\n"
	if err := os.WriteFile(path, []byte(text), 0o666); err != nil {
		L.RaiseError("gui.save: %v", err)
	}
	return 0
}

// formatLayout writes one part of a layout and its controls.
func formatLayout(t *lua.LTable, indent string) string {
	var keys []string
	t.ForEach(func(k, _ lua.LValue) {
		if s, ok := k.(lua.LString); ok {
			keys = append(keys, string(s))
		}
	})
	keys = orderProps(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, key(k)+" = "+formatValue(t.RawGetString(k)))
	}
	head := "{ " + strings.Join(parts, ", ")
	if t.Len() == 0 {
		return head + " }"
	}
	var b strings.Builder
	b.WriteString(head + ",\n")
	for i := 1; i <= t.Len(); i++ {
		if c, ok := t.RawGetInt(i).(*lua.LTable); ok {
			b.WriteString(indent + "  " + formatLayout(c, indent+"  ") + ",\n")
		}
	}
	b.WriteString(indent + "}")
	return b.String()
}

// formatValue writes a value as Lua: a list in order, then its named
// fields in alphabetical order.
func formatValue(v lua.LValue) string {
	switch x := v.(type) {
	case lua.LString:
		return quote(string(x))
	case lua.LNumber:
		f := float64(x)
		if f == float64(int64(f)) {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'g', -1, 64)
	case lua.LBool:
		return strconv.FormatBool(bool(x))
	case *lua.LTable:
		var parts []string
		n := x.Len()
		for i := 1; i <= n; i++ {
			parts = append(parts, formatValue(x.RawGetInt(i)))
		}
		var named []string
		x.ForEach(func(k, _ lua.LValue) {
			if s, ok := k.(lua.LString); ok {
				named = append(named, string(s))
			} else if num, ok := k.(lua.LNumber); ok && (int(num) < 1 || int(num) > n || float64(int(num)) != float64(num)) {
				named = append(named, "\x00"+formatValue(num))
			}
		})
		sort.Strings(named)
		for _, k := range named {
			if strings.HasPrefix(k, "\x00") {
				num, _ := strconv.ParseFloat(k[1:], 64)
				parts = append(parts, "["+k[1:]+"] = "+formatValue(x.RawGet(lua.LNumber(num))))
				continue
			}
			parts = append(parts, key(k)+" = "+formatValue(x.RawGetString(k)))
		}
		if len(parts) == 0 {
			return "{}"
		}
		return "{ " + strings.Join(parts, ", ") + " }"
	}
	return "nil"
}

func key(k string) string {
	if identifier.MatchString(k) && !luaKeywords[k] {
		return k
	}
	return "[" + quote(k) + "]"
}

// quote writes a string as a Lua literal that reads back the same.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\t':
			b.WriteString(`\t`)
		case c < 32 || c == 127:
			fmt.Fprintf(&b, `\%03d`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ---------------------------------------------------------------- kinds

// kindsTable is gui.kinds(): every kind there is, built in or defined, with
// its properties (type, default, whether it is fixed once made, the choices
// if it has them), its events, what it can hold, and its size when made
// without one. It is a fresh table each time; changing it changes nothing.
func (a *app) kindsTable(L *lua.LState) int {
	out := L.NewTable()
	for name, spec := range kinds {
		k := L.NewTable()
		props := L.NewTable()
		for _, p := range propsOf(name) {
			if name == "Form" && p == "visible" {
				continue
			}
			def := defaultOf(name, p)
			info := propType(p, def)
			if p == "items" && name == "Tree" {
				info.typ = "tree"
			} else if p == "items" && name == "Menu" {
				info.typ = "menu"
			}
			props.RawSetString(p, propEntry(L, p, info, def, isFixed(spec, p)))
		}
		k.RawSetString("props", props)
		k.RawSetString("events", stringList(L, append(append([]string{}, spec.events...), commonEvents...)))
		k.RawSetString("holds", stringList(L, spec.holds))
		k.RawSetString("width", lua.LNumber(spec.w))
		k.RawSetString("height", lua.LNumber(spec.h))
		out.RawSetString(name, k)
	}
	for name, ck := range a.custom {
		k := L.NewTable()
		props := L.NewTable()
		for _, p := range []string{"name", "left", "top", "width", "height", "visible", "enabled", "tooltip", "caption"} {
			def := defaultOf("Label", p)
			props.RawSetString(p, propEntry(L, p, propType(p, def), def, false))
		}
		for p, info := range ck.props {
			props.RawSetString(p, propEntry(L, p, propInfo{typ: info.typ, choices: info.choices}, info.def, false))
		}
		k.RawSetString("props", props)
		k.RawSetString("events", stringList(L, append(append([]string{}, ck.events...), commonEvents...)))
		k.RawSetString("holds", L.NewTable())
		k.RawSetString("defined", lua.LTrue)
		out.RawSetString(name, k)
	}
	L.Push(out)
	return 1
}

func isFixed(spec *kind, p string) bool {
	if p == "grow" {
		return true
	}
	for _, f := range spec.fixed {
		if f == p {
			return true
		}
	}
	return false
}

func propEntry(L *lua.LState, name string, info propInfo, def lua.LValue, fixed bool) *lua.LTable {
	e := L.NewTable()
	e.RawSetString("type", lua.LString(info.typ))
	if def != lua.LNil {
		e.RawSetString("default", copyData(L, def))
	} else if t := info.typ; t == "list" || t == "rows" {
		e.RawSetString("default", L.NewTable())
	}
	if fixed {
		e.RawSetString("fixed", lua.LTrue)
	}
	if len(info.choices) > 0 {
		e.RawSetString("choices", stringList(L, info.choices))
	}
	return e
}

func stringList(L *lua.LState, xs []string) *lua.LTable {
	t := L.NewTable()
	for _, x := range xs {
		t.Append(lua.LString(x))
	}
	return t
}
