package main

import (
	"path/filepath"
	"testing"
	"unsafe"
)

func TestWindowsCredentialRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credential.json")
	name, err := credentialName(path)
	if err != nil {
		t.Fatal(err)
	}
	value := storedCredential{"https://api.example", "synthetic-test-token"}
	if err := writeCredential(path, value); err != nil {
		t.Fatal(err)
	}
	defer func() {
		ok, _, err := advapi.NewProc("CredDeleteW").Call(uintptr(unsafe.Pointer(name)), credentialGeneric, 0)
		if ok == 0 {
			t.Error(err)
		}
	}()
	got, err := readCredential(path)
	if err != nil || got != value {
		t.Fatal("Credential Manager round trip failed")
	}
}
