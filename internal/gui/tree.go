package gui

import (
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// A Tree's items are the script's own tables: a string is a leaf, and
// {"Label", {children...}, open = true} is a branch, which is open when its
// open field says so. The Tree shows them as the lines of a list, and
// opening or closing a branch writes open back into the script's table.

// treeRow is one line of a Tree as it is shown: a node whose branches above
// it are all open.
type treeRow struct {
	depth  int
	label  string
	path   string
	node   *lua.LTable // the node's own table, or nil for a plain string
	branch bool
	open   bool
}

// flattenTree lists the nodes that are showing, in order.
func flattenTree(items *lua.LTable, depth int, prefix string, out []treeRow) []treeRow {
	if items == nil {
		return out
	}
	for i := 1; i <= items.Len(); i++ {
		switch v := items.RawGetInt(i).(type) {
		case *lua.LTable:
			label := lua.LVAsString(v.RawGetInt(1))
			kids, _ := v.RawGetInt(2).(*lua.LTable)
			row := treeRow{
				depth: depth, label: label, path: prefix + label, node: v,
				branch: kids != nil, open: kids != nil && lua.LVAsBool(v.RawGetString("open")),
			}
			out = append(out, row)
			if row.open {
				out = flattenTree(kids, depth+1, row.path+"/", out)
			}
		default:
			label := lua.LVAsString(v)
			out = append(out, treeRow{depth: depth, label: label, path: prefix + label})
		}
	}
	return out
}

// openTo opens every branch on the way to path, so that it shows, and says
// whether there is such a node at all.
func openTo(items *lua.LTable, path string) bool {
	if path == "" {
		return false
	}
	labels := strings.Split(path, "/")
	for depth, label := range labels {
		var next *lua.LTable
		found := false
		for i := 1; items != nil && i <= items.Len(); i++ {
			switch v := items.RawGetInt(i).(type) {
			case *lua.LTable:
				if lua.LVAsString(v.RawGetInt(1)) != label {
					continue
				}
				found = true
				if depth < len(labels)-1 {
					next, _ = v.RawGetInt(2).(*lua.LTable)
					if next != nil {
						v.RawSetString("open", lua.LTrue)
					}
				}
			default:
				if lua.LVAsString(v) == label {
					found = true
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
		if depth < len(labels)-1 {
			if next == nil {
				return false
			}
			items = next
		}
	}
	return true
}

// treeLine is how a row reads in the list.
func treeLine(r treeRow) string {
	marker := "    "
	if r.branch {
		if r.open {
			marker = "▾ "
		} else {
			marker = "▸ "
		}
	}
	return strings.Repeat("     ", r.depth) + marker + r.label
}
