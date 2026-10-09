package gui

import (
	"testing"

	"tlua/internal/interp"
)

func TestReadyInstallsBootgui(t *testing.T) {
	in := interp.New(&interp.Options{})
	defer in.Close()

	b := Ready(in, nil)
	if b == nil {
		t.Fatal("expected boot state")
	}
	if b.Wanted() {
		t.Fatal("bootgui should start unset")
	}
	if err := in.DoString(`assert(bootgui())`, "=test"); err != nil {
		t.Fatalf("bootgui call failed: %v", err)
	}
	if !b.Wanted() {
		t.Fatal("bootgui should record the request")
	}
}

func TestTooLateBlocksBootgui(t *testing.T) {
	in := interp.New(&interp.Options{})
	defer in.Close()

	b := Ready(in, nil)
	b.TooLate()
	if err := in.DoString(`local ok, err = pcall(bootgui); assert(not ok and err:match("too late"))`, "=test"); err != nil {
		t.Fatalf("bootgui late check failed: %v", err)
	}
}

func TestBootguiReturnsTheModule(t *testing.T) {
	in := interp.New(&interp.Options{})
	defer in.Close()

	b := Ready(in, nil)
	if err := in.DoString(`local gui = bootgui(); assert(gui == require("gui") and gui.Form)`, "=test"); err != nil {
		t.Fatal(err)
	}
	if !b.app.booted {
		t.Fatal("bootgui should make show() return at once")
	}
	// Nothing was shown, so the loop has nothing to wait for.
	if err := b.Show(); err != nil {
		t.Fatalf("show with no forms: %v", err)
	}
	if b.app.booted {
		t.Fatal("after the loop, show() should wait for its form again")
	}
}
