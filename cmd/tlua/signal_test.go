//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A GUI program stops when it is told to, whatever it is waiting in: its
// event loop after the script, a modal form, a message box.
func TestAGUIProgramStopsWhenToldTo(t *testing.T) {
	if os.Getenv("TLUA_GUI_TESTS") == "" {
		t.Skip("set TLUA_GUI_TESTS=1 to run tests that open windows")
	}
	for name, src := range map[string]string{
		"bootgui": `local gui = bootgui()
			gui.Form{caption = "waiting"}:show()
			print("up")`,
		"showModal": `local gui = require "gui"
			print("up")
			gui.Form{caption = "waiting"}:showModal()`,
		"msgbox": `local gui = require "gui"
			print("up")
			gui.msgbox("waiting")`,
	} {
		t.Run(name, func(t *testing.T) {
			script := filepath.Join(t.TempDir(), "main.lua")
			write(t, script, src)
			stopped(t, exec.Command(bin(t), script))
		})
	}
	// A fused program, which runs its loop the same way.
	dir := t.TempDir()
	write(t, filepath.Join(dir, "main.lua"), `local gui = bootgui()
		gui.Form{caption = "waiting"}:show()
		print("up")`)
	t.Run("fused", func(t *testing.T) { stopped(t, exec.Command(fuseApp(t, dir))) })
}

// stopped starts cmd, waits until it says it is up, sends it SIGTERM and
// checks that it ends, saying so, with status 1.
func stopped(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for !strings.Contains(out.String(), "up") && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond) // into the loop
	cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		if code != 1 {
			t.Errorf("exit status %d, want 1 (stderr %q)", code, errb.String())
		}
		if !strings.Contains(errb.String(), "interrupted") && !strings.Contains(errb.String(), "context canceled") {
			t.Errorf("it does not say it was stopped: %q", errb.String())
		}
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Fatalf("still running 5 seconds after SIGTERM (stdout %q)", out.String())
	}
}
