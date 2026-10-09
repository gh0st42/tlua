package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	lua "github.com/yuin/gopher-lua"
)

// The gui module describes a window as a tree of objects: a Form holding
// controls, some of which (Frame, Tabs and its Pages) hold controls of their
// own. Objects keep their properties in a table of their own until they are
// put on screen, and are the widgets' from then on, so a script can build and
// change a form the same way before and after showing it. Everything here is
// independent of the backend; fltk.go puts the tree on screen and nofltk.go
// stands in where there is no FLTK to do it with.

// kind is what one sort of object accepts.
type kind struct {
	// w and h are the size a control gets when the script gives none.
	w, h int
	// events are the handlers it can raise, as their property names.
	events []string
	// holds lists the kinds that can be made inside it; nil for a control.
	holds []string
	// props are its own properties, with their defaults, beyond the ones
	// every object has.
	props map[string]lua.LValue
	// fixed are properties that pick the widget itself, so they can only be
	// given while the object is not yet on screen.
	fixed []string
	// aliases are other names for its properties: a Label's text is its
	// caption, a CheckBox's value is whether it is checked.
	aliases map[string]string
}

// controls are what a Form, Frame or Page can hold.
var controls = []string{
	"Label", "Button", "TextBox", "CheckBox", "RadioButton", "ComboBox",
	"ListBox", "Tree", "Table", "Slider", "Spinner", "ProgressBar", "Image",
	"Canvas", "Frame", "Panel", "Scroll", "Splitter", "Tabs",
}

// commonEvents are the handlers every kind can raise: something dropped on
// it, or the user starting to drag out of it.
var commonEvents = []string{"onDrop", "onDrag"}

var kinds = map[string]*kind{
	"Form": {
		w: 360, h: 240,
		events: []string{"onClose", "onUnload", "onKey", "onResize"},
		holds:  append([]string{"Menu"}, controls...),
		props:  map[string]lua.LValue{"resizable": lua.LFalse},
		fixed:  []string{"resizable"},
	},
	"Label": {
		w: 120, h: 28,
		props:   map[string]lua.LValue{"align": lua.LString("left"), "image": lua.LString("")},
		aliases: map[string]string{"text": "caption"},
	},
	"Button": {
		w: 120, h: 28,
		events: []string{"onClick"},
		props:  map[string]lua.LValue{"tabIndex": lua.LNumber(0), "default": lua.LFalse, "image": lua.LString("")},
		fixed:  []string{"default"},
	},
	"TextBox": {
		w: 120, h: 28,
		events: []string{"onChange", "onKey", "onHover"},
		props: map[string]lua.LValue{"tabIndex": lua.LNumber(0),
			"text": lua.LString(""), "multiLine": lua.LFalse,
			"password": lua.LFalse, "readOnly": lua.LFalse,
			// For editing code: a multi-line box with line numbers, Lua's
			// colours, and Tab and Enter that indent.
			"lineNumbers": lua.LFalse, "syntax": lua.LString(""), "acceptsTab": lua.LFalse,
		},
		fixed:   []string{"multiLine", "password", "readOnly", "lineNumbers", "syntax", "acceptsTab"},
		aliases: map[string]string{"value": "text"},
	},
	"CheckBox": {
		w: 120, h: 28,
		events:  []string{"onChange"},
		props:   map[string]lua.LValue{"tabIndex": lua.LNumber(0), "checked": lua.LFalse},
		aliases: map[string]string{"value": "checked"},
	},
	"RadioButton": {
		w: 120, h: 28,
		events:  []string{"onChange"},
		props:   map[string]lua.LValue{"tabIndex": lua.LNumber(0), "checked": lua.LFalse},
		aliases: map[string]string{"value": "checked"},
	},
	"ComboBox": {
		w: 120, h: 28,
		events: []string{"onChange"},
		props:  map[string]lua.LValue{"tabIndex": lua.LNumber(0), "selected": lua.LNumber(0)},
	},
	"ListBox": {
		w: 160, h: 120,
		events: []string{"onChange", "onDoubleClick"},
		props:  map[string]lua.LValue{"tabIndex": lua.LNumber(0), "selected": lua.LNumber(0)},
	},
	"Tree": {
		w: 200, h: 160,
		events: []string{"onChange", "onDoubleClick", "onToggle"},
		props:  map[string]lua.LValue{"tabIndex": lua.LNumber(0), "path": lua.LString("")},
	},
	"Table": {
		w: 320, h: 160,
		events: []string{"onChange", "onDoubleClick", "onStartEdit", "onEdit", "onEditButton"},
		props:  map[string]lua.LValue{"tabIndex": lua.LNumber(0), "selected": lua.LNumber(0), "editable": lua.LFalse},
	},
	"Canvas": {
		w: 200, h: 150,
		events: []string{"onDraw", "onMouseDown", "onMouseUp", "onMouseMove", "onMouseDrag", "onMouseWheel", "onMouseEnter", "onMouseLeave", "onKey"},
		props:  map[string]lua.LValue{"tabIndex": lua.LNumber(0), "color": lua.LString("#ffffff"), "transparent": lua.LFalse},
		fixed:  []string{"transparent"},
	},
	"Slider": {
		w: 160, h: 28,
		events: []string{"onChange"},
		props: map[string]lua.LValue{"tabIndex": lua.LNumber(0),
			"min": lua.LNumber(0), "max": lua.LNumber(100), "step": lua.LNumber(1),
			"value": lua.LNumber(0), "vertical": lua.LFalse,
		},
		fixed: []string{"vertical"},
	},
	"Spinner": {
		w: 80, h: 28,
		events: []string{"onChange"},
		props: map[string]lua.LValue{"tabIndex": lua.LNumber(0),
			"min": lua.LNumber(0), "max": lua.LNumber(100), "step": lua.LNumber(1),
			"value": lua.LNumber(0),
		},
	},
	"ProgressBar": {
		w: 160, h: 24,
		props: map[string]lua.LValue{
			"min": lua.LNumber(0), "max": lua.LNumber(100), "value": lua.LNumber(0),
		},
	},
	"Image": {
		w: 100, h: 100,
		props: map[string]lua.LValue{"file": lua.LString(""), "fit": lua.LFalse},
	},
	"Frame": {
		w: 200, h: 120,
		holds: controls,
	},
	// A Panel is a Frame with no border or caption: a group of controls. It
	// can hold a Menu as well, as a stand-in for a form.
	"Panel": {
		w: 200, h: 120,
		holds: append([]string{"Menu"}, controls...),
	},
	// A Scroll shows part of what it holds, with scrollbars for the rest.
	"Scroll": {
		w: 200, h: 120,
		holds: controls,
	},
	// A Splitter's controls tile it edge to edge, and the user can drag the
	// lines between them.
	"Splitter": {
		w: 400, h: 300,
		holds: controls,
	},
	"Tabs": {
		w: 320, h: 200,
		events: []string{"onChange"},
		holds:  []string{"Page"},
		props:  map[string]lua.LValue{"tabIndex": lua.LNumber(0), "selected": lua.LNumber(1)},
	},
	"Page": {
		holds: controls,
	},
	"Menu": {
		// A width of 0 is the whole width of what holds it.
		w: 0, h: 25,
		events: []string{"onClick"},
	},
}

