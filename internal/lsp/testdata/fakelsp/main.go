// Command fakelsp is a language server that exists to be tested against. It
// speaks just enough of the protocol to be initialized and to format a
// document, and its idea of formatting is to trim trailing whitespace and turn
// leading tabs into two spaces.
//
// Its behaviour is steered by environment variables, so a test can ask for a
// server that cannot format (FAKELSP_NOFORMAT), one that never answers a
// formatting request (FAKELSP_HANG), or one that fails it (FAKELSP_ERROR).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf16"
)

var documents = map[string]string{}

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
			formatting := os.Getenv("FAKELSP_NOFORMAT") == ""
			reply(out, msg.ID, map[string]any{
				"capabilities": map[string]any{"documentFormattingProvider": formatting},
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

		case "shutdown":
			reply(out, msg.ID, nil)

		case "exit":
			return
		}
	}
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
