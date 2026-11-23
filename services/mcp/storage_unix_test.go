//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCredentialFilesRejectSymlinksModesAndReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "private", "credential.json")
	value := storedCredential{APIOrigin: "https://api.example.test", Token: strings.Repeat("a", 43)}
	if e := writeCredential(path, value); e != nil {
		t.Fatal(e)
	}
	got, e := readCredential(path)
	if e != nil || got != value {
		t.Fatal("private credential roundtrip failed")
	}
	if e = os.Chmod(path, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = readCredential(path); e == nil {
		t.Fatal("public credential accepted")
	}
	_ = os.Chmod(path, 0600)
	link := filepath.Join(dir, "private", "link")
	if e = os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if _, e = readCredential(link); e == nil {
		t.Fatal("symlink followed")
	}
	if e = os.Link(path, filepath.Join(dir, "private", "hardlink")); e != nil {
		t.Fatal(e)
	}
	if _, e = readCredential(path); e == nil {
		t.Fatal("hardlinked capability accepted")
	}
	if e = writeCredential(path, value); e != nil {
		t.Fatal(e)
	}
	if _, e = readCredential(path); e != nil {
		t.Fatal("atomic replacement invalid")
	}
}
func TestCredentialAncestorAndOversizeRefusal(t *testing.T) {
	dir := t.TempDir()
	private := filepath.Join(dir, "private")
	if e := os.Mkdir(private, 0700); e != nil {
		t.Fatal(e)
	}
	alias := filepath.Join(dir, "alias")
	if e := os.Symlink(private, alias); e != nil {
		t.Fatal(e)
	}
	if e := writeCredential(filepath.Join(alias, "credential.json"), storedCredential{}); e == nil {
		t.Fatal("credential ancestor symlink followed")
	}
	path := filepath.Join(private, "credential.json")
	if e := os.WriteFile(path, []byte(strings.Repeat("x", 4097)), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := readCredential(path); e == nil {
		t.Fatal("oversize capability file accepted")
	}
}