// common are the properties every object has.
var common = map[string]lua.LValue{
	"name":    lua.LString(""),
	"caption": lua.LString(""),
	"visible": lua.LTrue,
	"enabled": lua.LTrue,
	"left":    lua.LNumber(0),
	"top":     lua.LNumber(0),
	"grow":    lua.LFalse,
	"tooltip": lua.LString(""),
}

// methods are what every object answers to beyond its properties; which of
// them a kind has is decided by methodsOf.
var methods map[string]lua.LGFunction

// The methods are set apart from their declaration because they reach back
// into it: add adopts, which sets, which asks whether a name is a method.
func init() {
	methods = map[string]lua.LGFunction{
		"show":      guiShow,
		"showModal": guiShowModal,
		"close":     guiClose,
		"add":       guiAdd,
		"on":        guiOn,
		"focus":     guiFocus,
		"redraw":    guiRedraw,
		"select":    guiSelect,
		"fire":      guiFire,
		"find":      guiFind,
		"remove":    guiRemove,
		"raise":     guiRaise,
		"lower":     guiLower,
		"edit":      guiEdit,
		"insert":    guiInsert,
		"pointAt":   guiPointAt,
		"editing":   guiEditing,
	}
}

// app is the gui module as one Lua state sees it. Each state that requires
// the module has its own, so nothing here outlives the state or is shared
// between two of them.
type app struct {
	L *lua.LState
	// lastForm is where gui.Button{} and the like go when they are given no
	// parent: the Form made most recently.
	lastForm *guiObject
	// forms are the forms with a window: shown, or closed and not yet freed.
	forms []*guiObject
	// err is the first error raised by a handler. It stops whatever event
	// loop is running, and is reported from where that loop was started.
	err error
	// booted is set by bootgui(): show() then puts a form up and returns,
	// and the loop runs once the script has finished.
	booted  bool
	methods map[string]*lua.LFunction
	// custom are the controls the script defined with gui.define.
	custom map[string]*customKind
	// read is where a fused program's own files come from, archive first;
	// nil for a program on disk.
	read func(string) ([]byte, error)
}

