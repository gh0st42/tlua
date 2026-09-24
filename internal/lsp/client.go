package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client talks to one language server over its standard input and output.
//
// It implements the little of the protocol that formatting needs: the
// initialize handshake, keeping the server's copy of a document up to date, and
// textDocument/formatting. Notifications from the server are read and dropped;
// requests from the server are answered blandly, because a server that is
// waiting for an answer will not get on with the job.
type Client struct {
	name string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	mu      sync.Mutex
	nextID  int
	pending map[int]chan message
	docs    map[string]int // open documents and their version
	closed  bool

	writeMu sync.Mutex

	canFormat      bool
	canComplete    bool
	canHover       bool
	canSignature   bool
	triggerChars   []string
	signatureChars []string
	serverName     string
}

// shutdownTimeout is how long the client waits for a server to leave before it
// stops asking.
const shutdownTimeout = 2 * time.Second

// Start launches a language server and runs the initialize handshake. Root is
// the directory the server should treat as the workspace.
func Start(ctx context.Context, command string, args []string, root string) (*Client, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = root
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// A server's log chatter would otherwise land on the editor's screen.
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	c := &Client{
		name:    filepath.Base(command),
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		pending: make(map[int]chan message),
		docs:    make(map[string]int),
	}
	go c.read()

	if err := c.initialize(ctx, root); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// Name is the server's own name if it gave one, else the binary's.
func (c *Client) Name() string {
	if c.serverName != "" {
		return c.serverName
	}
	return c.name
}

// CanFormat reports whether the server offers document formatting.
func (c *Client) CanFormat() bool { return c.canFormat }

func (c *Client) initialize(ctx context.Context, root string) error {
	params := map[string]any{
		"processId": os.Getpid(),
		"clientInfo": map[string]string{
			"name":    "tlua",
			"version": "0.1.0",
		},
		"rootUri": pathToURI(root),
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"synchronization": map[string]any{
					"dynamicRegistration": false,
					"didSave":             true,
				},
				"formatting": map[string]any{"dynamicRegistration": false},
				"completion": map[string]any{
					"dynamicRegistration": false,
					"completionItem": map[string]any{
						"snippetSupport":       false,
						"documentationFormat":  []string{"markdown", "plaintext"},
						"insertReplaceSupport": false,
						"deprecatedSupport":    false,
						"resolveSupport":       map[string]any{"properties": []string{"documentation", "detail"}},
					},
					"contextSupport": true,
				},
				"hover": map[string]any{
					"dynamicRegistration": false,
					"contentFormat":       []string{"markdown", "plaintext"},
				},
				"signatureHelp": map[string]any{
					"dynamicRegistration": false,
					"signatureInformation": map[string]any{
						"documentationFormat":    []string{"markdown", "plaintext"},
						"parameterInformation":   map[string]any{"labelOffsetSupport": true},
						"activeParameterSupport": true,
					},
					"contextSupport": true,
				},
			},
			"workspace": map[string]any{"configuration": true},
		},
		"workspaceFolders": nil,
	}

	var result initializeResult
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	c.canFormat = hasCapability(result.Capabilities.DocumentFormattingProvider)
	c.canComplete = hasCapability(result.Capabilities.CompletionProvider)
	c.triggerChars = triggerCharacters(result.Capabilities.CompletionProvider)
	c.canSignature = hasCapability(result.Capabilities.SignatureHelpProvider)
	c.signatureChars = signatureCharacters(result.Capabilities.SignatureHelpProvider)
	c.canHover = hasCapability(result.Capabilities.HoverProvider)
	c.serverName = result.ServerInfo.Name

	return c.notify("initialized", map[string]any{})
}

// hasCapability reads a capability that may be a boolean or an options object.
func hasCapability(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var flag bool
	if err := json.Unmarshal(raw, &flag); err == nil {
		return flag
	}
	var options map[string]any
	return json.Unmarshal(raw, &options) == nil
}

// Sync brings the server's copy of a document in line with the editor's.
func (c *Client) Sync(path, text string) error {
	uri := pathToURI(path)

	c.mu.Lock()
	version, open := c.docs[uri]
	version++
	c.docs[uri] = version
	c.mu.Unlock()

	if !open {
		return c.notify("textDocument/didOpen", map[string]any{
			"textDocument": map[string]any{
				"uri":        uri,
				"languageId": "lua",
				"version":    version,
				"text":       text,
			},
		})
	}
	return c.notify("textDocument/didChange", map[string]any{
		"textDocument": map[string]any{"uri": uri, "version": version},
		// The client declares no incremental sync, so a change is the whole text.
		"contentChanges": []map[string]any{{"text": text}},
	})
}

