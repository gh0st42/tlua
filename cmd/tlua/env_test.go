package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tlua/internal/interp"
)

// siteDir creates a directory holding one module, standing in for a machine's
// site-wide Lua library folder.
func siteDir(t *testing.T, mod, body string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, mod+".lua"), body)
	return dir
}

func listSep() string { return string(os.PathListSeparator) }

func TestIncludeEnvAddsDirectories(t *testing.T) {
	a := siteDir(t, "amod", `return "from a"`)
	b := t.TempDir()
	mkdirs(t, filepath.Join(b, "pkg"))
	write(t, filepath.Join(b, "pkg", "init.lua"), `return "from b/pkg"`)

	got := runEnv(t, t.TempDir(), []string{interp.EnvInclude + "=" + a + listSep() + b}, "",
		"-e", `print(require("amod"), require("pkg"))`)
	if want := "from a\tfrom b/pkg\n"; got.stdout != want {
		t.Fatalf("stdout = %q (stderr %q)", got.stdout, got.stderr)
	}
}

func TestIncludeEnvKeepsListOrder(t *testing.T) {
	first := siteDir(t, "dup", `return "first"`)
	second := siteDir(t, "dup", `return "second"`)

	got := runEnv(t, t.TempDir(), []string{interp.EnvInclude + "=" + first + listSep() + second}, "",
		"-e", `print(require("dup"))`)
	if strings.TrimSpace(got.stdout) != "first" {
		t.Fatalf("got %+v", got)
	}
}

func TestIncludeEnvStillFindsLocalAndDefaultPaths(t *testing.T) {
	site := siteDir(t, "sitemod", `return "site"`)
	cwd := t.TempDir()
	write(t, filepath.Join(cwd, "local.lua"), `return "local"`)

	got := runEnv(t, cwd, []string{interp.EnvInclude + "=" + site}, "",
		"-e", `print(require("sitemod"), require("local"))`)
	if want := "site\tlocal\n"; got.stdout != want {
		t.Fatalf("stdout = %q (stderr %q)", got.stdout, got.stderr)
	}
}

func TestPathEnvBeatsIncludeEnv(t *testing.T) {
	inc := siteDir(t, "dup", `return "include"`)
	pat := siteDir(t, "dup", `return "pattern"`)

	got := runEnv(t, t.TempDir(), []string{
		interp.EnvPath + "=" + filepath.Join(pat, "?.lua") + ";;",
		interp.EnvInclude + "=" + inc,
	}, "", "-e", `print(require("dup"))`)
	if strings.TrimSpace(got.stdout) != "pattern" {
		t.Fatalf("got %+v", got)
	}
}

func TestTluaPathBeatsLuaPath(t *testing.T) {
	tl := siteDir(t, "dup", `return "TLUA_PATH"`)
	lp := siteDir(t, "dup", `return "LUA_PATH"`)

	got := runEnv(t, t.TempDir(), []string{
		interp.EnvPath + "=" + filepath.Join(tl, "?.lua"),
		"LUA_PATH=" + filepath.Join(lp, "?.lua"),
	}, "", "-e", `print(require("dup"))`)
	if strings.TrimSpace(got.stdout) != "TLUA_PATH" {
		t.Fatalf("got %+v", got)
	}
}

func TestCommandLinePathBeatsEnvironment(t *testing.T) {
	env := siteDir(t, "dup", `return "env"`)
	cli := siteDir(t, "dup", `return "cli"`)

	got := runEnv(t, t.TempDir(), []string{interp.EnvInclude + "=" + env}, "",
		"-p", cli, "-e", `print(require("dup"))`)
	if strings.TrimSpace(got.stdout) != "cli" {
		t.Fatalf("got %+v", got)
	}
}

func TestScriptDirectoryBeatsEnvironment(t *testing.T) {
	env := siteDir(t, "dup", `return "env"`)
	scriptDir := t.TempDir()
	write(t, filepath.Join(scriptDir, "dup.lua"), `return "beside the script"`)
	write(t, filepath.Join(scriptDir, "main.lua"), `print(require("dup"))`)

	got := runEnv(t, t.TempDir(), []string{interp.EnvInclude + "=" + env}, "",
		filepath.Join(scriptDir, "main.lua"))
	if strings.TrimSpace(got.stdout) != "beside the script" {
		t.Fatalf("got %+v", got)
	}
}

func TestNoEnvOptionIgnoresTluaVariables(t *testing.T) {
	site := siteDir(t, "sitemod", `return "site"`)

	got := runEnv(t, t.TempDir(), []string{
		interp.EnvInclude + "=" + site,
		interp.EnvPath + "=" + filepath.Join(site, "?.lua"),
		interp.EnvInit + `=print("INIT-RAN")`,
	}, "", "-E", "-e", `print(pcall(require, "sitemod"))`)
	if !strings.HasPrefix(got.stdout, "false\t") {
		t.Fatalf("stdout = %q, want the require to fail under -E", got.stdout)
	}
	if strings.Contains(got.stdout, "INIT-RAN") {
		t.Errorf("-E still ran %s", interp.EnvInit)
	}
}

func TestPackagePathIsDeduplicated(t *testing.T) {
	site := siteDir(t, "m", "return 1")

	got := runEnv(t, t.TempDir(), []string{
		interp.EnvInclude + "=" + site + listSep() + site,
		"LUA_PATH=;;",
	}, "", "-p", site, "-e", `print(package.path)`)

	path := strings.TrimSpace(got.stdout)
	seen := map[string]int{}
	for _, p := range strings.Split(path, ";") {
		seen[p]++
	}
	for p, n := range seen {
		if n > 1 {
			t.Errorf("pattern %q appears %d times in %s", p, n, path)
		}
	}
}

func TestInitEnvVariables(t *testing.T) {
	got := runEnv(t, t.TempDir(), []string{interp.EnvInit + `=print("tlua init")`, `LUA_INIT=print("lua init")`}, "",
		"-e", `print("body")`)
	if want := "tlua init\nbody\n"; got.stdout != want {
		t.Fatalf("stdout = %q", got.stdout)
	}

	dir := t.TempDir()
	initFile := filepath.Join(dir, "init.lua")
	write(t, initFile, `print("from file")`)
	got = runEnv(t, t.TempDir(), []string{interp.EnvInit + "=@" + initFile}, "", "-e", `print("body")`)
	if want := "from file\nbody\n"; got.stdout != want {
		t.Fatalf("stdout = %q (stderr %q)", got.stdout, got.stderr)
	}
}

// A fused app is still a tlua process: site-wide directories apply to it too,
// though its own archive always wins.
func TestFusedAppHonoursIncludeEnv(t *testing.T) {
	site := siteDir(t, "sitemod", `return "site"`)
	dir := t.TempDir()
	mkdirs(t, filepath.Join(dir, "lib"))
	write(t, filepath.Join(dir, "lib", "util.lua"), `return "embedded"`)
	write(t, filepath.Join(dir, "main.lua"), `print(require("sitemod"), require("lib.util"))`)

	app := fuseApp(t, dir)
	cmd := appCmd(t, app, t.TempDir(), interp.EnvInclude+"="+site)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if want := "site\tembedded\n"; string(out) != want {
		t.Fatalf("stdout = %q", out)
	}
}