type guiObject struct {
	app      *app
	kind     string
	spec     *kind
	ud       *lua.LUserData
	props    map[string]lua.LValue
	events   map[string]*lua.LFunction
	parent   *guiObject
	children []*guiObject
	// widget is the backend's, from the moment the object is on screen.
	widget any
	// placed is set once the script gives a position, so that a Form left
	// alone opens in the middle of the screen rather than at 0,0.
	placed bool
	// mouse is the backend's note of a press that may become a drag, and of
	// a drop on its way.
	mouse struct {
		armed, dropping bool
		x, y            int
		// hover counts the mouse's moves over a TextBox, so that only the
		// last one's rest is a hover; hovered is set while one is shown.
		hover   int
		hovered bool
	}
	// state is anything else the backend keeps for the object.
	state any
	// custom is the name a control made by gui.define goes by, and extra
	// the events it was defined with.
	custom string
	extra  []string
	// names are a Form's controls by name, for frm.<name> and find.
	names map[string]*guiObject
	// given are file properties (an Image's file, a Button's image) as the
	// script wrote them, for a layout; the properties themselves are where
	// the files were found.
	given map[string]string
}

const guiObjectType = "tlua.gui.object"

// Open installs the gui module into a Lua state.
func Open(L *lua.LState) *app {
	a := &app{L: L, methods: map[string]*lua.LFunction{}, custom: map[string]*customKind{}}
	L.PreloadModule("gui", a.open)
	return a
}

func (a *app) open(L *lua.LState) int {
	mod := L.NewTable()
	for name, spec := range kinds {
		if name == "Page" {
			continue // a Page is only ever made by its Tabs
		}
		name, spec := name, spec
		mod.RawSetString(name, L.NewFunction(func(L *lua.LState) int {
			var parent *guiObject
			if name != "Form" {
				parent = a.lastForm
			}
			L.Push(a.make(L, name, spec, L.OptTable(1, L.NewTable()), parent).ud)
			return 1
		}))
	}
	L.SetFuncs(mod, map[string]lua.LGFunction{
		"msgbox":      a.msgbox,
		"inputbox":    a.inputbox,
		"openfile":    a.fileFunc("open"),
		"savefile":    a.fileFunc("save"),
		"choosedir":   a.fileFunc("dir"),
		"choosecolor": a.choosecolor,
		"clipboard":   a.clipboard,
		"load":        a.load,
		"dump":        a.dump,
		"save":        a.save,
		"kinds":       a.kindsTable,
		"spawn":       a.spawn,
		"after":       a.timerFunc(false),
		"every":       a.timerFunc(true),
	})
	mod.RawSetString("define", L.NewClosure(a.define, mod))
	if exe, err := os.Executable(); err == nil {
		// The tlua running this, for running another program with it.
		mod.RawSetString("interpreter", lua.LString(exe))
	}
	mod.RawSetString("_DESCRIPTION", lua.LString("Desktop GUI module for tlua"))

	mt := L.NewTypeMetatable(guiObjectType)
	L.SetFuncs(mt, map[string]lua.LGFunction{
		"__index":    guiIndex,
		"__newindex": guiNewIndex,
		"__tostring": guiToString,
	})
	L.Push(mod)
	return 1
}

// make builds one object of the given kind from the table a script wrote,
// inside parent if there is one.
func (a *app) make(L *lua.LState, name string, spec *kind, opts *lua.LTable, parent *guiObject) *guiObject {
	obj := &guiObject{
		app:    a,
		kind:   name,
		spec:   spec,
		props:  map[string]lua.LValue{},
		events: map[string]*lua.LFunction{},
	}
	for k, v := range common {
		obj.props[k] = v
	}
	obj.props["width"] = lua.LNumber(spec.w)
	obj.props["height"] = lua.LNumber(spec.h)
	for k, v := range spec.props {
		obj.props[k] = v
	}
	switch name {
	case "ComboBox", "ListBox", "Menu", "Tree":
		obj.props["items"] = L.NewTable()
	case "Table":
		obj.props["columns"] = L.NewTable()
		obj.props["rows"] = L.NewTable()
	}
	obj.ud = L.NewUserData()
	obj.ud.Value = obj
	L.SetMetatable(obj.ud, L.GetTypeMetatable(guiObjectType))

	// A Menu is written as a list of its items.
	if name == "Menu" && opts.Len() > 0 {
		items := L.NewTable()
		for i := 1; i <= opts.Len(); i++ {
			items.Append(opts.RawGetInt(i))
		}
		obj.props["items"] = items
	}
	var err error
	opts.ForEach(func(key, value lua.LValue) {
		k, ok := key.(lua.LString)
		if !ok || err != nil {
			return
		}
		if k == "parent" {
			p, ok := toObject(value)
			if !ok {
				err = fmt.Errorf("gui: parent must be a gui object")
				return
			}
			parent = p
			return
		}
		err = obj.set(L, string(k), value)
	})
	if err != nil {
		L.RaiseError("%s", err.Error())
	}

	if name == "Form" {
		a.lastForm = obj
	} else if parent != nil {
		if err := parent.adopt(obj); err != nil {
			L.RaiseError("%s", err.Error())
		}
	}
	return obj
}

