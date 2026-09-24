package editor

import (
	"regexp"
	"strings"

	"github.com/yuin/gopher-lua/ast"
	"github.com/yuin/gopher-lua/parse"
)

// entryKind is how a function is reachable, which is what the list colours by.
type entryKind uint8

const (
	kindGlobal entryKind = iota // function f, f = function, M.f, M:f
	kindLocal                   // local function f, local f = function
	kindNested                  // defined inside another function, at any depth
)

func (k entryKind) String() string {
	switch k {
	case kindLocal:
		return "local"
	case kindNested:
		return "nested"
	default:
		return "global"
	}
}

// outlineEntry is one function found in a buffer: what QBasic listed on F2 and
// RHIDE on Alt-F2.
type outlineEntry struct {
	Name  string
	Kind  entryKind
	Line  int // 1-based, where the definition starts
	Depth int // how many functions it sits inside
}

// outline lists the functions defined in a Lua source file, in source order.
// It reads the parse tree, which knows the difference between a local and a
// global and gets nested definitions right; while the buffer does not parse,
// which is most of the time during typing, it falls back to scanning lines.
func outline(src string) []outlineEntry {
	chunk, err := parse.Parse(strings.NewReader(src), "outline")
	if err != nil {
		return outlineByScan(src)
	}
	var entries []outlineEntry
	walkStmts(chunk, &entries, 0)
	return entries
}

// kindAt decides how to label a definition: anything inside another function is
// nested, whatever its name would otherwise make it.
func kindAt(local bool, depth int) entryKind {
	switch {
	case depth > 0:
		return kindNested
	case local:
		return kindLocal
	default:
		return kindGlobal
	}
}

func walkStmts(stmts []ast.Stmt, out *[]outlineEntry, depth int) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.FuncDefStmt:
			name := funcDefName(s.Name)
			*out = append(*out, outlineEntry{
				Name: name, Kind: kindAt(false, depth), Line: s.Line(), Depth: depth,
			})
			if s.Func != nil {
				walkStmts(s.Func.Stmts, out, depth+1)
			}

		case *ast.LocalAssignStmt:
			for i, expr := range s.Exprs {
				if fn, ok := expr.(*ast.FunctionExpr); ok && i < len(s.Names) {
					*out = append(*out, outlineEntry{
						Name: s.Names[i], Kind: kindAt(true, depth), Line: s.Line(), Depth: depth,
					})
					walkStmts(fn.Stmts, out, depth+1)
					continue
				}
				walkExpr(expr, "", out, depth)
			}

		case *ast.AssignStmt:
			for i, expr := range s.Rhs {
				name := ""
				if i < len(s.Lhs) {
					name = exprName(s.Lhs[i])
				}
				if fn, ok := expr.(*ast.FunctionExpr); ok {
					if name == "" {
						name = "(anonymous)"
					}
					*out = append(*out, outlineEntry{
						Name: name, Kind: kindAt(false, depth), Line: s.Line(), Depth: depth,
					})
					walkStmts(fn.Stmts, out, depth+1)
					continue
				}
				walkExpr(expr, name, out, depth)
			}

		case *ast.DoBlockStmt:
			walkStmts(s.Stmts, out, depth)
		case *ast.WhileStmt:
			walkStmts(s.Stmts, out, depth)
		case *ast.RepeatStmt:
			walkStmts(s.Stmts, out, depth)
		case *ast.IfStmt:
			walkStmts(s.Then, out, depth)
			walkStmts(s.Else, out, depth)
		case *ast.NumberForStmt:
			walkStmts(s.Stmts, out, depth)
		case *ast.GenericForStmt:
			walkStmts(s.Stmts, out, depth)
		case *ast.FuncCallStmt:
			walkExpr(s.Expr, "", out, depth)
		case *ast.ReturnStmt:
			for _, expr := range s.Exprs {
				walkExpr(expr, "", out, depth)
			}
		}
	}
}

// walkExpr finds functions hiding inside expressions, above all the table of
// functions that a Lua module usually is.
func walkExpr(expr ast.Expr, prefix string, out *[]outlineEntry, depth int) {
	switch e := expr.(type) {
	case *ast.FunctionExpr:
		walkStmts(e.Stmts, out, depth+1)
	case *ast.TableExpr:
		for _, field := range e.Fields {
			name := exprName(field.Key)
			if prefix != "" && name != "" {
				name = prefix + "." + name
			}
			if fn, ok := field.Value.(*ast.FunctionExpr); ok {
				if name == "" {
					name = "(anonymous)"
				}
				*out = append(*out, outlineEntry{
					Name: name, Kind: kindAt(false, depth), Line: fn.Line(), Depth: depth,
				})
				walkStmts(fn.Stmts, out, depth+1)
				continue
			}
			walkExpr(field.Value, name, out, depth)
		}
	case *ast.FuncCallExpr:
		for _, arg := range e.Args {
			walkExpr(arg, "", out, depth)
		}
	}
}

// funcDefName spells out the name in a "function a.b:c()" header.
func funcDefName(name *ast.FuncName) string {
	if name == nil {
		return "(anonymous)"
	}
	if name.Method != "" {
		return exprName(name.Receiver) + ":" + name.Method
	}
	return exprName(name.Func)
}

// exprName spells out the dotted name an expression refers to, if it is one.
func exprName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		return e.Value
	case *ast.StringExpr:
		return e.Value
	case *ast.AttrGetExpr:
		object, key := exprName(e.Object), exprName(e.Key)
		if object == "" || key == "" {
			return ""
		}
		return object + "." + key
	}
	return ""
}

// The patterns the fallback scanner recognises, in the order it tries them.
var scanPatterns = []struct {
	re    *regexp.Regexp
	local bool
}{
	{regexp.MustCompile(`^(\s*)local\s+function\s+([\w]+)`), true},
	{regexp.MustCompile(`^(\s*)function\s+([\w.]*[\w]+:[\w]+)`), false},
	{regexp.MustCompile(`^(\s*)function\s+([\w.]+)`), false},
	{regexp.MustCompile(`^(\s*)local\s+([\w]+)\s*=\s*function`), true},
	{regexp.MustCompile(`^(\s*)([\w.]+)\s*=\s*function`), false},
	{regexp.MustCompile(`^(\s*)\[?["']?([\w]+)["']?\]?\s*=\s*function`), false},
}

// outlineByScan reads definitions line by line, which still works while the
// buffer is half-written and the parser refuses it.
func outlineByScan(src string) []outlineEntry {
	var entries []outlineEntry
	for i, line := range strings.Split(src, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		for _, pattern := range scanPatterns {
			m := pattern.re.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			// Without a parse tree, indentation is the only clue that a
			// definition sits inside something else.
			depth := 0
			if indent := len(strings.ReplaceAll(m[1], "\t", "  ")); indent > 0 {
				depth = 1
			}
			entries = append(entries, outlineEntry{
				Name: m[2], Kind: kindAt(pattern.local, depth), Line: i + 1, Depth: depth,
			})
			break
		}
	}
	return entries
}
