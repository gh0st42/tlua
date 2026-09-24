package interp

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
)

// SyntaxError is a compile-time error with the position that caused it, so an
// editor can put the cursor on the offending line.
type SyntaxError struct {
	Line    int // 0 when the error has no position, e.g. at end of input
	Column  int
	Message string
}

func (e *SyntaxError) Error() string {
	if e.Line <= 0 {
		return e.Message
	}
	return fmt.Sprintf("line %d: %s", e.Line, e.Message)
}

// CheckSyntax compiles src without running it, the way a compiler's syntax
// pass would, and reports the first error it finds.
func CheckSyntax(src, name string) error {
	chunk, err := parse.Parse(strings.NewReader(src), name)
	if err == nil {
		_, err = lua.Compile(chunk, name)
	}
	if err == nil {
		return nil
	}
	if perr, ok := err.(*parse.Error); ok {
		line := perr.Pos.Line
		if line == parse.EOF {
			line = 0
		}
		msg := strings.TrimSpace(perr.Message)
		if perr.Token != "" && line > 0 {
			msg = fmt.Sprintf("%s (near '%s')", msg, perr.Token)
		}
		return &SyntaxError{Line: line, Column: perr.Pos.Column, Message: msg}
	}
	return &SyntaxError{Message: strings.TrimSpace(err.Error())}
}
