//go:build windows

package main

import (
	"errors"
	"os"
)

func readCredential(path string) (storedCredential, error) {
	_, e := os.Stat(path)
	if os.IsNotExist(e) {
		return storedCredential{}, e
	}
	return storedCredential{}, errors.New("Windows persistent credentials not supported")
}
func writeCredential(_ string, _ storedCredential) error {
	return errors.New("Windows persistent credentials not supported")
}
func removeCredential(_ string) error { return nil }
