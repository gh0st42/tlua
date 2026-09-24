package lsp

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestDiagnosticsArriveUnprompted(t *testing.T) {
	client := startFake(t)

	reports := make(chan []Diagnostic, 8)
	var reported string
	client.OnDiagnostics(func(path string, list []Diagnostic) {
		reported = path
		reports <- list
	})

	path := filepath.Join(t.TempDir(), "main.lua")
	if err := client.Sync(path, "local x = 1\nlocal y = ??\nprint(y) !!\n"); err != nil {
		t.Fatal(err)
	}

	var list []Diagnostic
	select {
	case list = <-reports:
	case <-time.After(5 * time.Second):
		t.Fatal("no diagnostics arrived")
	}

	if reported != path {
		t.Errorf("reported for %q, want %q", reported, path)
	}
	if len(list) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %+v", len(list), list)
	}

	first := list[0]
	if first.Severity != SeverityError || first.Range.Start.Line != 1 {
		t.Errorf("first = %+v", first)
	}
	if first.Range.Start.Character != 10 || first.Range.End.Character != 12 {
		t.Errorf("first range = %+v, want it over the marker", first.Range)
	}
	if first.Message == "" || first.Source != "fakelsp" {
		t.Errorf("first = %+v", first)
	}
	if second := list[1]; second.Severity != SeverityWarning || second.Range.Start.Line != 2 {
		t.Errorf("second = %+v", second)
	}

	// The client keeps the latest report, so it can be asked as well as told.
	if got := client.Diagnostics(path); len(got) != 2 {
		t.Errorf("Diagnostics returned %d", len(got))
	}
}

// A document that is put right is reported as clean, which has to clear what was
// there before.
func TestDiagnosticsAreClearedWhenPutRight(t *testing.T) {
	client := startFake(t)
	reports := make(chan []Diagnostic, 8)
	client.OnDiagnostics(func(_ string, list []Diagnostic) { reports <- list })

	path := filepath.Join(t.TempDir(), "main.lua")
	if err := client.Sync(path, "local y = ??\n"); err != nil {
		t.Fatal(err)
	}
	if list := <-reports; len(list) != 1 {
		t.Fatalf("got %d diagnostics", len(list))
	}

	if err := client.Sync(path, "local y = 1\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case list := <-reports:
		if len(list) != 0 {
			t.Errorf("got %d diagnostics, want none", len(list))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the server did not report the document as clean")
	}
	if got := client.Diagnostics(path); len(got) != 0 {
		t.Errorf("the client kept %d stale diagnostics", len(got))
	}
}

func TestDiagnosticsCanBeAbsentAltogether(t *testing.T) {
	client := startFake(t, "FAKELSP_NODIAGNOSTICS=1")
	client.OnDiagnostics(func(string, []Diagnostic) {
		t.Error("a server that reports nothing reported something")
	})

	path := filepath.Join(t.TempDir(), "main.lua")
	if err := client.Sync(path, "local y = ??\n"); err != nil {
		t.Fatal(err)
	}
	// Give it a moment to be wrong in.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	<-ctx.Done()
	if got := client.Diagnostics(path); len(got) != 0 {
		t.Errorf("got %d diagnostics", len(got))
	}
}

func TestURIToPath(t *testing.T) {
	cases := map[string]string{
		"file:///tmp/a%20dir/main.lua": "/tmp/a dir/main.lua",
		"file:///tmp/main.lua":         "/tmp/main.lua",
		"not a uri":                    "not a uri",
	}
	for uri, want := range cases {
		if got := URIToPath(uri); got != want {
			t.Errorf("URIToPath(%q) = %q, want %q", uri, got, want)
		}
	}
	// A path goes out and comes back unchanged.
	const path = "/tmp/some dir/main.lua"
	if got := URIToPath(pathToURI(path)); got != path {
		t.Errorf("round trip gave %q", got)
	}
}

func TestSeverityNames(t *testing.T) {
	cases := map[Severity]string{
		SeverityError: "error", SeverityWarning: "warning",
		SeverityInformation: "note", SeverityHint: "hint", Severity(99): "error",
	}
	for severity, want := range cases {
		if got := severity.String(); got != want {
			t.Errorf("%d = %q, want %q", severity, got, want)
		}
	}
}
