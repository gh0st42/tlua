package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// An export writes every declaration, sub-modules in folders as require
// finds them, and an export again removes what tlua no longer declares but
// nothing else.
func TestExport(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "gone.lua"), []byte("---@meta gone\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "mine.lua"), []byte("-- the user's\n"), 0o644)
	n, err := Export(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gui.lua", "lfs.lua", "socket.lua", "socket/http.lua", "ltn12.lua", "tui.lua", "mine.lua"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Error(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "gone.lua")); err == nil {
		t.Error("gone.lua, which tlua no longer declares, is still there")
	}
	all, _ := declarations()
	if n != len(all) || n < 20 {
		t.Errorf("wrote %d of %d", n, len(all))
	}
	for name, data := range all {
		if !strings.HasPrefix(string(data), "---@meta") {
			t.Errorf("%s does not start with ---@meta, so a later export would not clean it up", name)
		}
	}
}

func readRC(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".luarc.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rc map[string]any
	if err := json.Unmarshal(data, &rc); err != nil {
		t.Fatal(err)
	}
	return rc
}

func TestWriteLuaRC(t *testing.T) {
	t.Run("new", func(t *testing.T) {
		project := t.TempDir()
		made, err := WriteLuaRC(project, "/lib")
		if err != nil || !made {
			t.Fatal(made, err)
		}
		again, err := WriteLuaRC(project, "/lib")
		if err != nil || again {
			t.Fatal(again, err)
		}
		rc := readRC(t, project)
		if rc["runtime.version"] != "Lua 5.1" || rc["$schema"] == nil || rc["workspace.checkThirdParty"] != false {
			t.Errorf("%v", rc)
		}
		if !reflect.DeepEqual(rc["workspace.library"], []any{"/lib"}) {
			t.Errorf("library %v, want [/lib] once", rc["workspace.library"])
		}
	})

	// A project's own settings stay, in the form it wrote them.
	t.Run("merged", func(t *testing.T) {
		project := t.TempDir()
		os.WriteFile(filepath.Join(project, ".luarc.json"), []byte(`{
			"runtime": {"version": "Lua 5.4", "path": ["src/?.lua"]},
			"workspace.library": ["/other"],
			"workspace": {"checkThirdParty": "Ask"},
			"diagnostics.globals": ["love", "_draw"],
			"format.enable": true
		}`), 0o644)
		made, err := WriteLuaRC(project, "/lib")
		if err != nil || made {
			t.Fatal(made, err)
		}
		rc := readRC(t, project)
		runtime := rc["runtime"].(map[string]any)
		if runtime["version"] != "Lua 5.1" || runtime["path"] == nil || rc["runtime.version"] != nil {
			t.Errorf("runtime %v, %v", runtime, rc["runtime.version"])
		}
		if !reflect.DeepEqual(rc["workspace.library"], []any{"/other", "/lib"}) {
			t.Errorf("library %v", rc["workspace.library"])
		}
		if rc["workspace"].(map[string]any)["checkThirdParty"] != "Ask" || rc["workspace.checkThirdParty"] != nil {
			t.Errorf("checkThirdParty, which the project chose, changed: %v", rc)
		}
		if !reflect.DeepEqual(rc["diagnostics.globals"], []any{"love", "_draw", "_init", "_update"}) {
			t.Errorf("globals %v", rc["diagnostics.globals"])
		}
		if rc["format.enable"] != true || rc["$schema"] != nil {
			t.Errorf("%v", rc)
		}
	})

	// A file it cannot read is left as it is.
	t.Run("not JSON", func(t *testing.T) {
		project := t.TempDir()
		path := filepath.Join(project, ".luarc.json")
		os.WriteFile(path, []byte("{ // a comment\n}"), 0o644)
		if _, err := WriteLuaRC(project, "/lib"); err == nil {
			t.Error("no error")
		}
		if data, _ := os.ReadFile(path); string(data) != "{ // a comment\n}" {
			t.Errorf("changed to %q", data)
		}
	})
}

func TestCommand(t *testing.T) {
	lib, project := t.TempDir(), t.TempDir()
	var out, errs strings.Builder
	if code := command([]string{"luarc", "-lib", lib, project}, &out, &errs); code != 0 {
		t.Fatalf("status %d: %s", code, errs.String())
	}
	if _, err := os.Stat(filepath.Join(lib, "socket", "http.lua")); err != nil {
		t.Error(err)
	}
	if rc := readRC(t, project); !reflect.DeepEqual(rc["workspace.library"], []any{lib}) {
		t.Errorf("library %v", rc["workspace.library"])
	}
	out.Reset()
	if code := command([]string{"export", lib}, &out, &errs); code != 0 || !strings.Contains(out.String(), "tlua lib luarc") {
		t.Errorf("status %d: %s", code, out.String())
	}
	if code := command([]string{"nonsense"}, &out, &errs); code != 2 {
		t.Errorf("status %d for an unknown command", code)
	}
}
