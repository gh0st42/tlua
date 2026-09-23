package main

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fuseApp builds a standalone executable from src and returns its path.
func fuseApp(t *testing.T, src string, extra ...string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "app")
	args := append([]string{"fuse", "-o", out}, extra...)
	args = append(args, src)
	cmd := exec.Command(bin(t), args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fuse %v: %v\n%s", args, err, out)
	}
	return out
}

// appCmd prepares a fused app for running with extra environment variables.
func appCmd(t *testing.T, app, dir string, env ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(app)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

func runApp(t *testing.T, app, dir string, args ...string) result {
	t.Helper()
	cmd := exec.Command(app, args...)
	cmd.Dir = dir
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %s: %v (stderr %s)", app, err, errb.String())
		}
		code = exitErr.ExitCode()
	}
	return result{stdout: out.String(), stderr: errb.String(), code: code}
}

// sampleApp writes a small multi-file program and returns its directory.
func sampleApp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mkdirs(t, filepath.Join(dir, "lib"), filepath.Join(dir, "data"))
	write(t, filepath.Join(dir, "main.lua"), `
local util = require("lib.util")
local embed = require("embed")
print("args:", ...)
print("sum:", util.sum{1, 2, 3, 4})
print("kind:", embed.kind)
print("asset:", (embed.read("data/message.txt"):gsub("%s+$", "")))
print("levels:", dofile("data/levels.lua")[1])
`)
	write(t, filepath.Join(dir, "lib", "util.lua"),
		"local util = {}\nfunction util.sum(t) local n = 0 for _, v in ipairs(t) do n = n + v end return n end\nreturn util\n")
	write(t, filepath.Join(dir, "data", "message.txt"), "hello from the archive\n")
	write(t, filepath.Join(dir, "data", "levels.lua"), `return {"forest", "cave"}`)
	return dir
}

func mkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuseSingleLuaFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "greeter.lua")
	write(t, src, `print("single", select("#", ...), ...)`)

	app := fuseApp(t, src)
	got := runApp(t, app, t.TempDir(), "a", "b")
	if want := "single\t2\ta\tb\n"; got.stdout != want {
		t.Fatalf("stdout = %q (stderr %q)", got.stdout, got.stderr)
	}
}

func TestFuseDirectory(t *testing.T) {
	app := fuseApp(t, sampleApp(t))

	// A decoy module in the working directory must lose to the embedded one.
	cwd := t.TempDir()
	mkdirs(t, filepath.Join(cwd, "lib"), filepath.Join(cwd, "data"))
	write(t, filepath.Join(cwd, "lib", "util.lua"), "return { sum = function() return -1 end }")
	write(t, filepath.Join(cwd, "data", "levels.lua"), `return {"decoy"}`)

	got := runApp(t, app, cwd, "x")
	for _, want := range []string{"args:\tx", "sum:\t10", "kind:\tzip", "asset:\thello from the archive", "levels:\tforest"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout missing %q:\n%s%s", want, got.stdout, got.stderr)
		}
	}
}

func TestFuseZipArchive(t *testing.T) {
	// A zip made with "zip -r app.zip mygame": everything under one folder.
	archive := filepath.Join(t.TempDir(), "app.zip")
	writeZip(t, archive, map[string]string{
		"mygame/main.lua":  `print(require("mod").name)`,
		"mygame/mod.lua":   `return { name = "from zip" }`,
		"mygame/notes.txt": "ignored",
	})

	app := fuseApp(t, archive)
	if got := runApp(t, app, t.TempDir()); strings.TrimSpace(got.stdout) != "from zip" {
		t.Fatalf("got %+v", got)
	}
}

// TestAppendedZipRunsLikeLove covers `cat tlua app.zip > app`.
func TestAppendedZipRunsLikeLove(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "app.zip")
	writeZip(t, archive, map[string]string{
		"main.lua": `print("concatenated", ...)`,
	})

	interp, err := os.ReadFile(bin(t))
	if err != nil {
		t.Fatal(err)
	}
	zipped, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(t.TempDir(), "app")
	if err := os.WriteFile(app, append(interp, zipped...), 0o755); err != nil {
		t.Fatal(err)
	}

	got := runApp(t, app, t.TempDir(), "arg1")
	if want := "concatenated\targ1\n"; got.stdout != want {
		t.Fatalf("stdout = %q (stderr %q)", got.stdout, got.stderr)
	}
}

func TestFusedBinaryTakesNoOptions(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "opts.lua")
	write(t, src, `print(table.concat({...}, "|"))`)

	app := fuseApp(t, src)
	got := runApp(t, app, t.TempDir(), "-e", "print(1)", "-v")
	if want := "-e|print(1)|-v\n"; got.stdout != want {
		t.Fatalf("stdout = %q", got.stdout)
	}
}

func TestFuseOnFusedBinaryStripsOldPayload(t *testing.T) {
	first := fuseApp(t, sampleApp(t))

	dir := t.TempDir()
	src := filepath.Join(dir, "second.lua")
	write(t, src, `print("second")`)
	second := fuseApp(t, src, "--base", first)

	if got := runApp(t, second, t.TempDir()); strings.TrimSpace(got.stdout) != "second" {
		t.Fatalf("got %+v", got)
	}

	// The rebuilt app must not be carrying the first payload around.
	clean, err := os.Stat(bin(t))
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if grown := st.Size() - clean.Size(); grown > 1024 {
		t.Errorf("fused binary is %d bytes larger than the interpreter; old payload kept?", grown)
	}
}

func TestFusedAppReportsErrorsAndExits(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "boom.lua")
	write(t, src, `error("app blew up")`)

	app := fuseApp(t, src)
	got := runApp(t, app, t.TempDir())
	if got.code != 1 {
		t.Errorf("exit = %d, want 1", got.code)
	}
	if !strings.Contains(got.stderr, "app blew up") || !strings.HasPrefix(got.stderr, "app:") {
		t.Errorf("stderr = %q, want it prefixed with the app name", got.stderr)
	}
}

func TestFuseRejectsArchiveWithoutMain(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "bad.zip")
	writeZip(t, archive, map[string]string{"other.lua": "return 1"})

	out := filepath.Join(t.TempDir(), "app")
	cmd := exec.Command(bin(t), "fuse", "-o", out, archive)
	combined, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("fuse accepted an archive without main.lua")
	}
	if !strings.Contains(string(combined), "main.lua") {
		t.Errorf("unhelpful error: %s", combined)
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("a failed fuse left %s behind", out)
	}
}

func TestInterpreterStillWorksAfterFuseSupport(t *testing.T) {
	got := runCLI(t, t.TempDir(), "", "-e", `print("plain")`)
	if strings.TrimSpace(got.stdout) != "plain" {
		t.Fatalf("got %+v", got)
	}
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	write(t, path, buf.String())
}