func toObject(v lua.LValue) (*guiObject, bool) {
	if ud, ok := v.(*lua.LUserData); ok {
		if obj, ok := ud.Value.(*guiObject); ok {
			return obj, true
		}
	}
	return nil, false
}

func checkObject(L *lua.LState, n int) *guiObject {
	obj, ok := toObject(L.Get(n))
	if !ok {
		L.ArgError(n, "gui object expected")
	}
	return obj
}

func (o *guiObject) holds(kind string) bool {
	for _, k := range o.spec.holds {
		if k == kind {
			return true
		}
	}
	return false
}

// adopt makes child one of o's, taking it from wherever it was before.
func (o *guiObject) adopt(child *guiObject) error {
	if !o.holds(child.kind) {
		return fmt.Errorf("gui: a %s cannot hold a %s", o.kind, child.kind)
	}
	if child.parent == o {
		return nil
	}
	for p := o; p != nil; p = p.parent {
		if p == child {
			return fmt.Errorf("gui: a %s cannot hold itself", child.kind)
		}
	}
	oldForm, newForm := child.form(), o.form()
	if oldForm != newForm {
		if err := fitNames(newForm, child); err != nil {
			return err
		}
	}
	if old := child.parent; old != nil {
		for i, c := range old.children {
			if c == child {
				old.children = append(old.children[:i], old.children[i+1:]...)
				break
			}
		}
	}
	child.parent = o
	o.children = append(o.children, child)
	if oldForm != newForm {
		unregister(oldForm, child)
		register(newForm, child)
	}
	if child.kind == "RadioButton" && propBool(child, "checked") {
		// Settles which one of its new siblings is on.
		if err := child.set(o.app.L, "checked", lua.LTrue); err != nil {
			return err
		}
	}
	if o.widget != nil {
		return buildLive(child)
	}
	return nil
}

// form is the Form an object is on, if it is on one.
func (o *guiObject) form() *guiObject {
	for p := o; p != nil; p = p.parent {
		if p.kind == "Form" {
			return p
		}
	}
	return nil
}

func (o *guiObject) hasEvent(name string) bool {
	for _, e := range commonEvents {
		if e == name {
			return true
		}
	}
	for _, e := range o.spec.events {
		if e == name {
			return true
		}
	}
	for _, e := range o.extra {
		if e == name {
			return true
		}
	}
	return false
}

// eventName turns "click" and "onClick" alike into "onClick".
func eventName(name string) string {
	if len(name) > 2 && strings.HasPrefix(name, "on") && unicode.IsUpper(rune(name[2])) {
		return name
	}
	if name == "" {
		return name
	}
	return "on" + strings.ToUpper(name[:1]) + name[1:]
}

// looksLikeEvent is a property name in the shape of a handler.
func looksLikeEvent(name string) bool {
	return len(name) > 2 && strings.HasPrefix(name, "on") && unicode.IsUpper(rune(name[2]))
}

func (o *guiObject) noSuchEvent(name string) error {
	all := append(append(append([]string{}, o.extra...), o.spec.events...), commonEvents...)
	return fmt.Errorf("gui: a %s has no event %s (it has %s)", o.displayKind(), name, strings.Join(all, ", "))
}

// hasMethod says whether name is one of the methods or factories. Every
// object answers to all of them, so that the ones that do not apply say why
// ("only a Form can be shown", "a Tabs cannot hold a Button") rather than
// being nil, and none of them can be taken for a property.
func (o *guiObject) hasMethod(name string) bool {
	if _, ok := methods[name]; ok {
		return true
	}
	if _, ok := kinds[name]; ok {
		return true
	}
	_, ok := o.app.custom[name]
	return ok
}