// DidSave tells the server the file is on disk, which is when servers that run
// checks tend to run them.
func (c *Client) DidSave(path, text string) error {
	return c.notify("textDocument/didSave", map[string]any{
		"textDocument": map[string]any{"uri": pathToURI(path)},
		"text":         text,
	})
}

// DidClose tells the server to forget a document.
func (c *Client) DidClose(path string) error {
	uri := pathToURI(path)
	c.mu.Lock()
	_, open := c.docs[uri]
	delete(c.docs, uri)
	c.mu.Unlock()
	if !open {
		return nil
	}
	return c.notify("textDocument/didClose", map[string]any{
		"textDocument": map[string]any{"uri": uri},
	})
}

// Format asks the server to format a document and returns the formatted text.
// It syncs the text first, so the server formats what the editor has rather
// than what is on disk.
func (c *Client) Format(ctx context.Context, path, text string, options FormatOptions) (string, error) {
	if !c.canFormat {
		return "", errors.New("the language server does not offer formatting")
	}
	if err := c.Sync(path, text); err != nil {
		return "", err
	}

	var edits []TextEdit
	err := c.call(ctx, "textDocument/formatting", map[string]any{
		"textDocument": map[string]any{"uri": pathToURI(path)},
		"options":      options,
	}, &edits)
	if err != nil {
		return "", err
	}
	return ApplyEdits(text, edits)
}

// Close asks the server to shut down, then makes sure it has.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	// Politely, but without waiting long: the editor is on its way out.
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	_ = c.call(ctx, "shutdown", nil, nil)
	_ = c.notify("exit", nil)

	c.stdin.Close()
	done := make(chan struct{})
	go func() {
		_ = c.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		_ = c.cmd.Process.Kill()
		<-done
	}
	return nil
}

/* --- the wire --- */

// call sends a request and waits for its response, or for ctx to give up.
func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	c.mu.Lock()
	if c.closed && method != "shutdown" {
		c.mu.Unlock()
		return errors.New("the language server has gone")
	}
	c.nextID++
	id := c.nextID
	reply := make(chan message, 1)
	c.pending[id] = reply
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}); err != nil {
		return err
	}

	select {
	case msg := <-reply:
		if msg.Error != nil {
			return fmt.Errorf("%s: %w", method, msg.Error)
		}
		if result == nil || len(msg.Result) == 0 || string(msg.Result) == "null" {
			return nil
		}
		return json.Unmarshal(msg.Result, result)
	case <-ctx.Done():
		return fmt.Errorf("%s: %w", method, ctx.Err())
	}
}

func (c *Client) notify(method string, params any) error {
	return c.write(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

func (c *Client) write(msg map[string]any) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := fmt.Fprintf(c.stdin, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.stdin.Write(body)
	return err
}

// read is the reader loop: responses go to whoever is waiting, requests get a
// bland answer, notifications are dropped.
func (c *Client) read() {
	reader := bufio.NewReader(c.stdout)
	for {
		body, err := readMessage(reader)
		if err != nil {
			c.failPending(err)
			return
		}
		var msg message
		if err := json.Unmarshal(body, &msg); err != nil {
			continue // not something this client can use
		}

		switch {
		case msg.Method != "" && len(msg.ID) > 0:
			c.answer(msg)
		case len(msg.ID) > 0:
			id, err := strconv.Atoi(strings.Trim(string(msg.ID), `"`))
			if err != nil {
				continue
			}
			c.mu.Lock()
			reply := c.pending[id]
			c.mu.Unlock()
			if reply != nil {
				reply <- msg
			}
		}
	}
}

// answer replies to a request from the server. Servers ask about configuration
// and about registering capabilities before they will settle down; an empty
// answer is a true one for this client.
func (c *Client) answer(msg message) {
	var result any
	switch msg.Method {
	case "workspace/configuration":
		var params struct {
			Items []json.RawMessage `json:"items"`
		}
		_ = json.Unmarshal(msg.Params, &params)
		items := make([]any, len(params.Items))
		result = items
	case "client/registerCapability", "client/unregisterCapability",
		"window/workDoneProgress/create", "workspace/semanticTokens/refresh",
		"workspace/codeLens/refresh", "workspace/inlayHint/refresh":
		result = nil
	default:
		_ = c.write(map[string]any{
			"jsonrpc": "2.0",
			"id":      json.RawMessage(msg.ID),
			"error": map[string]any{
				"code":    -32601, // method not found
				"message": "tlua implements only what formatting needs",
			},
		})
		return
	}
	_ = c.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(msg.ID),
		"result":  result,
	})
}

// failPending unblocks everyone waiting when the server goes away.
func (c *Client) failPending(err error) {
	c.mu.Lock()
	pending := c.pending
	c.pending = make(map[int]chan message)
	c.mu.Unlock()
	for _, reply := range pending {
		reply <- message{Error: &responseError{Code: -32000, Message: "the language server stopped: " + err.Error()}}
	}
}

