// Package acpfixture runs a scripted ACP agent over a byte stream.
//
// It exists so the agent panel can be verified offline: the viewer spawns it like any
// other agent (`agent.command: ["sdt", "agent", "fixture"]`), and it answers initialize,
// session/new and session/prompt with a canned conversation — a long markdown answer or a
// provider refusal. No socket, no test harness, no network: a browser cell that used to
// need a paid turn runs against this.
package acpfixture

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// The protocol tags these messages carry.
const (
	jsonrpcField   = "jsonrpc"
	jsonrpcVersion = "2.0"
	sessionIDField = "sessionId"
	modeField      = "mode"
	typeField      = "type"
)

// Repeat is how many times the answer script repeats its text. The default 1 fits a tall
// panel; the follow-bottom cell sets it so the reply overflows and the scroll can be watched.
var Repeat = 1

// Script is the conversation the fixture plays.
type Script string

const (
	// ScriptAnswer streams a long markdown answer: enough prose to overflow the thread,
	// with a code fence, a table, a callout and a mermaid diagram.
	ScriptAnswer Script = "answer"
	// ScriptFail refuses the turn the way a provider out of quota does.
	ScriptFail Script = "fail"
	// ScriptAsk asks a question (elicitation) and then answers, so the panel's question
	// card can be driven without an agent that has one.
	ScriptAsk Script = "ask"
)

// LongAnswer is the text ScriptAnswer streams. It is deliberately longer than the panel's
// viewport at any width, so follow-the-bottom can be watched rather than inferred.
const LongAnswer = `## Fixture answer

This answer is streamed by the acceptance fixture, not by a provider. It exists so the
panel's thread can be driven offline: it is long enough to overflow, it carries every
block the panel renders, and it is the same text on every run.

The first paragraph repeats itself a few times so the follow-bottom rule has something to
follow. A reader who scrolls up should stop following; a reader at the bottom should stay
there as each chunk arrives.

A list, for the spacing:

- one item, short;
- a second item with **bold** and ` + "`inline code`" + `;
- a third with a [link](https://example.com).

` + "```go" + `
func meaning(x int) int {
	if x == 0 {
		return 0
	}
	return meaning(x-1) + 1
}
` + "```" + `

| Block | Rendered by | Checked |
|---|---|---|
| code fence | the panel's markdown | yes |
| table | the panel's markdown | yes |
| callout | the panel's markdown | yes |

> [!NOTE]
> The fixture exists for the cells a paid turn could not run: the answer overflows, so
> follow-bottom is observable, and the blocks are the ones the stylesheet styles.

` + "```mermaid" + `
graph LR
  A[fixture] --> B[panel]
  B --> C[answer]
` + "```" + `

Closing paragraph: the stream ends here, with a stop reason of end_turn.
`

// Serve reads JSON-RPC requests, one per line, and writes the scripted answers until the
// reader ends. Writes are newline-delimited JSON, the transport the bridge speaks.
func Serve(r io.Reader, w io.Writer, script Script) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	// Unbuffered on purpose: the client waits for each line as it is produced, and a
	// buffered writer that flushes at exit would deadlock the handshake.
	var sessionID string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			return fmt.Errorf("acpfixture: request: %w", err)
		}
		if request.Method == "" {
			// A response to the fixture's own request (the elicitation): the turn does not
			// depend on it, and answering it with "method not found" would be noise.
			continue
		}
		switch request.Method {
		case "initialize":
			if err := reply(w, request.ID, map[string]any{
				"protocolVersion":   1,
				"agentCapabilities": map[string]any{},
				"authMethods":       []any{},
			}); err != nil {
				return err
			}
		case "session/new":
			sessionID = "fixture-session"
			if err := reply(w, request.ID, map[string]any{sessionIDField: sessionID}); err != nil {
				return err
			}
		case "session/set_config_option":
			if err := reply(w, request.ID, map[string]any{"configOptions": []any{}}); err != nil {
				return err
			}
		case "session/cancel":
			if err := reply(w, request.ID, nil); err != nil {
				return err
			}
		case "session/prompt":
			if err := turn(w, request.ID, sessionID, script); err != nil {
				return err
			}
		default:
			if err := replyError(w, request.ID, -32601, "method not found: "+request.Method); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// turn plays one prompt: the refusal, or the streamed answer and its ending.
func turn(w io.Writer, id json.RawMessage, sessionID string, script Script) error {
	if script == ScriptFail {
		return replyError(w, id, -32000, "provider.free-limit: free usage limit reached (fixture)")
	}
	if script == ScriptAsk {
		// An agent-to-client request: the bridge answers it with the reader's card. Its
		// response arrives as a line this loop ignores, which is harmless: the turn does
		// not depend on it.
		if err := write(w, map[string]any{
			jsonrpcField: jsonrpcVersion,
			"id":         json.RawMessage("9001"),
			"method":     "elicitation/create",
			"params": map[string]any{
				"requestId":    "fixture-question",
				sessionIDField: sessionID,
				"mode":         "form",
				"message":      "Which fixture mode should run?",
				"schema": map[string]any{
					typeField: "object",
					"properties": map[string]any{
						modeField: map[string]any{typeField: "string", "title": "Mode", "enum": []string{"answer", "fail"}},
					},
					"required": []string{"mode"},
				},
			},
		}); err != nil {
			return err
		}
	}
	for _, chunk := range chunks(strings.Repeat(LongAnswer, Repeat)) {
		if err := notify(w, "session/update", map[string]any{
			sessionIDField: sessionID,
			"update": map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       map[string]any{typeField: "text", "text": chunk},
			},
		}); err != nil {
			return err
		}
	}
	return reply(w, id, map[string]any{"stopReason": "end_turn"})
}

// chunks splits the answer into lines, kept with their newlines so the joined text is the
// original: a reader sees the answer arrive, not appear.
func chunks(text string) []string {
	lines := strings.SplitAfter(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func reply(w io.Writer, id json.RawMessage, result any) error {
	return write(w, map[string]any{jsonrpcField: jsonrpcVersion, "id": json.RawMessage(id), "result": result})
}

func replyError(w io.Writer, id json.RawMessage, code int, message string) error {
	return write(w, map[string]any{
		jsonrpcField: jsonrpcVersion,
		"id":         json.RawMessage(id),
		"error":      map[string]any{"code": code, "message": message},
	})
}

func notify(w io.Writer, method string, params any) error {
	return write(w, map[string]any{jsonrpcField: jsonrpcVersion, "method": method, "params": params})
}

func write(w io.Writer, payload map[string]any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("acpfixture: encode: %w", err)
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("acpfixture: write: %w", err)
	}
	return nil
}