// set is a script assigning to a property or a handler.
func (o *guiObject) set(L *lua.LState, name string, value lua.LValue) error {
	if o.hasMethod(name) {
		return fmt.Errorf("gui: %s is a method, not a property", name)
	}
	if looksLikeEvent(name) {
		if !o.hasEvent(name) {
			return o.noSuchEvent(name)
		}
		switch fn := value.(type) {
		case *lua.LNilType:
			delete(o.events, name)
		case *lua.LFunction:
			o.events[name] = fn
		default:
			return fmt.Errorf("gui: %s must be a function or nil, not a %s", name, value.Type())
		}
		return nil
	}
	if alias, ok := o.spec.aliases[name]; ok {
		name = alias
	}
	if o.kind == "Form" && o.names[name] != nil {
		return fmt.Errorf("gui: %s is a control on this form", name)
	}
	if name == "parent" {
		return fmt.Errorf("gui: parent can only be given when a control is made; add() moves it")
	}
	switch name {
	case "name":
		s, ok := value.(lua.LString)
		if !ok && value != lua.LNil {
			return fmt.Errorf("gui: a name is a string, not a %s", value.Type())
		}
		if o.kind != "Form" {
			if err := o.rename(string(s)); err != nil {
				return err
			}
		} else if err := o.checkName(string(s)); err != nil {
			return err
		}
		value = s
	case "file", "image":
		if o.given == nil {
			o.given = map[string]string{}
		}
		o.given[name] = lua.LVAsString(value)
	}
	if o.widget != nil {
		for _, f := range o.spec.fixed {
			if f == name {
				return fmt.Errorf("gui: %s of a %s can only be given when it is made", name, o.kind)
			}
		}
		if name == "grow" {
			return fmt.Errorf("gui: grow can only be given before the form is shown")
		}
	}
	value, err := o.app.checkProp(L, name, value)
	if err != nil {
		return err
	}
	o.props[name] = value
	if name == "left" || name == "top" {
		o.placed = true
	}
	// Only one radio button among those that share a parent is on.
	if o.kind == "RadioButton" && name == "checked" && lua.LVAsBool(value) && o.parent != nil {
		for _, sib := range o.parent.children {
			if sib != o && sib.kind == "RadioButton" {
				sib.props["checked"] = lua.LFalse
				if sib.widget != nil {
					if err := applyProp(sib, "checked", lua.LFalse); err != nil {
						return err
					}
				}
			}
		}
	}
	if o.widget != nil {
		// A Canvas draws from its properties, the script's own included,
		// so a control drawn on one shows a change as soon as it is made.
		if o.kind == "Canvas" {
			redraw(o)
		}
		return applyProp(o, name, value)
	}
	return nil
}

// checkProp catches values the backend could only get wrong, and settles
// what can be settled once: a relative image path is found beside the script
// that gave it.
func (a *app) checkProp(L *lua.LState, name string, value lua.LValue) (lua.LValue, error) {
	switch name {
	case "items", "columns", "rows":
		if _, ok := value.(*lua.LTable); !ok {
			return nil, fmt.Errorf("gui: %s must be a table, not a %s", name, value.Type())
		}
	case "editable":
		switch value.(type) {
		case lua.LBool, *lua.LTable:
		default:
			return nil, fmt.Errorf("gui: editable is true, false or a list of columns, not a %s", value.Type())
		}
	case "columnWidths":
		if _, ok := value.(*lua.LTable); !ok && value != lua.LNil {
			return nil, fmt.Errorf("gui: columnWidths must be a table, not a %s", value.Type())
		}
	case "color", "textColor":
		if value == lua.LNil {
			return value, nil
		}
		if _, _, _, err := parseColor(lua.LVAsString(value)); err != nil {
			return nil, err
		}
	case "font":
		switch lua.LVAsString(value) {
		case "sans", "serif", "mono":
		default:
			return nil, fmt.Errorf("gui: font must be \"sans\", \"serif\" or \"mono\", not %q", lua.LVAsString(value))
		}
	case "syntax":
		switch lua.LVAsString(value) {
		case "", "lua":
		default:
			return nil, fmt.Errorf("gui: syntax is \"lua\" or nothing, not %q", lua.LVAsString(value))
		}
	case "align":
		switch lua.LVAsString(value) {
		case "left", "center", "right":
		default:
			return nil, fmt.Errorf("gui: align must be \"left\", \"center\" or \"right\", not %q", lua.LVAsString(value))
		}
	case "file", "image":
		return lua.LString(a.besideCaller(L, lua.LVAsString(value))), nil
	}
	return value, nil
}

// exists says whether a program can read a file: from its archive when it
// is fused, or from the disk.
func (a *app) exists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	if a.read != nil {
		if _, err := a.read(path); err == nil {
			return true
		}
	}
	return false
}

// besideCaller finds a relative path next to the script that named it, when
// it is not where the current directory would put it.
func (a *app) besideCaller(L *lua.LState, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if a.exists(path) {
		return path
	}
	for level := 1; level < 8; level++ {
		dbg, ok := L.GetStack(level)
		if !ok {
			break
		}
		if _, err := L.GetInfo("S", dbg, lua.LNil); err != nil {
			continue
		}
		src := dbg.Source
		if strings.HasPrefix(src, "@") {
			src = src[1:]
		}
		if src == "" || strings.HasPrefix(src, "=") || strings.HasPrefix(src, "<") {
			continue
		}
		candidate := filepath.Join(filepath.Dir(src), path)
		if a.exists(candidate) {
			return candidate
		}
	}
	return path
}

var colorNames = map[string][3]uint8{
	"black": {0, 0, 0}, "white": {255, 255, 255}, "gray": {128, 128, 128},
	"grey": {128, 128, 128}, "red": {220, 50, 47}, "green": {60, 160, 60},
	"blue": {38, 110, 210}, "yellow": {240, 200, 40}, "orange": {240, 140, 30},
	"purple": {130, 80, 180},
}

