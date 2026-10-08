package gui

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func treeItems(t *testing.T, L *lua.LState) *lua.LTable {
	t.Helper()
	run(t, L, `items = {
  "README",
  {"src", {"main.lua", {"lib", {"util.lua"}}}, open = true},
  {"docs", {"guide.md"}},
  {"empty"},
}`)
	return L.GetGlobal("items").(*lua.LTable)
}

func paths(rows []treeRow) string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.path
	}
	return strings.Join(out, " ")
}

func TestFlattenTreeShowsOpenBranches(t *testing.T) {
	L, _ := newState(t)
	rows := flattenTree(treeItems(t, L), 0, "", nil)
	if got := paths(rows); got != "README src src/main.lua src/lib docs empty" {
		t.Fatalf("rows: %s", got)
	}
	if !rows[1].branch || !rows[1].open || rows[3].open || !rows[3].branch || rows[5].branch {
		t.Fatalf("branch flags: %+v", rows)
	}
	if rows[3].depth != 1 {
		t.Fatalf("depth of src/lib = %d", rows[3].depth)
	}
	if got := treeLine(rows[3]); got != "     ▸ lib" {
		t.Fatalf("line %q", got)
	}
	if got := treeLine(rows[1]); got != "▾ src" {
		t.Fatalf("line %q", got)
	}
}

func TestOpenToOpensTheWay(t *testing.T) {
	L, _ := newState(t)
	items := treeItems(t, L)
	if !openTo(items, "src/lib/util.lua") {
		t.Fatal("src/lib/util.lua should be found")
	}
	if got := paths(flattenTree(items, 0, "", nil)); !strings.Contains(got, "src/lib/util.lua") {
		t.Fatalf("not opened: %s", got)
	}
	run(t, L, `assert(items[2][2][2].open == true, "open is written to the script's table")`)
	for _, missing := range []string{"nope", "src/nope", "README/x", ""} {
		if openTo(items, missing) {
			t.Errorf("openTo(%q) found something", missing)
		}
	}
}

func TestNewKinds(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
form = gui.Form{}
tree = form:Tree{items = {"a", {"b", {"c"}}}, onToggle = function() end}
table = form:Table{columns = {"Name", "Age"}, rows = {{"Ada", 36}}, columnWidths = {120, 60}}
canvas = form:Canvas{onDraw = function(self, g) end, onMouseDown = function() end}
assert(canvas.color == "#ffffff" and tree.width == 200 and table.selected == 0)
canvas:redraw()
form.onDrop = function(self, text, lines) end
assert(gui.clipboard ~= nil)`)
	fails(t, L, `canvas.onClick = print`, "a Canvas has no event onClick")
	fails(t, L, `table.onToggle = print`, "a Table has no event onToggle")
}
