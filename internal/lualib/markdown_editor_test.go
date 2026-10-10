package lualib

import (
	"os"
	"testing"
)

// TestMarkdownEditor runs testdata/markdown_editor.lua: markdown.editor's
// tests, written in Lua, as microword had them.
func TestMarkdownEditor(t *testing.T) {
	src, err := os.ReadFile("testdata/markdown_editor.lua")
	if err != nil {
		t.Fatal(err)
	}
	run(t, string(src))
}
