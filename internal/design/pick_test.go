package design

import (
	"os"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// With no folder named, tlua design asks which project to open: the one
// in the current folder (started there if it has none), another, or none.
// answerStart and answerDir answer the questions, so nothing is shown.
func TestDesignerPicksAProjectWhenNoneIsNamed(t *testing.T) {
	t.Setenv("TLUA_LSP", "off")
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd) // opening a project moves into its forms/

	other := t.TempDir()
	// A project there already: the designer's own, made the way it makes one.
	r, _ := newLua(t)
	r.L.SetGlobal("other", lua.LString(other))
	run(t, r.L, `require("design.project").create(other)`)

	for _, c := range []struct {
		name, answer, answerDir string
		create                  bool
		want                    string // "here", "other", "none" or "empty"
	}{
		{name: "this folder, made a project", answer: "here", want: "here"},
		{name: "another project", answer: "choose", answerDir: other, want: "other"},
		{name: "an empty folder, made a project", answer: "choose", answerDir: "empty", create: true, want: "empty"},
		{name: "an empty folder, but no", answer: "choose", answerDir: "empty", want: "none"},
		{name: "no project", answer: "none", want: "none"},
	} {
		t.Run(c.name, func(t *testing.T) {
			here, empty := t.TempDir(), t.TempDir()
			dir := map[string]string{"here": here, "other": other, "empty": empty, "none": ""}[c.want]
			answerDir := c.answerDir
			if answerDir == "empty" {
				answerDir = empty
			}
			r, _ := newLua(t)
			L := r.L
			L.SetGlobal("here", lua.LString(here))
			L.SetGlobal("answer", lua.LString(c.answer))
			L.SetGlobal("answerDir", lua.LString(answerDir))
			L.SetGlobal("create", lua.LBool(c.create))
			L.SetGlobal("want", lua.LString(dir))
			run(t, L, `
local gui = require "gui"
-- No project in the empty folder, when asked: what a user who says no does.
gui.msgbox = function() return "no" end
local D = require("design.main").start{dir = here, pick = true, show = false,
  answerStart = answer, answerDir = answerDir ~= "" and answerDir or nil, create = create}
assert(D.dir == (want ~= "" and want or nil), "opened " .. tostring(D.dir) .. ", not " .. want)
D.win:close()`)
			// Starting a project here makes one; anything else leaves it be.
			_, err := os.Stat(filepath.Join(here, "main.lua"))
			if made := err == nil; made != (c.want == "here") {
				t.Errorf("main.lua in the current folder: %v", made)
			}
			os.Chdir(cwd)
		})
	}
}
