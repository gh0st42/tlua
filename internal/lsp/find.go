package lsp

import (
	"os"
	"os/exec"
	"strings"
)

// EnvServer names the environment variable that picks a language server: a
// command with any arguments, or "off" to do without one.
const EnvServer = "TLUA_LSP"

// knownServers are the Lua language servers this looks for, in the order it
// prefers them.
var knownServers = []string{
	"lua-language-server", // sumneko, formats through EmmyLuaCodeStyle
	"emmylua_ls",          // the EmmyLua server
	"lua-lsp",             // the Lua-written server
}

// Find reports the language server to use: whatever TLUA_LSP names, else the
// first known server on PATH. It reports false when there is none, which is not
// an error: the editor simply works without one.
func Find() (command string, args []string, ok bool) {
	if chosen := strings.TrimSpace(os.Getenv(EnvServer)); chosen != "" {
		switch strings.ToLower(chosen) {
		case "off", "none", "no", "0":
			return "", nil, false
		}
		fields := strings.Fields(chosen)
		path, err := exec.LookPath(fields[0])
		if err != nil {
			return "", nil, false
		}
		return path, fields[1:], true
	}

	for _, name := range knownServers {
		if path, err := exec.LookPath(name); err == nil {
			return path, defaultArgs(name), true
		}
	}
	return "", nil, false
}

// defaultArgs are the arguments a given server needs to speak over stdio.
func defaultArgs(name string) []string {
	switch name {
	case "emmylua_ls":
		return []string{"--stdio"}
	default:
		return nil
	}
}
