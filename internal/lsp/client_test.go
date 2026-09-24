package lsp

import (
	"context"
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
	fakePath  string
	buildErr  error
)

// fakeServer builds the fake language server in testdata and returns its path.
func fakeServer(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "fakelsp")
		if err != nil {
			buildErr = err
			return
		}
		fakePath = filepath.Join(dir, "fakelsp")
		out, err := exec.Command("go", "build", "-o", fakePath, "./testdata/fakelsp").CombinedOutput()
		if err != nil {
			buildErr = err
			t.Logf("build output: %s", out)
		}
	})
	if buildErr != nil {
		t.Fatalf("building fakelsp: %v", buildErr)
	}
	return fakePath
}

func startFake(t *testing.T, env ...string) *Client {
	t.Helper()
	for _, pair := range env {
		name, value, _ := strings.Cut(pair, "=")
		t.Setenv(name, value)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := Start(ctx, fakeServer(t), nil, t.TempDir())
	if err != nil {
		t.Fatalf("starting the server: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestStartAndFormat(t *testing.T) {
	client := startFake(t)
	if !client.CanFormat() {
		t.Fatal("the server said it formats, but CanFormat is false")
	}
	if got := client.Name(); got != "fakelsp" {
		t.Errorf("Name = %q, want the name the server gave", got)
	}

	path := filepath.Join(t.TempDir(), "main.lua")
	text := "local x = 1   \n\tprint(x)\t\nlocal y = 2\n"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	got, err := client.Format(ctx, path, text, DefaultFormatOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := "local x = 1\n  print(x)\nlocal y = 2\n"
	if got != want {
		t.Errorf("formatted text = %q, want %q", got, want)
	}
}

// Formatting the same document twice goes through didOpen and then didChange,
// and the server has to be looking at the newer text.
func TestFormatTwiceSyncsChanges(t *testing.T) {
	client := startFake(t)
	path := filepath.Join(t.TempDir(), "main.lua")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.Format(ctx, path, "first   \n", DefaultFormatOptions()); err != nil {
		t.Fatal(err)
	}
	got, err := client.Format(ctx, path, "second\t\nthird   \n", DefaultFormatOptions())
	if err != nil {
		t.Fatal(err)
	}
	if want := "second\nthird\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatOfAlreadyTidyTextChangesNothing(t *testing.T) {
	client := startFake(t)
	path := filepath.Join(t.TempDir(), "main.lua")
	text := "local x = 1\nprint(x)\n"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	got, err := client.Format(ctx, path, text, DefaultFormatOptions())
	if err != nil {
		t.Fatal(err)
	}
	if got != text {
		t.Errorf("got %q, want it untouched", got)
	}
}

func TestServerWithoutFormatting(t *testing.T) {
	client := startFake(t, "FAKELSP_NOFORMAT=1")
	if client.CanFormat() {
		t.Fatal("CanFormat is true for a server that does not offer it")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Format(ctx, "main.lua", "x\n", DefaultFormatOptions()); err == nil {
		t.Error("formatting was attempted anyway")
	}
}

// A server that never answers must not hold the editor up for ever.
func TestFormatGivesUpOnASilentServer(t *testing.T) {
	client := startFake(t, "FAKELSP_HANG=1")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.Format(ctx, "main.lua", "x   \n", DefaultFormatOptions())
	if err == nil {
		t.Fatal("a silent server was waited on for ever")
	}
	if !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "context") {
		t.Errorf("error = %v, want it to be about the deadline", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("gave up after %v", elapsed)
	}
}

func TestFormatReportsAServerError(t *testing.T) {
	client := startFake(t, "FAKELSP_ERROR=1")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Format(ctx, "main.lua", "x   \n", DefaultFormatOptions())
	if err == nil || !strings.Contains(err.Error(), "broken today") {
		t.Errorf("error = %v, want the server's message", err)
	}
}

func TestCloseStopsTheServer(t *testing.T) {
	client := startFake(t)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if client.cmd.ProcessState == nil || !client.cmd.ProcessState.Exited() {
		t.Error("the server is still running")
	}
	// Closing twice is harmless, since the editor closes on its way out.
	if err := client.Close(); err != nil {
		t.Errorf("the second Close returned %v", err)
	}
}

func TestStartFailsOnACommandThatIsNotAServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if client, err := Start(ctx, "echo", []string{"hello"}, t.TempDir()); err == nil {
		client.Close()
		t.Error("a command that speaks no protocol was accepted")
	}
}

func TestFindHonoursTheEnvironment(t *testing.T) {
	t.Setenv(EnvServer, "off")
	if _, _, ok := Find(); ok {
		t.Error("TLUA_LSP=off still found a server")
	}

	t.Setenv(EnvServer, "definitely-not-installed-xyz")
	if _, _, ok := Find(); ok {
		t.Error("a server that is not on PATH was found")
	}

	t.Setenv(EnvServer, fakeServer(t)+" --stdio")
	command, args, ok := Find()
	if !ok || command != fakeServer(t) {
		t.Errorf("Find = %q, %v, %v", command, args, ok)
	}
	if len(args) != 1 || args[0] != "--stdio" {
		t.Errorf("args = %v, want the ones given", args)
	}
}

func TestPathToURI(t *testing.T) {
	got := pathToURI("/tmp/a dir/main.lua")
	if want := "file:///tmp/a%20dir/main.lua"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
