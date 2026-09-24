// Command fakelsp is a language server that exists to be tested against. It
// speaks just enough of the protocol to be initialized and to format a
// document, and its idea of formatting is to trim trailing whitespace and turn
// leading tabs into two spaces.
//
// It also completes and hovers: the completions are a fixed set that covers the
// shapes a client has to handle, and the hover answer is markdown.
//
// Its behaviour is steered by environment variables, so a test can ask for a
// server that cannot format (FAKELSP_NOFORMAT), complete (FAKELSP_NOCOMPLETE)
// or hover (FAKELSP_NOHOVER), one that answers completion with null as a server
// still reading a workspace does (FAKELSP_NULLCOMPLETE), one that never answers
// a formatting request (FAKELSP_HANG), or one that fails it (FAKELSP_ERROR).
// FAKELSP_LOG names a file to record what the server was told, and FAKELSP_SLOW
// a number of milliseconds to think before answering a completion or signature
// request, which is how a test sees whether the editor waits.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

var documents = map[string]string{}

// think waits, as a server under load would.
func think() {
	ms, err := strconv.Atoi(os.Getenv("FAKELSP_SLOW"))
	if err == nil && ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

// record appends a line to the file named by FAKELSP_LOG, when a test asked for
// one, so it can see what the server was told and when.
func record(line string) {
	path := os.Getenv("FAKELSP_LOG")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}

func main() {
	in := bufio.NewReader(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	for {
		body, err := read(in)
		if err != nil {
			return
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(body, &msg); err != nil {
			continue
		}

		switch msg.Method {
		case "initialize":
			capabilities := map[string]any{
				"documentFormattingProvider": os.Getenv("FAKELSP_NOFORMAT") == "",
				"hoverProvider":              os.Getenv("FAKELSP_NOHOVER") == "",
			}
			if os.Getenv("FAKELSP_NOSIGNATURE") == "" {
				capabilities["signatureHelpProvider"] = map[string]any{
					// As broad as a real server's, for the editor to narrow.
					"triggerCharacters":   []string{"(", ",", " "},
					"retriggerCharacters": []string{")"},
				}
			}
			if os.Getenv("FAKELSP_NOCOMPLETE") == "" {
				capabilities["completionProvider"] = map[string]any{
					// As broad as a real server's: the editor is expected to
					// narrow this down rather than pop up on every space.
					"triggerCharacters": []string{".", ":", " ", "(", "=", ",", "-"},
				}
			}
			reply(out, msg.ID, map[string]any{
				"capabilities": capabilities,
				"serverInfo":   map[string]any{"name": "fakelsp", "version": "1.0"},
			})

		case "initialized":
			// Chatter a real server produces, to be read and ignored, and a
			// request, to be answered.
			notify(out, "window/logMessage", map[string]any{"type": 3, "message": "fakelsp ready"})
			request(out, 9001, "workspace/configuration", map[string]any{
				"items": []map[string]any{{"section": "Lua"}},
			})

		case "textDocument/didOpen":
			var params struct {
				TextDocument struct {
					URI  string `json:"uri"`
					Text string `json:"text"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			documents[params.TextDocument.URI] = params.TextDocument.Text
			record("didOpen " + params.TextDocument.URI)

		case "textDocument/didChange":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
				ContentChanges []struct {
					Text string `json:"text"`
				} `json:"contentChanges"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			if len(params.ContentChanges) > 0 {
				documents[params.TextDocument.URI] = params.ContentChanges[0].Text
			}

		case "textDocument/formatting":
			if os.Getenv("FAKELSP_HANG") != "" {
				continue // never answer
			}
			if os.Getenv("FAKELSP_ERROR") != "" {
				replyError(out, msg.ID, "formatting is broken today")
				continue
			}
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			reply(out, msg.ID, edits(documents[params.TextDocument.URI]))

		case "textDocument/completion":
			think()
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
				Position struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"position"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			if os.Getenv("FAKELSP_NULLCOMPLETE") != "" {
				reply(out, msg.ID, nil) // as a server still reading a workspace does
				continue
			}
			reply(out, msg.ID, completions(documents[params.TextDocument.URI],
				params.Position.Line, params.Position.Character))

		case "textDocument/signatureHelp":
			think()
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
				Position struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"position"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			reply(out, msg.ID, signatureHelp(documents[params.TextDocument.URI],
				params.Position.Line, params.Position.Character))

		case "textDocument/hover":
			var params struct {
				Position struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"position"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			reply(out, msg.ID, map[string]any{
				"contents": map[string]any{
					"kind": "markdown",
					"value": fmt.Sprintf("```lua\nfunction print(...)\n```\n\n"+
						"Prints its arguments. **Asked at line %d, character %d.**",
						params.Position.Line, params.Position.Character),
				},
			})

		case "shutdown":
			reply(out, msg.ID, nil)

		case "exit":
			return
		}
	}
}

// completions covers the shapes a client has to deal with: a plain label, an
// insertText that differs from it, a snippet, and an explicit textEdit that
// replaces the word being typed.
func completions(text string, line, character int) []map[string]any {
	word, wordStart := wordAt(text, line, character)

	items := []map[string]any{
		{"label": "print", "kind": 3, "detail": "function print(...)"},
		{"label": "pairs", "kind": 3, "detail": "function pairs(t)",
			"documentation": map[string]any{"kind": "markdown", "value": "Iterates **a table**."}},
		{"label": "table_insert", "kind": 3, "insertText": "table.insert"},
		{"label": "for_loop", "kind": 15, "insertTextFormat": 2,
			"insertText": "for ${1:i} = 1, ${2:n} do\n\t$0\nend"},
		{"label": "replaced_word", "kind": 6, "textEdit": map[string]any{
			"range": map[string]any{
				"start": map[string]any{"line": line, "character": wordStart},
				"end":   map[string]any{"line": line, "character": wordStart + len(utf16.Encode([]rune(word)))},
			},
			"newText": "REPLACED",
		}},
	}
	return items
}

// signatureHelp answers for a call the cursor is inside, and for nothing else.
// The active parameter is the number of commas between the opening bracket and
// the cursor, which is how a real server works it out too.
//
// The first signature labels its parameters with offsets, the second with their
// text, so both shapes a client has to handle are exercised.
func signatureHelp(text string, line, character int) any {
	lines := strings.Split(text, "\n")
	if line < 0 || line >= len(lines) {
		return nil
	}
	runes := []rune(lines[line])
	if character > len(runes) {
		character = len(runes)
	}

	// Walk back to the bracket that opened the call the cursor is in.
	depth, open := 0, -1
	for i := character - 1; i >= 0; i-- {
		switch runes[i] {
		case ')':
			depth++
		case '(':
			if depth == 0 {
				open = i
			} else {
				depth--
			}
		}
		if open >= 0 {
			break
		}
	}
	if open < 0 {
		return nil // not inside a call
	}

	commas := 0
	for _, r := range runes[open+1 : character] {
		if r == ',' {
			commas++
		}
	}

	const label = "string.format(format, ...)"
	start := len("string.format(")
	return map[string]any{
		"activeSignature": 0,
		"activeParameter": commas,
		"signatures": []map[string]any{
			{
				"label": label,
				"parameters": []map[string]any{
					{"label": []int{start, start + len("format")}},
					{"label": []int{start + len("format, "), len(label) - 1}},
				},
				"documentation": map[string]any{"kind": "plaintext", "value": "Formats a string."},
			},
			{
				"label":      "string.format(format)",
				"parameters": []map[string]any{{"label": "format"}},
			},
		},
	}
}

// wordAt reports the word being typed at a position, and where it starts.
func wordAt(text string, line, character int) (string, int) {
	lines := strings.Split(text, "\n")
	if line < 0 || line >= len(lines) {
		return "", character
	}
	runes := []rune(lines[line])
	if character > len(runes) {
		character = len(runes)
	}
	start := character
	for start > 0 && isWordRune(runes[start-1]) {
		start--
	}
	return string(runes[start:character]), start
}

func isWordRune(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// edits returns one edit per line that formatting would change.
func edits(text string) []map[string]any {
	var out []map[string]any
	for i, line := range strings.Split(text, "\n") {
		formatted := strings.TrimRight(line, " \t")
		for strings.HasPrefix(formatted, "\t") {
			formatted = "  " + strings.TrimPrefix(formatted, "\t")
		}
		if formatted == line {
			continue
		}
		out = append(out, map[string]any{
			"range": map[string]any{
				"start": map[string]any{"line": i, "character": 0},
				"end":   map[string]any{"line": i, "character": len(utf16.Encode([]rune(line)))},
			},
			"newText": formatted,
		})
	}
	return out
}

func read(in *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if name, value, ok := strings.Cut(line, ":"); ok &&
			strings.EqualFold(strings.TrimSpace(name), "content-length") {
			if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &length); err != nil {
				return nil, err
			}
		}
	}
	if length < 0 {
		return nil, io.ErrUnexpectedEOF
	}
	body := make([]byte, length)
	_, err := io.ReadFull(in, body)
	return body, err
}

func send(out *bufio.Writer, msg map[string]any) {
	body, err := json.Marshal(msg)
	if err != nil {
		return
	}
	fmt.Fprintf(out, "Content-Length: %d\r\n\r\n", len(body))
	out.Write(body)
	out.Flush()
}

func reply(out *bufio.Writer, id json.RawMessage, result any) {
	send(out, map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func replyError(out *bufio.Writer, id json.RawMessage, message string) {
	send(out, map[string]any{"jsonrpc": "2.0", "id": id,
		"error": map[string]any{"code": -32000, "message": message}})
}

func notify(out *bufio.Writer, method string, params any) {
	send(out, map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func request(out *bufio.Writer, id int, method string, params any) {
	send(out, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
}
