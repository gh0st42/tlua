package design

// design.server is the language server the code window asks for
// completions, hover help and what a call takes. Every request runs in the
// background and is asked later whether it is done, so that a server still
// reading the workspace never holds up the window.
//
//	local server = require "design.server"
//	local s = server.start(dir)              -- nil, why when there is none
//	local req = s:complete(path, text, cursor)
//	gui.every(0.05, function()
//	  local done, items = req:result()
//	  ...
//	end)
//
// cursor counts bytes before the cursor, as a TextBox's does.

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/lsp"
	"tlua/library"
)

const (
	// serverStartTimeout is how long a server has to answer the handshake.
	serverStartTimeout = 30 * time.Second
	// requestTimeout is how long one question waits, the start included.
	requestTimeout = 20 * time.Second
)

// server is a language server starting or started.
type server struct {
	ready  chan struct{}
	client *lsp.Client
	err    error
	name   string
}

// servers are the ones started, closed when the designer ends.
var (
	serversMu sync.Mutex
	servers   []*server
)

// closeServers shuts down every server the designer started.
func closeServers() {
	serversMu.Lock()
	list := servers
	servers = nil
	serversMu.Unlock()
	for _, s := range list {
		s.close()
	}
}

func (s *server) close() {
	<-s.ready
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}
}

// request is one question to a server, answered in the background.
type request struct {
	mu    sync.Mutex
	done  bool
	value func(L *lua.LState) lua.LValue
	err   error
}

func (r *request) finish(value func(L *lua.LState) lua.LValue, err error) {
	r.mu.Lock()
	r.done, r.value, r.err = true, value, err
	r.mu.Unlock()
}

const serverType = "design.server"
const requestType = "design.server.request"

func preloadServer(L *lua.LState) {
	smt := L.NewTypeMetatable(serverType)
	L.SetField(smt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"state":     serverState,
		"triggers":  serverTriggers,
		"complete":  serverAsk("complete"),
		"hover":     serverAsk("hover"),
		"signature": serverAsk("signature"),
		"close":     serverClose,
	}))
	rmt := L.NewTypeMetatable(requestType)
	L.SetField(rmt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"result": requestResult,
	}))

	L.PreloadModule("design.server", func(L *lua.LState) int {
		mod := L.NewTable()
		L.SetField(mod, "start", L.NewFunction(serverStart))
		L.Push(mod)
		return 1
	})
}

// serverStart is server.start(root): the server for a project, starting in
// the background, or nil and why there is none.
func serverStart(L *lua.LState) int {
	root := L.CheckString(1)
	command, args, ok := lsp.Find()
	if !ok {
		L.Push(lua.LNil)
		L.Push(lua.LString("no language server: put lua-language-server on PATH, or set " + lsp.EnvServer))
		return 2
	}
	s := &server{ready: make(chan struct{}), name: command}
	go func() {
		defer close(s.ready)
		ctx, cancel := context.WithTimeout(context.Background(), serverStartTimeout)
		defer cancel()
		s.client, s.err = lsp.StartWith(ctx, command, args, root, library.Settings())
		if s.client != nil {
			s.name = s.client.Name()
		}
	}()
	serversMu.Lock()
	servers = append(servers, s)
	serversMu.Unlock()
	ud := L.NewUserData()
	ud.Value = s
	L.SetMetatable(ud, L.GetTypeMetatable(serverType))
	L.Push(ud)
	return 1
}

func checkServer(L *lua.LState) *server {
	ud := L.CheckUserData(1)
	s, ok := ud.Value.(*server)
	if !ok {
		L.ArgError(1, "a language server expected")
	}
	return s
}

// serverState is s:state(): "starting", "ready" with the server's name, or
// "failed" with why.
func serverState(L *lua.LState) int {
	s := checkServer(L)
	select {
	case <-s.ready:
	default:
		L.Push(lua.LString("starting"))
		L.Push(lua.LString(s.name))
		return 2
	}
	if s.err != nil || s.client == nil {
		L.Push(lua.LString("failed"))
		if s.err != nil {
			L.Push(lua.LString(s.err.Error()))
		} else {
			L.Push(lua.LString("closed"))
		}
		return 2
	}
	L.Push(lua.LString("ready"))
	L.Push(lua.LString(s.name))
	return 2
}

// serverTriggers is s:triggers(): the characters after which a completion
// is asked for without being asked, and those after which what a call takes
// is; empty while the server starts.
func serverTriggers(L *lua.LState) int {
	s := checkServer(L)
	complete, signature := L.NewTable(), L.NewTable()
	select {
	case <-s.ready:
		if s.client != nil {
			for _, c := range s.client.TriggerCharacters() {
				complete.Append(lua.LString(c))
			}
			for _, c := range s.client.SignatureTriggerCharacters() {
				signature.Append(lua.LString(c))
			}
		}
	default:
	}
	L.Push(complete)
	L.Push(signature)
	return 2
}

func serverClose(L *lua.LState) int {
	s := checkServer(L)
	go s.close()
	return 0
}

