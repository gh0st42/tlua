package lsp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSignatureHelp(t *testing.T) {
	client := startFake(t)
	if !client.CanSignature() {
		t.Fatal("CanSignature is false")
	}

	path := filepath.Join(t.TempDir(), "main.lua")
	text := "local s = string.format(\n"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	help, err := client.Signature(ctx, path, text, Position{Line: 0, Character: 24})
	if err != nil {
		t.Fatal(err)
	}
	if help == nil {
		t.Fatal("no signature help came back")
	}

	signature, active, ok := help.Active()
	if !ok {
		t.Fatal("Active found no signature")
	}
	if signature.Label != "string.format(format, ...)" {
		t.Errorf("label = %q", signature.Label)
	}
	if active != 0 {
		t.Errorf("active parameter = %d, want the first", active)
	}
	// This signature labels its parameters with offsets into the label.
	if got := signature.Parameters[0].Text(signature.Label); got != "format" {
		t.Errorf("first parameter = %q", got)
	}
	if got := signature.Parameters[1].Text(signature.Label); got != "..." {
		t.Errorf("second parameter = %q", got)
	}
}

// The parameter being typed is the one after the last comma.
func TestSignatureHelpFollowsTheComma(t *testing.T) {
	client := startFake(t)
	path := filepath.Join(t.TempDir(), "main.lua")
	text := "string.format(\"%d\", x\n"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	help, err := client.Signature(ctx, path, text, Position{Line: 0, Character: 21})
	if err != nil {
		t.Fatal(err)
	}
	_, active, ok := help.Active()
	if !ok {
		t.Fatal("no signature")
	}
	if active != 1 {
		t.Errorf("active parameter = %d, want the second", active)
	}
}

// Outside a call there is nothing to say, which is not an error.
func TestSignatureHelpOutsideACall(t *testing.T) {
	client := startFake(t)
	path := filepath.Join(t.TempDir(), "main.lua")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	help, err := client.Signature(ctx, path, "local x = 1\n", Position{Line: 0, Character: 11})
	if err != nil {
		t.Fatalf("outside a call was an error: %v", err)
	}
	if help != nil {
		t.Errorf("got %+v", help)
	}
}

func TestSignatureOnAServerWithoutIt(t *testing.T) {
	client := startFake(t, "FAKELSP_NOSIGNATURE=1")
	if client.CanSignature() {
		t.Fatal("CanSignature is true for a server without it")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Signature(ctx, "main.lua", "f(\n", Position{}); err == nil {
		t.Error("it was asked anyway")
	}
}

func TestSignatureTriggerCharacters(t *testing.T) {
	client := startFake(t)
	got := strings.Join(client.SignatureTriggerCharacters(), "")
	if got != "(, )" {
		t.Errorf("trigger characters = %q, want the ones the server named", got)
	}
	// A server that names none gets the two that matter.
	if got := signatureCharacters(json.RawMessage(`true`)); strings.Join(got, "") != "(," {
		t.Errorf("got %v", got)
	}
	if got := signatureCharacters(nil); got != nil {
		t.Errorf("got %v", got)
	}
}

// A parameter's label is either its text or a pair of UTF-16 offsets.
func TestParameterSpan(t *testing.T) {
	const label = "f(first, second)"

	byText := ParameterInformation{Label: json.RawMessage(`"second"`)}
	start, end, ok := byText.Span(label)
	if !ok || label[start:end] != "second" {
		t.Errorf("by text: %d..%d ok=%v", start, end, ok)
	}

	byOffsets := ParameterInformation{Label: json.RawMessage(`[2,7]`)}
	start, end, ok = byOffsets.Span(label)
	if !ok || label[start:end] != "first" {
		t.Errorf("by offsets: %d..%d ok=%v", start, end, ok)
	}

	// Offsets count UTF-16 units, so a wide character counts twice.
	const wide = "f(\U0001F600, x)"
	byWideOffsets := ParameterInformation{Label: json.RawMessage(`[2,4]`)}
	if start, end, ok := byWideOffsets.Span(wide); !ok || wide[start:end] != "\U0001F600" {
		t.Errorf("wide: %d..%d ok=%v text=%q", start, end, ok, wide[start:end])
	}

	missing := ParameterInformation{Label: json.RawMessage(`"absent"`)}
	if _, _, ok := missing.Span(label); ok {
		t.Error("a parameter that is not in the label was found in it")
	}
}

// Active settles which signature and parameter to show, whichever way the
// server chose to say it.
func TestSignatureHelpActive(t *testing.T) {
	two := 2
	one := 1
	help := &SignatureHelp{
		Signatures: []SignatureInformation{
			{Label: "first"},
			{Label: "second", ActiveParameter: &two},
		},
		ActiveSignature: &one,
		ActiveParameter: &one,
	}
	signature, active, ok := help.Active()
	if !ok || signature.Label != "second" {
		t.Errorf("signature = %q", signature.Label)
	}
	// The signature's own active parameter wins over the help's.
	if active != 2 {
		t.Errorf("active = %d, want 2", active)
	}

	// Nothing said: the first signature, no parameter picked out.
	help = &SignatureHelp{Signatures: []SignatureInformation{{Label: "only"}}}
	signature, active, ok = help.Active()
	if !ok || signature.Label != "only" || active != -1 {
		t.Errorf("got %q, %d, %v", signature.Label, active, ok)
	}

	if _, _, ok := (*SignatureHelp)(nil).Active(); ok {
		t.Error("nothing at all reported a signature")
	}
}
