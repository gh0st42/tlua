package interp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
)

const (
	prompt     = "> "
	promptCont = ">> "
)

// REPL reads a line at a time, keeps collecting while the chunk is
// syntactically unfinished, and prints whatever the chunk returns.
func (r *Interp) REPL() {
	in := bufio.NewReader(os.Stdin)

	for {
		line, err := readLine(in, prompt)
		if err == io.EOF {
			fmt.Println()
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "tlua: %v\n", err)
			return
		}
		if strings.TrimSpace(line) == "" {
			continue
		}

		src := line
		fn, lerr := r.compileREPL(src)
		for isIncomplete(lerr) {
			cont, err := readLine(in, promptCont)
			if err != nil {
				break
			}
			src += "\n" + cont
			fn, lerr = r.compileREPL(src)
		}

		if lerr != nil {
			r.Report(lerr)
			continue
		}
		if err := r.callREPL(fn); err != nil {
			r.Report(err)
		}
	}
}

// compileREPL compiles a line, first as an expression so that bare values
// ("1+1", "os.time()") print their result, then as a statement.
func (r *Interp) compileREPL(src string) (*lua.LFunction, error) {
	if fn, err := r.L.Load(strings.NewReader("return "+src), "=stdin"); err == nil {
		return fn, nil
	}
	return r.L.Load(strings.NewReader(src), "=stdin")
}

func (r *Interp) callREPL(fn *lua.LFunction) error {
	base := r.L.GetTop()
	err := r.protect(func() error {
		r.L.Push(fn)
		return r.L.PCall(0, lua.MultRet, nil)
	})
	if err != nil {
		r.L.SetTop(base)
		return err
	}

	n := r.L.GetTop() - base
	if n > 0 {
		parts := make([]string, 0, n)
		for i := 1; i <= n; i++ {
			parts = append(parts, tostring(r.L, r.L.Get(base+i)))
		}
		fmt.Println(strings.Join(parts, "\t"))
	}
	r.L.SetTop(base)
	return nil
}

// tostring goes through Lua's own tostring so __tostring metamethods apply.
func tostring(L *lua.LState, v lua.LValue) string {
	if err := L.CallByParam(lua.P{
		Fn:      L.GetGlobal("tostring"),
		NRet:    1,
		Protect: true,
	}, v); err != nil {
		return v.String()
	}
	s := lua.LVAsString(L.Get(-1))
	L.Pop(1)
	return s
}

// isIncomplete reports whether a parse failed only because the chunk ran out
// of input, which is the cue to keep reading lines.
func isIncomplete(err error) bool {
	if err == nil {
		return false
	}
	apiErr, ok := err.(*lua.ApiError)
	if !ok {
		return false
	}
	if perr, ok := apiErr.Cause.(*parse.Error); ok {
		return perr.Pos.Line == parse.EOF
	}
	return strings.Contains(apiErr.Error(), "at EOF")
}

func readLine(in *bufio.Reader, p string) (string, error) {
	fmt.Print(p)
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