// serverAsk makes s:complete, s:hover and s:signature, each (path, text,
// cursor) and each returning a request.
func serverAsk(what string) lua.LGFunction {
	return func(L *lua.LState) int {
		s := checkServer(L)
		path, text, cursor := L.CheckString(2), L.CheckString(3), L.CheckInt(4)
		if cursor < 0 {
			cursor = 0
		}
		if cursor > len(text) {
			cursor = len(text)
		}
		req := &request{}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			defer cancel()
			select {
			case <-s.ready:
			case <-ctx.Done():
				req.finish(nil, ctx.Err())
				return
			}
			if s.client == nil {
				req.finish(nil, s.err)
				return
			}
			value, err := ask(ctx, s.client, what, path, text, cursor)
			req.finish(value, err)
		}()
		ud := L.NewUserData()
		ud.Value = req
		L.SetMetatable(ud, L.GetTypeMetatable(requestType))
		L.Push(ud)
		return 1
	}
}

// requestResult is req:result(): whether it is done, and then the answer,
// or nil and why.
func requestResult(L *lua.LState) int {
	ud := L.CheckUserData(1)
	req, ok := ud.Value.(*request)
	if !ok {
		L.ArgError(1, "a request expected")
	}
	req.mu.Lock()
	done, value, err := req.done, req.value, req.err
	req.mu.Unlock()
	if !done {
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LTrue)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 3
	}
	if value == nil {
		L.Push(lua.LNil)
		return 2
	}
	L.Push(value(L))
	return 2
}

// ask puts one question to the server, and returns how to give its answer
// to Lua: built on the Lua side, when it is asked for.
func ask(ctx context.Context, c *lsp.Client, what, path, text string, cursor int) (func(L *lua.LState) lua.LValue, error) {
	pos := positionOf(text, cursor)
	switch what {
	case "complete":
		if !c.CanComplete() {
			return nil, nil
		}
		items, err := c.Complete(ctx, path, text, pos)
		if err != nil {
			return nil, err
		}
		list := completions(text, cursor, items)
		return func(L *lua.LState) lua.LValue {
			t := L.NewTable()
			for _, it := range list {
				e := L.NewTable()
				e.RawSetString("label", lua.LString(it.label))
				e.RawSetString("text", lua.LString(it.text))
				e.RawSetString("kind", lua.LString(it.kind))
				e.RawSetString("help", lua.LString(it.help))
				e.RawSetString("filter", lua.LString(it.filter))
				e.RawSetString("from", lua.LNumber(it.from))
				e.RawSetString("to", lua.LNumber(it.to))
				t.Append(e)
			}
			return t
		}, nil
	case "hover":
		if !c.CanHover() {
			return nil, nil
		}
		help, err := c.Hover(ctx, path, text, pos)
		if err != nil || strings.TrimSpace(help) == "" {
			return nil, err
		}
		return func(L *lua.LState) lua.LValue { return lua.LString(strings.TrimSpace(help)) }, nil
	case "signature":
		if !c.CanSignature() {
			return nil, nil
		}
		help, err := c.Signature(ctx, path, text, pos)
		if err != nil || help == nil {
			return nil, err
		}
		sig, param, ok := help.Active()
		if !ok {
			return nil, nil
		}
		from, to := 0, 0
		if param >= 0 && param < len(sig.Parameters) {
			if a, b, ok := sig.Parameters[param].Span(sig.Label); ok {
				from, to = a+1, b
			}
		}
		return func(L *lua.LState) lua.LValue {
			t := L.NewTable()
			t.RawSetString("label", lua.LString(sig.Label))
			t.RawSetString("from", lua.LNumber(from))
			t.RawSetString("to", lua.LNumber(to))
			return t
		}, nil
	}
	return nil, nil
}

// completion is one item, ready for the Lua side: what it reads as and
// puts in, and the bytes of the text it replaces, as string.sub counts.
type completion struct {
	label, text, kind, help, filter string
	sort                            string
	from, to                        int
}

func completions(text string, cursor int, items []lsp.CompletionItem) []completion {
	// Without an edit of its own, an item replaces the word being typed.
	word := cursor
	for word > 0 && isWordByte(text[word-1]) {
		word--
	}
	out := make([]completion, 0, len(items))
	for _, it := range items {
		c := completion{
			label: it.Label, text: it.Text(), kind: it.Kind.String(), help: it.Help(),
			filter: it.FilterText, sort: it.SortText, from: word + 1, to: cursor,
		}
		if c.filter == "" {
			c.filter = it.Label
		}
		if it.TextEdit != nil {
			start, err1 := lsp.Offset(text, it.TextEdit.Range.Start)
			end, err2 := lsp.Offset(text, it.TextEdit.Range.End)
			if err1 == nil && err2 == nil && start <= end {
				c.text, c.from, c.to = it.EditText(), start+1, end
			}
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].sort, out[j].sort
		if a == "" {
			a = out[i].label
		}
		if b == "" {
			b = out[j].label
		}
		return a < b
	})
	return out
}

func isWordByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b >= 0x80
}

// positionOf turns bytes before the cursor into the line and UTF-16 column
// a language server counts in.
func positionOf(text string, cursor int) lsp.Position {
	line, start := 0, 0
	for i := 0; i < cursor; i++ {
		if text[i] == '\n' {
			line++
			start = i + 1
		}
	}
	units := 0
	for _, r := range text[start:cursor] {
		if r >= 0x10000 {
			units += 2
		} else {
			units++
		}
	}
	return lsp.Position{Line: line, Character: units}
}
