package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
)

type registration struct {
	Name      string
	Enabled   bool
	Transport struct {
		Type    string
		Command string
		Args    []string
		Env     map[string]string
		EnvVars []string `json:"env_vars"`
		Cwd     *string
	}
}

// npm installs a .cmd shim on Windows. Run its JavaScript entry point
// directly so launcher arguments never pass through cmd.exe.
func codexCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	path, err := exec.LookPath("codex")
	if err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" && strings.EqualFold(filepath.Ext(path), ".cmd") {
		entry := filepath.Join(filepath.Dir(path), "node_modules", "@openai", "codex", "bin", "codex.js")
		if _, err := os.Stat(entry); err != nil {
			return nil, err
		}
		return exec.CommandContext(ctx, "node", append([]string{entry}, args...)...), nil
	}
	return exec.CommandContext(ctx, path, args...), nil
}

// Inspect launcher metadata without printing other servers' configuration.
func setupCodex(origin, path string) error {
	if !validOrigin(origin) {
		return errors.New("trusted HTTPS origin required")
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil || strings.IndexFunc(binary, unicode.IsControl) >= 0 || runtime.GOOS == "windows" && strings.ContainsAny(binary, "‘’‚‛") {
		return errors.New("invalid executable path")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd, err := codexCommand(ctx, "mcp", "list", "--json")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Install Codex CLI on this computer, then run setup again.")
		return err
	}
	raw, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Install Codex CLI and confirm that codex mcp list works in this terminal.")
		return err
	}
	defer clear(raw)
	var entries []registration
	if err = json.Unmarshal(raw, &entries); err != nil || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		return errors.New("cannot read registrations")
	}
	registered := false
	for _, entry := range entries {
		if entry.Name != "kurier" {
			continue
		}
		if !entry.Enabled || len(entry.Transport.Env) != 0 || len(entry.Transport.EnvVars) != 0 || entry.Transport.Cwd != nil || entry.Transport.Type != "stdio" || entry.Transport.Command != binary || len(entry.Transport.Args) != 1 || entry.Transport.Args[0] != "serve" {
			fmt.Fprintln(os.Stderr, "A different or disabled Kurier entry already exists. Review it in Codex MCP settings before continuing.")
			return errors.New("conflicting registration")
		}
		registered = true
	}
	c, err := readCredential(path)
	paired := false
	if err == nil && c.APIOrigin == origin {
		if b, err := newBridge(c); err == nil {
			env, _, err := b.invoke(context.Background(), "list_projects", json.RawMessage("{}"))
			paired = err == nil && env.Error == nil
		}
	}
	if !paired {
		fmt.Fprintln(os.Stderr, "In Kurier Settings, create a read credential for your projects. Paste it at the hidden prompt to save it for this user.")
		if err = run([]string{"pair", "--api-origin", origin, "--persist"}); err != nil {
			return err
		}
	}
	if !registered {
		quoted := "'" + strings.ReplaceAll(binary, "'", "'\"'\"'") + "'"
		if runtime.GOOS == "windows" {
			quoted = "'" + strings.ReplaceAll(binary, "'", "''") + "'"
		}
		fmt.Fprintln(os.Stderr, "Project access verified. To register this bridge, run this command in your terminal (PowerShell on Windows). If another setup has added kurier, rerun setup first.")
		fmt.Fprintln(os.Stderr, "codex mcp add kurier -- "+quoted+" serve")
	}
	fmt.Fprintln(os.Stderr, "Start a fresh Codex session and ask: Which Kurier projects can I see?")
	return nil
}
