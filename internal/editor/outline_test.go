package editor

import (
	"fmt"
	"strings"
	"testing"
)

func formatOutline(entries []outlineEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, fmt.Sprintf("%d %s %s depth%d", e.Line, e.Kind, e.Name, e.Depth))
	}
	return out
}

func TestOutlineFindsEveryShapeOfDefinition(t *testing.T) {
	src := `local M = {}

function greet(name)
  return "hi " .. name
end

local function helper(x)
  return x * 2
end

function M.render(data)
  local function inner()
    return 1
  end
  return inner()
end

function M:method(arg)
  return self
end

M.assigned = function()
  return true
end

local anon = function()
  return false
end

M.table = {
  nested = function() return 1 end,
}
`
	got := formatOutline(outline(src))
	want := []string{
		"3 global greet depth0",
		"7 local helper depth0",
		"11 global M.render depth0",
		"12 nested inner depth1",
		"18 global M:method depth0",
		"22 global M.assigned depth0",
		"26 local anon depth0",
		"31 global M.table.nested depth0",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("outline:\n got %q\nwant %q", got, want)
	}
}

// While the buffer is half-written the parser refuses it, and the scanner has
// to carry the list on its own.
func TestOutlineFallsBackWhenTheBufferDoesNotParse(t *testing.T) {
	src := `function first()
  return 1
end

local function second(

function third()
`
	got := formatOutline(outline(src))
	want := []string{"1 global first depth0", "5 local second depth0", "7 global third depth0"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("outline:\n got %q\nwant %q", got, want)
	}
}

func TestOutlineIgnoresCommentedDefinitions(t *testing.T) {
	src := "-- function ghost()\nfunction real()\nend\n"
	got := formatOutline(outline(src))
	if len(got) != 1 || got[0] != "2 global real depth0" {
		t.Errorf("outline = %q", got)
	}
}

func TestOutlineOfFileWithoutFunctions(t *testing.T) {
	if got := outline("local x = 1\nprint(x)\n"); len(got) != 0 {
		t.Errorf("outline = %v, want nothing", got)
	}
}

// A function defined inside another one is nested however it is named, which is
// what the list colours differently.
func TestOutlineMarksDefinitionsInsideFunctionsAsNested(t *testing.T) {
	src := `function outer()
  func_callback = function() return 1 end
  local helper = function() return 2 end
  function deep()
    inner_callback = function() return 3 end
  end
end

func_toplevel = function() return 4 end
`
	got := formatOutline(outline(src))
	want := []string{
		"1 global outer depth0",
		"2 nested func_callback depth1",
		"3 nested helper depth1",
		"4 nested deep depth1",
		"5 nested inner_callback depth2",
		"9 global func_toplevel depth0",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("outline:\n got %q\nwant %q", got, want)
	}
}

// The fallback scanner has only indentation to go on, and uses it.
func TestOutlineByScanMarksIndentedDefinitionsAsNested(t *testing.T) {
	src := "function outer(\n  local cb = function() end\nfunction top()\n"
	got := formatOutline(outline(src))
	want := []string{"1 global outer depth0", "2 nested cb depth1", "3 global top depth0"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("outline:\n got %q\nwant %q", got, want)
	}
}