// readMessage reads one header-framed JSON-RPC message.
func readMessage(reader *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // the blank line before the body
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "content-length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("bad Content-Length: %w", err)
			}
		}
	}
	if length < 0 {
		return nil, errors.New("a message arrived without a Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return body, nil
}

// pathToURI turns a file path into the file:// URI the protocol wants.
func pathToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	abs = filepath.ToSlash(abs)
	if !strings.HasPrefix(abs, "/") {
		abs = "/" + abs // a Windows drive letter
	}
	u := url.URL{Scheme: "file", Path: abs}
	return u.String()
}

// CanComplete reports whether the server offers completions.
func (c *Client) CanComplete() bool { return c.canComplete }

// TriggerCharacters are the characters after which the server expects to be
// asked for completions without being prompted. A server that offers
// completions but names none gets Lua's two, which is what asking after "io."
// and "obj:" depends on.
func (c *Client) TriggerCharacters() []string { return c.triggerChars }

// triggerCharacters reads them out of the completion capability.
func triggerCharacters(raw json.RawMessage) []string {
	if !hasCapability(raw) {
		return nil
	}
	var options struct {
		TriggerCharacters []string `json:"triggerCharacters"`
	}
	if err := json.Unmarshal(raw, &options); err == nil && len(options.TriggerCharacters) > 0 {
		return options.TriggerCharacters
	}
	return []string{".", ":"}
}

// CanHover reports whether the server offers hover help.
func (c *Client) CanHover() bool { return c.canHover }

// Complete asks what could go at a position. The text is synced first, so the
// server completes against what the editor has rather than what is on disk.
func (c *Client) Complete(ctx context.Context, path, text string, pos Position) ([]CompletionItem, error) {
	if !c.canComplete {
		return nil, errors.New("the language server does not offer completions")
	}
	if err := c.Sync(path, text); err != nil {
		return nil, err
	}

	var raw json.RawMessage
	err := c.call(ctx, "textDocument/completion", map[string]any{
		"textDocument": map[string]any{"uri": pathToURI(path)},
		"position":     pos,
		"context":      map[string]any{"triggerKind": 1}, // invoked by hand
	}, &raw)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		// A server may answer null when it has nothing to offer, or nothing
		// yet: it is still reading the workspace. That is not an error.
		return nil, nil
	}

	// The answer is either the items or a list wrapping them.
	var items []CompletionItem
	if err := json.Unmarshal(raw, &items); err == nil {
		return items, nil
	}
	var list completionList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("completion: %w", err)
	}
	return list.Items, nil
}

// CanSignature reports whether the server offers signature help.
func (c *Client) CanSignature() bool { return c.canSignature }

// SignatureTriggerCharacters are the characters after which the server expects
// to be asked what a call takes; a server that names none gets the two that
// matter, which open a call and separate its arguments.
func (c *Client) SignatureTriggerCharacters() []string { return c.signatureChars }

// signatureCharacters reads them out of the signature help capability.
func signatureCharacters(raw json.RawMessage) []string {
	if !hasCapability(raw) {
		return nil
	}
	var options struct {
		TriggerCharacters   []string `json:"triggerCharacters"`
		RetriggerCharacters []string `json:"retriggerCharacters"`
	}
	if err := json.Unmarshal(raw, &options); err == nil {
		both := append(options.TriggerCharacters, options.RetriggerCharacters...)
		if len(both) > 0 {
			return both
		}
	}
	return []string{"(", ","}
}

// Signature asks what the call at a position takes.
func (c *Client) Signature(ctx context.Context, path, text string, pos Position) (*SignatureHelp, error) {
	if !c.canSignature {
		return nil, errors.New("the language server does not offer signature help")
	}
	if err := c.Sync(path, text); err != nil {
		return nil, err
	}

	var help SignatureHelp
	err := c.call(ctx, "textDocument/signatureHelp", map[string]any{
		"textDocument": map[string]any{"uri": pathToURI(path)},
		"position":     pos,
	}, &help)
	if err != nil {
		return nil, err
	}
	if len(help.Signatures) == 0 {
		return nil, nil // nothing being called here
	}
	return &help, nil
}

// Hover asks what is at a position and returns it as plain text.
func (c *Client) Hover(ctx context.Context, path, text string, pos Position) (string, error) {
	if !c.canHover {
		return "", errors.New("the language server does not offer hover help")
	}
	if err := c.Sync(path, text); err != nil {
		return "", err
	}

	var result struct {
		Contents json.RawMessage `json:"contents"`
	}
	err := c.call(ctx, "textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": pathToURI(path)},
		"position":     pos,
	}, &result)
	if err != nil {
		return "", err
	}
	return PlainText(markupText(result.Contents)), nil
}
