//go:build windows

package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Run in a disposable console so injecting a synthetic paste cannot interact
// with the operator's terminal or an existing Codex profile.
func TestHiddenWindowsConsole(t *testing.T) {
	if os.Getenv("KURIER_TEST_NEW_CONSOLE") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHiddenWindowsConsole$")
		cmd.SysProcAttr = &windows.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
		cmd.Env = append(os.Environ(), "KURIER_TEST_NEW_CONSOLE=1")
		if raw, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("disposable console check failed: %v %s", err, raw)
		}
		return
	}
	input, err := os.OpenFile(consoleInput, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	h := windows.Handle(input.Fd())
	mode := func() uint32 {
		var flags uint32
		if err := windows.GetConsoleMode(h, &flags); err != nil {
			t.Fatal(err)
		}
		return flags
	}
	initial := mode()
	type event struct {
		Kind, Padding           uint16
		Down                    int32
		Repeat, Key, Scan, Char uint16
		Controls                uint32
	}
	write := windows.NewLazySystemDLL("kernel32.dll").NewProc("WriteConsoleInputW")
	token := strings.Repeat("x", 43)
	for _, line := range []string{token + "\r", "\x03"} {
		console, err := os.OpenFile(consoleInput, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		type result struct {
			raw []byte
			err error
		}
		done := make(chan result, 1)
		go func() { raw, err := readPairingToken(context.Background(), console); done <- result{raw, err} }()
		deadline := time.Now().Add(2 * time.Second)
		for mode()&windows.ENABLE_ECHO_INPUT != 0 {
			if time.Now().After(deadline) {
				t.Fatal("console input echo remained enabled")
			}
			time.Sleep(time.Millisecond)
		}
		events := make([]event, 0, len(line)*2)
		for _, c := range line {
			e := event{Kind: 1, Down: 1, Repeat: 1, Char: uint16(c)}
			events = append(events, e)
			e.Down = 0
			events = append(events, e)
		}
		var count uint32
		ok, _, err := write.Call(uintptr(h), uintptr(unsafe.Pointer(&events[0])), uintptr(len(events)), uintptr(unsafe.Pointer(&count)))
		if ok == 0 || int(count) != len(events) {
			t.Fatal("synthetic console input failed", err)
		}
		select {
		case got := <-done:
			if line == "\x03" {
				if got.err == nil || len(got.raw) != 0 {
					t.Fatal("cancelled paste accepted")
				}
			} else if got.err != nil || string(got.raw) != token {
				t.Fatal("valid paste lost")
			}
			if mode() != initial {
				t.Fatal("console mode was not restored")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("hidden input did not finish")
		}
	}
}
