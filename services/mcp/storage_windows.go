//go:build windows

package main

import (
	"errors"
	"os"
)

type storedCredential struct {
	APIOrigin string `json:"apiOrigin"`
	Token     string `json:"token"`
}

func readCredential(string) (storedCredential, error) {
	return storedCredential{}, errors.New("private Windows credential storage unsupported")
}
func writeCredential(string, storedCredential) error {
	return errors.New("private Windows credential storage unsupported")
}

func deadlineOutput(*os.File) (*os.File, error) {
	return nil, errors.New("Windows protocol output unsupported")
}
