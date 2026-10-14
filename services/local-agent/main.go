package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

type storedCredential struct {
	API   string `json:"api"`
	Token string `json:"token"`
}

func main() {
	if e := command(os.Args[1:], os.Stdin, os.Stderr); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func command(args []string, input *os.File, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: kurier-agent pair|run|status|unpair --api https://Kurier-origin")
	}
	operation := args[0]
	if operation != "pair" && operation != "run" && operation != "status" && operation != "unpair" {
		return errors.New("unknown agent command")
	}
	flags := flag.NewFlagSet("kurier-agent", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	api := flags.String("api", "", "explicit HTTPS Kurier API origin")
	persist := flags.Bool("persist", false, "explicitly persist capability credential in a private file (pair only)")
	stdin := flags.Bool("stdin", false, "read token from stdin rather than a hidden terminal prompt")
	if e := flags.Parse(args[1:]); e != nil {
		return errors.New("invalid startup flags")
	}
	if flags.NArg() != 0 || *persist && operation != "pair" {
		return errors.New("invalid startup flags")
	}
	path, e := credentialPath()
	if e != nil {
		return e
	}
	credential := storedCredential{}
	if operation != "pair" && !*stdin {
		credential, e = readCredential(path)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	if *api != "" && credential.API != "" && *api != credential.API {
		return errors.New("stored credential belongs to another API; unpair first")
	}
	if *api != "" {
		credential.API = *api
	}
	reader := bufio.NewReader(input)
	if credential.Token == "" {
		fmt.Fprintln(out, "Enter the browser-issued agent token (never use an argument or environment variable):")
		if *stdin {
			line, e := reader.ReadString('\n')
			if e != nil {
				return errors.New("token input incomplete")
			}
			credential.Token = strings.TrimSpace(line)
		} else {
			if !term.IsTerminal(int(input.Fd())) {
				return errors.New("hidden terminal required; use explicit --stdin for a controlled test")
			}
			raw, e := term.ReadPassword(int(input.Fd()))
			if e != nil {
				return errors.New("token input unavailable")
			}
			credential.Token = string(raw)
			clear(raw)
			fmt.Fprintln(out)
		}
	}
	c, e := newClient(credential.API, credential.Token)
	if e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if operation == "unpair" {
		if e = c.call(ctx, "DELETE", "/api/v1/local-agent/session", nil, nil, 16*1024); e == nil {
			fmt.Fprintln(out, "Server credential revoked.")
		} else {
			fmt.Fprintln(out, "Server revocation unconfirmed; browser Revoke remains required.")
		}
		if e = removeCredential(path); e != nil {
			return e
		}
		return nil
	}
	if e = c.session(ctx); e != nil {
		return errors.New("pairing/session unavailable; refresh browser pairing status")
	}
	safe, _ := json.Marshal(c.agent)
	fmt.Fprintf(out, "Agent scope and expiry: %s\n", safe)
	if operation == "status" {
		return nil
	}
	fmt.Fprintln(out, "This device receives runtime secrets and may affect every path on these services. Confirm exact destinations by typing yes:")
	line, e := reader.ReadString('\n')
	if e != nil || strings.TrimSpace(line) != "yes" {
		return errors.New("local destination consent required")
	}
	if *persist {
		if e = writeCredential(path, credential); e != nil {
			return e
		}
		fmt.Fprintln(out, "Capability credential saved privately. Request inputs and jobs are never saved.")
	}
	if operation == "pair" {
		fmt.Fprintln(out, "Scope confirmed. Start run to poll in the foreground; memory-only tokens must be entered again.")
		return nil
	}
	e = c.run(ctx, func(s string) { fmt.Fprintln(out, s) })
	if errors.Is(e, context.Canceled) {
		return nil
	}
	return e
}
func credentialPath() (string, error) {
	dir, e := os.UserConfigDir()
	if e != nil {
		return "", errors.New("user configuration directory unavailable")
	}
	return filepath.Join(dir, "kurier-agent", "credential.json"), nil
}
