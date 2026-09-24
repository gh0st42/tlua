package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRealServerFormats exercises the client against whatever language server
// is installed on the machine running the tests, and skips when there is none.
// The fake server in testdata proves the protocol handling; this proves it
// against a server that was not written to match it.
func TestRealServerFormats(t *testing.T) {
	command, args, ok := Find()
	if !ok {
		t.Skip("no language server on PATH")
	}
	t.Logf("using %s %v", command, args)

	dir := t.TempDir()
	path := filepath.Join(dir, "main.lua")
	messy := "local   t  =  {a=1,b=2}\nfor i=1,10   do\nprint( i ,t.a )\nend\n"
	if err := os.WriteFile(path, []byte(messy), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := Start(ctx, command, args, dir)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer client.Close()

	t.Logf("server name: %q, formats: %v", client.Name(), client.CanFormat())
	if !client.CanFormat() {
		t.Skip("this server does not offer formatting")
	}

	formatted, err := client.Format(ctx, path, messy, DefaultFormatOptions())
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	t.Logf("before:\n%s\nafter:\n%s", messy, formatted)
	if formatted == messy {
		t.Error("the server changed nothing")
	}
}
