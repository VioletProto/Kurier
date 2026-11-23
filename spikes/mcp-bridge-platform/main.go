// An isolated platform probe. Every response is synthetic.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"
)

const fixtureValue = "synthetic-project"

type frameWriter func([]byte, time.Time) error

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "platform probe:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 2 || !strings.HasPrefix(args[1], "Kurier/PlatformSpike/") {
		return errors.New("expected fixture operation and disposable target")
	}
	action, target := args[0], args[1]
	switch action {
	case "seed", "cleanup", "missing":
		return accessFixture(action, target)
	case "serve", "pressure":
	default:
		return errors.New("unknown fixture operation")
	}

	write, mode, err := newWriter(os.Stdout)
	if err != nil {
		return err
	}
	if action == "pressure" {
		deadline := time.Now().Add(250 * time.Millisecond)
		frame := make([]byte, 256<<10)
		for range 64 {
			if err := write(frame, deadline); err != nil {
				if time.Since(deadline) > time.Second {
					return errors.New("blocked output exceeded its deadline")
				}
				fmt.Fprintln(os.Stderr, "withheld blocked output:", mode)
				return nil
			}
		}
		return errors.New("output never encountered backpressure")
	}
	if err := accessFixture("load", target); err != nil {
		return err
	}
	return serve(os.Stdin, write, mode)
}

func serve(input io.Reader, write frameWriter, mode string) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			return errors.New("invalid request")
		}
		if len(request.ID) == 0 {
			continue
		}
		response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		var result any
		switch request.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": "2025-11-25",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]string{"name": "kurier-platform-probe", "version": "1"},
			}
		case "ping":
			result = map[string]any{}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{
				"name": "list_projects",
				"inputSchema": map[string]any{
					"type": "object",
				},
			}}}
		case "tools/call":
			if request.Params.Name != "list_projects" {
				return errors.New("unexpected tool")
			}
			data := map[string]string{"project": fixtureValue, "platform": runtime.GOOS + "/" + runtime.GOARCH, "writer": mode}
			result = map[string]any{"content": []any{map[string]string{"type": "text", "text": fixtureValue}}, "structuredContent": data}
		default:
			response["error"] = map[string]any{"code": -32601, "message": "Method not found"}
		}
		if result != nil {
			response["result"] = result
		}
		frame, err := json.Marshal(response)
		if err != nil {
			return err
		}
		// A short write ends the stream. No subsequent frame can be appended.
		if err = write(append(frame, '\n'), time.Now().Add(250*time.Millisecond)); err != nil {
			return err
		}
	}
	return scanner.Err()
}
