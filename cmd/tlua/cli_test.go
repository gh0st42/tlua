package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

// bin builds the interpreter once and hands back its path.
func bin(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "tlua-build")
		if err != nil {
			buildErr = err
			return
		}
		binPath = filepath.Join(dir, "tlua")
		out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput()
		if err != nil {
			buildErr = err
			t.Logf("build output: %s", out)
		}
	})
	if buildErr != nil {
		t.Fatalf("building tlua: %v", buildErr)
	}
	return binPath
}

type result struct {
	stdout, stderr string
	code           int
}

func runCLI(t *testing.T, dir, stdin string, args ...string) result {
	t.Helper()
	return runEnv(t, dir, nil, stdin, args...)
}

// runEnv runs the interpreter with extra environment variables.
func runEnv(t *testing.T, dir string, env []string, stdin string, args ...string) result {
	t.Helper()
	cmd := exec.Command(bin(t), args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("running tlua %v: %v", args, err)
		}
		code = exitErr.ExitCode()
	}
	return result{stdout: out.String(), stderr: errb.String(), code: code}
}

// modulePath resolves a path given relative to the module root, two
// directories above this package.
func modulePath(t *testing.T, rel string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// TestRunScriptFromAnotherDirectory is the headline case: a script that
// require()s its neighbours still resolves them when run from elsewhere.
func TestRunScriptFromAnotherDirectory(t *testing.T) {
	script := modulePath(t, "examples/hello.lua")
	got := runCLI(t, t.TempDir(), "", script, "tester")
	if got.code != 0 {
		t.Fatalf("exit %d, stderr: %s", got.code, got.stderr)
	}
	for _, want := range []string{"hello, tester!", "squares:\t1, 4, 9, 16", "sum:\t10", "coroutine:\t2\t4"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, got.stdout)
		}
	}
}

func TestRequireFromWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "mod.lua"), "return { n = 7 }")
	write(t, filepath.Join(dir, "main.lua"), `print(require("mod").n)`)

	got := runCLI(t, dir, "", "main.lua")
	if got.code != 0 || strings.TrimSpace(got.stdout) != "7" {
		t.Fatalf("got %+v", got)
	}
}

func TestRequirePackageInitFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "pkg", "init.lua"), "return { name = 'pkg' }")
	write(t, filepath.Join(dir, "main.lua"), `print(require("pkg").name)`)

	got := runCLI(t, t.TempDir(), "", filepath.Join(dir, "main.lua"))
	if strings.TrimSpace(got.stdout) != "pkg" {
		t.Fatalf("got %+v", got)
	}
}

func TestSearchPathOptionAndLibraryOption(t *testing.T) {
	lib := modulePath(t, "examples/lib")
	got := runCLI(t, t.TempDir(), "", "-p", lib, "-l", "greet", "-l", "u=util",
		"-e", `print(greet.hello("lib"), u.sum{1,2,3})`)
	if want := "hello, lib!\t6\n"; got.stdout != want {
		t.Fatalf("stdout = %q (stderr %q)", got.stdout, got.stderr)
	}
}

func TestLuaPathEnvironment(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "envmod.lua"), "return 'from LUA_PATH'")

	got := runEnv(t, t.TempDir(), []string{"LUA_PATH=" + filepath.Join(dir, "?.lua") + ";;"}, "",
		"-e", `print(require("envmod"))`)
	if strings.TrimSpace(got.stdout) != "from LUA_PATH" {
		t.Fatalf("got %+v", got)
	}
}

func TestArgTable(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.lua"), `print(arg[0], arg[1], arg[2], select("#", ...), ...)`)

	got := runCLI(t, dir, "", "a.lua", "one", "two")
	want := "a.lua\tone\ttwo\t2\tone\ttwo\n"
	if got.stdout != want {
		t.Fatalf("stdout = %q", got.stdout)
	}
}

func TestStdinScript(t *testing.T) {
	got := runCLI(t, t.TempDir(), `print("stdin", ...)`, "-", "x")
	if want := "stdin\tx\n"; got.stdout != want {
		t.Fatalf("stdout = %q", got.stdout)
	}
	got = runCLI(t, t.TempDir(), `print("piped")`)
	if want := "piped\n"; got.stdout != want {
		t.Fatalf("stdout = %q", got.stdout)
	}
}

func TestShebangIsSkipped(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "s.lua"), "#!/usr/bin/env tlua\nprint('ok')\n")
	if got := runCLI(t, dir, "", "s.lua"); strings.TrimSpace(got.stdout) != "ok" {
		t.Fatalf("got %+v", got)
	}
}

func TestErrorsExitNonZero(t *testing.T) {
	got := runCLI(t, t.TempDir(), "", "-e", `error("kaboom")`)
	if got.code != 1 {
		t.Errorf("exit = %d, want 1", got.code)
	}
	if !strings.Contains(got.stderr, "kaboom") {
		t.Errorf("stderr = %q", got.stderr)
	}

	got = runCLI(t, t.TempDir(), "", "-e", `require("definitely_missing")`)
	if got.code != 1 || !strings.Contains(got.stderr, "definitely_missing") {
		t.Errorf("got %+v", got)
	}

	got = runCLI(t, t.TempDir(), "", "no_such_file.lua")
	if got.code != 1 {
		t.Errorf("exit = %d, want 1", got.code)
	}
}

func TestInteractiveMode(t *testing.T) {
	in := "x = 20\nx + 22\nfunction f(a)\n  return a * 2\nend\nf(21)\n"
	got := runCLI(t, t.TempDir(), in, "-i")
	if strings.Count(got.stdout, "42") != 2 {
		t.Fatalf("expected two 42s, stdout = %q", got.stdout)
	}
}

func TestInterruptStopsRunningScript(t *testing.T) {
	cmd := exec.Command(bin(t), "-e", "while true do end")
	cmd.Dir = t.TempDir()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	time.Sleep(300 * time.Millisecond)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("interpreter ignored SIGINT")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