// parseColor reads "#rrggbb", "#rgb" or one of a few names.
func parseColor(s string) (r, g, b uint8, err error) {
	if c, ok := colorNames[strings.ToLower(s)]; ok {
		return c[0], c[1], c[2], nil
	}
	var v uint32
	switch {
	case len(s) == 7 && s[0] == '#':
		_, err = fmt.Sscanf(s[1:], "%06x", &v)
	case len(s) == 4 && s[0] == '#':
		_, err = fmt.Sscanf(s[1:], "%03x", &v)
		v = (v&0xf00)<<12 | (v&0xf00)<<8 | (v&0x0f0)<<8 | (v&0x0f0)<<4 | (v&0x00f)<<4 | v&0x00f
	default:
		err = fmt.Errorf("bad")
	}
	if err != nil {
		names := make([]string, 0, len(colorNames))
		for n := range colorNames {
			names = append(names, n)
		}
		sort.Strings(names)
		return 0, 0, 0, fmt.Errorf("gui: colour must be \"#rrggbb\" or one of %s, not %q", strings.Join(names, ", "), s)
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v), nil
}

func guiIndex(L *lua.LState) int {
	obj := checkObject(L, 1)
	key := L.CheckString(2)
	if obj.hasMethod(key) {
		L.Push(obj.app.method(L, key))
		return 1
	}
	if looksLikeEvent(key) {
		if fn, ok := obj.events[key]; ok {
			L.Push(fn)
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}
	if alias, ok := obj.spec.aliases[key]; ok {
		key = alias
	}
	if c, ok := obj.names[key]; ok {
		L.Push(c.ud)
		return 1
	}
	if key == "parent" {
		// What holds it: read-only, since add() is how it moves.
		if obj.parent != nil {
			L.Push(obj.parent.ud)
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}
	L.Push(obj.get(key))
	return 1
}

// get reads a property, from the widget while there is one: the user may
// have typed, ticked, picked or resized since the script last looked.
func (o *guiObject) get(name string) lua.LValue {
	if o.widget != nil {
		if v, ok := readProp(o, name); ok {
			return v
		}
	}
	if v, ok := o.props[name]; ok {
		return v
	}
	return lua.LNil
}

func guiNewIndex(L *lua.LState) int {
	obj := checkObject(L, 1)
	if err := obj.set(L, L.CheckString(2), L.Get(3)); err != nil {
		L.RaiseError("%s", err.Error())
	}
	return 0
}

func guiToString(L *lua.LState) int {
	obj := checkObject(L, 1)
	if c := propString(obj, "caption"); c != "" {
		L.Push(lua.LString(fmt.Sprintf("%s %q", obj.displayKind(), c)))
	} else {
		L.Push(lua.LString(obj.displayKind()))
	}
	return 1
}

// method hands out one function per name and state, so obj.show == obj.show.
func (a *app) method(L *lua.LState, name string) *lua.LFunction {
	if fn, ok := a.methods[name]; ok {
		return fn
	}
	var fn *lua.LFunction
	if m, ok := methods[name]; ok {
		fn = L.NewFunction(m)
	} else if _, ok := a.custom[name]; ok {
		// A defined control: frame:Rating{...}.
		kind := name
		fn = L.NewFunction(func(L *lua.LState) int {
			parent := checkObject(L, 1)
			L.Push(a.makeCustom(L, kind, parent, L.OptTable(2, L.NewTable())).ud)
			return 1
		})
	} else {
		// A factory: frame:Button{...} and the like.
		kind := name
		fn = L.NewFunction(func(L *lua.LState) int {
			parent := checkObject(L, 1)
			if !parent.holds(kind) {
				L.RaiseError("gui: a %s cannot hold a %s", parent.kind, kind)
			}
			L.Push(a.make(L, kind, kinds[kind], L.OptTable(2, L.NewTable()), parent).ud)
			return 1
		})
	}
	a.methods[name] = fn
	return fn
}

func checkForm(L *lua.LState) *guiObject {
	obj := checkObject(L, 1)
	if obj.kind != "Form" {
		L.RaiseError("gui: only a Form can be shown, not a %s", obj.kind)
	}
	return obj
}

// show puts a form on screen. Under bootgui() that is all; otherwise it also
// runs the event loop until the form is closed, so a plain script, -e or the
// REPL can show a window and carry on afterwards.
func guiShow(L *lua.LState) int {
	f := checkForm(L)
	a := f.app
	if err := a.build(f); err != nil {
		L.RaiseError("%s", err.Error())
	}
	showWindow(f, false)
	if !a.booted {
		a.loop(func() bool { return !shown(f) })
		a.raisePending(L)
	}
	return 0
}

// showModal puts a form up in front of the others and waits for it to be
// closed, under bootgui() or not: the way to ask a question with a form.
func guiShowModal(L *lua.LState) int {
	f := checkForm(L)
	a := f.app
	if err := a.build(f); err != nil {
		L.RaiseError("%s", err.Error())
	}
	showWindow(f, true)
	a.loop(func() bool { return !shown(f) })
	a.raisePending(L)
	return 0
}

func guiClose(L *lua.LState) int {
	f := checkForm(L)
	f.app.requestClose(f)
	f.app.raisePending(L)
	return 0
}

func guiAdd(L *lua.LState) int {
	parent := checkObject(L, 1)
	if err := parent.adopt(checkObject(L, 2)); err != nil {
		L.RaiseError("%s", err.Error())
	}
	L.Push(L.Get(2))
	return 1
}

func guiOn(L *lua.LState) int {
	obj := checkObject(L, 1)
	name := eventName(L.CheckString(2))
	if err := obj.set(L, name, L.Get(3)); err != nil {
		L.RaiseError("%s", err.Error())
	}
	L.Push(L.Get(1))
	return 1
}

func guiFocus(L *lua.LState) int {
	focus(checkObject(L, 1))
	return 0
}

// remove takes a control off whatever holds it, and frees its widgets. It
// can be added somewhere again afterwards, and is built anew there.
func guiRemove(L *lua.LState) int {
	obj := checkObject(L, 1)
	if obj.kind == "Form" {
		L.RaiseError("gui: a Form is closed, not removed")
	}
	obj.detach()
	return 0
}

func (o *guiObject) detach() {
	p := o.parent
	if p == nil {
		return
	}
	unregister(o.form(), o)
	for i, c := range p.children {
		if c == o {
			p.children = append(p.children[:i], p.children[i+1:]...)
			break
		}
	}
	if o.widget != nil {
		settle(o)
		destroyWidget(o) // while it still knows its form, to redraw it
	}
	o.parent = nil
}

// raise puts a control in front of the others in its container; lower puts
// it behind them. The order is also the order a layout lists them in.
func guiRaise(L *lua.LState) int { return restackTo(L, true) }
func guiLower(L *lua.LState) int { return restackTo(L, false) }

func restackTo(L *lua.LState, front bool) int {
	obj := checkObject(L, 1)
	p := obj.parent
	if p == nil {
		return 0
	}
	rest := make([]*guiObject, 0, len(p.children))
	for _, c := range p.children {
		if c != obj {
			rest = append(rest, c)
		}
	}
	if front {
		p.children = append(rest, obj)
	} else {
		p.children = append([]*guiObject{obj}, rest...)
	}
	if p.widget != nil {
		restack(p)
	}
	return 0
}

// select is textBox:select(i, j): selects its text from byte i to byte j,
// as string.sub counts them, and puts the cursor after it.
func guiSelect(L *lua.LState) int {
	obj := checkObject(L, 1)
	if obj.kind != "TextBox" {
		L.RaiseError("gui: only a TextBox has text to select, not a %s", obj.displayKind())
	}
	i, j := L.CheckInt(2), L.OptInt(3, L.CheckInt(2)-1)
	selectText(obj, i, j)
	return 0
}

// edit starts editing a Table's cell, as a click on it would; with no
// arguments it finishes the edit there is, keeping what was typed.
func guiEdit(L *lua.LState) int {
	obj := checkObject(L, 1)
	if obj.kind != "Table" {
		L.RaiseError("gui: only a Table has cells to edit, not a %s", obj.displayKind())
	}
	if obj.widget != nil {
		tableEdit(obj, L.OptInt(2, 0), L.OptInt(3, 0))
	}
	return 0
}

// editing reports the row and column of the cell being edited, or nothing.
func guiEditing(L *lua.LState) int {
	obj := checkObject(L, 1)
	if obj.kind != "Table" || obj.widget == nil {
		return 0
	}
	row, col := tableEditing(obj)
	if row == 0 {
		return 0
	}
	L.Push(lua.LNumber(row))
	L.Push(lua.LNumber(col))
	return 2
}

// insert puts text in a TextBox in place of what is selected, or at the
// cursor, leaves the cursor after it, and raises onChange.
func guiInsert(L *lua.LState) int {
	obj := checkObject(L, 1)
	if obj.kind != "TextBox" {
		L.RaiseError("gui: only a TextBox has text to insert into, not a %s", obj.displayKind())
	}
	if obj.widget != nil {
		insertText(obj, L.CheckString(2))
		obj.app.fire(obj, "onChange")
	}
	return 0
}

// pointAt is where a TextBox shows the text position pos, counted as cursor
// counts: x and y of its top left, from the box's own top left, and the
// height of its line. Nothing when the box is not on screen.
func guiPointAt(L *lua.LState) int {
	obj := checkObject(L, 1)
	if obj.kind != "TextBox" {
		L.RaiseError("gui: only a TextBox has text positions, not a %s", obj.displayKind())
	}
	if obj.widget == nil {
		return 0
	}
	x, y, h, ok := pointAt(obj, L.CheckInt(2))
	if !ok {
		return 0
	}
	L.Push(lua.LNumber(x))
	L.Push(lua.LNumber(y))
	L.Push(lua.LNumber(h))
	return 3
}

// redraw asks for an object to be drawn again: a Canvas whose picture has
// changed, mostly.
func guiRedraw(L *lua.LState) int {
	redraw(checkObject(L, 1))
	return 0
}

// requestClose is the close box, Escape and form:close() alike: either
// handler can keep the form open by returning false.
func (a *app) requestClose(f *guiObject) {
	if a.fire(f, "onClose") == lua.LFalse || a.err != nil {
		return
	}
	if a.fire(f, "onUnload") == lua.LFalse || a.err != nil {
		return
	}
	hideWindow(f)
	a.release(f)
}

// build puts a form's window together, if it has none, and keeps track of it.
func (a *app) build(f *guiObject) error {
	if err := buildForm(f); err != nil {
		return err
	}
	for _, g := range a.forms {
		if g == f {
			return nil
		}
	}
	a.forms = append(a.forms, f)
	return nil
}

// release frees a closed form's window and everything in it, once the event
// loop is past whatever closed it. What the user left in the controls is
// copied back into their properties first, so the script can still read it,
// and showing the form again builds it afresh from them.
func (a *app) release(f *guiObject) {
	settle(f)
	f.props["visible"] = lua.LFalse
	releaseForm(f, func() {
		for i, g := range a.forms {
			if g == f {
				a.forms = append(a.forms[:i], a.forms[i+1:]...)
				break
			}
		}
	})
}

// settle copies what is on screen into an object's properties, and its
// children's.
func settle(o *guiObject) {
	if o.widget != nil {
		for name := range o.props {
			if name == "visible" && o.kind == "Form" {
				continue
			}
			if v, ok := readProp(o, name); ok {
				o.props[name] = v
			}
		}
	}
	for _, c := range o.children {
		settle(c)
	}
}

// fire calls one of an object's handlers with the object itself as self. An
// error is kept for whoever runs the loop, and nothing runs after one.
func (a *app) fire(o *guiObject, event string, args ...lua.LValue) lua.LValue {
	fn := o.events[event]
	if fn == nil {
		return lua.LNil
	}
	return a.call(fn, append([]lua.LValue{o.ud}, args...)...)
}

func (a *app) call(fn *lua.LFunction, args ...lua.LValue) lua.LValue {
	if a.err != nil {
		return lua.LNil
	}
	L := a.L
	if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, args...); err != nil {
		a.err = err
		return lua.LNil
	}
	ret := L.Get(-1)
	L.Pop(1)
	return ret
}

// loop runs the event loop until done says so or a handler fails.
func (a *app) loop(done func() bool) {
	for a.err == nil && !done() {
		wait()
	}
}

func (a *app) anyShown() bool {
	for _, f := range a.forms {
		if f.widget != nil && shown(f) {
			return true
		}
	}
	return false
}

// takeErr hands over a handler's error, once, taking every form down with
// it: the program is about to stop, or go back to the prompt.
func (a *app) takeErr() error {
	err := a.err
	if err == nil {
		return nil
	}
	a.err = nil
	for _, f := range a.forms {
		if f.widget != nil {
			hideWindow(f)
		}
	}
	return err
}

// raisePending raises a handler's error in the Lua code that started the
// loop, as if the handler had been called from there.
func (a *app) raisePending(L *lua.LState) {
	err := a.takeErr()
	if err == nil {
		return
	}
	if apiErr, ok := err.(*lua.ApiError); ok {
		L.Error(apiErr.Object, 0)
	}
	L.RaiseError("%s", err.Error())
}

func propInt(obj *guiObject, name string, def int) int {
	if v, ok := obj.props[name]; ok {
		if n, ok := v.(lua.LNumber); ok {
			return int(n)
		}
	}
	return def
}

func propFloat(obj *guiObject, name string) float64 {
	return float64(lua.LVAsNumber(obj.props[name]))
}

func propBool(obj *guiObject, name string) bool {
	v, ok := obj.props[name]
	return ok && lua.LVAsBool(v)
}

func propString(obj *guiObject, name string) string {
	if v, ok := obj.props[name]; ok && v != lua.LNil {
		return lua.LVAsString(v)
	}
	return ""
}

// items reads a ComboBox, ListBox or Menu's list as strings.
func propItems(obj *guiObject) []string {
	t, ok := obj.props["items"].(*lua.LTable)
	if !ok {
		return nil
	}
	out := make([]string, 0, t.Len())
	for i := 1; i <= t.Len(); i++ {
		out = append(out, lua.LVAsString(t.RawGetInt(i)))
	}
	return out
}

// applyStoredProps hands a fresh widget what the script said before it was
// there. Visibility goes last, and a Form's is left to show().
func (o *guiObject) applyStoredProps() error {
	names := make([]string, 0, len(o.props))
	for k := range o.props {
		if k != "visible" {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	if o.kind != "Form" {
		names = append(names, "visible")
	}
	for _, k := range names {
		if err := applyProp(o, k, o.props[k]); err != nil {
			return err
		}
	}
	return nil
}
