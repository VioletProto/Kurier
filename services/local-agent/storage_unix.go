//go:build !windows

package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func privateDirectory(path string, create bool) error {
	dir := filepath.Dir(path)
	for ancestor := filepath.Dir(dir); ; ancestor = filepath.Dir(ancestor) {
		info, e := os.Lstat(ancestor)
		if e != nil {
			return errors.New("credential ancestor unavailable")
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0022 != 0 && info.Mode()&os.ModeSticky == 0 {
			return errors.New("credential ancestors must not be symlinks or shared writable directories")
		}
		if filepath.Dir(ancestor) == ancestor {
			break
		}
	}

	if create {
		if e := os.Mkdir(dir, 0700); e != nil && !os.IsExist(e) {
			return errors.New("private credential directory unavailable")
		}
	}
	info, e := os.Lstat(dir)
	if e != nil {
		return e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return errors.New("credential directory must be private 0700 without symlinks")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) {
		return errors.New("credential directory ownership invalid")
	}
	return nil
}
func readCredential(path string) (storedCredential, error) {
	var value storedCredential
	if e := privateDirectory(path, false); e != nil {
		return value, e
	}
	info, e := os.Lstat(path)
	if e != nil {
		return value, e
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return value, errors.New("credential file must be private 0600 without symlinks")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) {
		return value, errors.New("credential file ownership invalid")
	}
	file, e := os.Open(path)
	if e != nil {
		return value, errors.New("credential file unavailable")
	}
	defer file.Close()
	raw, e := io.ReadAll(io.LimitReader(file, 4097))
	defer clear(raw)
	if e != nil || len(raw) > 4096 || json.Unmarshal(raw, &value) != nil {
		return value, errors.New("credential file invalid")
	}
	return value, nil
}
func writeCredential(path string, value storedCredential) error {
	if e := privateDirectory(path, true); e != nil {
		return e
	}
	if info, e := os.Lstat(path); e == nil && !info.Mode().IsRegular() {
		return errors.New("credential destination must not be a symlink")
	}
	raw, e := json.Marshal(value)
	if e != nil {
		return errors.New("credential encoding unavailable")
	}
	defer clear(raw)
	file, e := os.CreateTemp(filepath.Dir(path), ".credential-")
	if e != nil {
		return errors.New("private credential file unavailable")
	}
	name := file.Name()
	defer os.Remove(name)
	if e = file.Chmod(0600); e == nil {
		_, e = file.Write(raw)
	}
	if e == nil {
		e = file.Sync()
	}
	closeErr := file.Close()
	if e != nil || closeErr != nil {
		return errors.New("private credential write unavailable")
	}
	if e = os.Rename(name, path); e != nil {
		return errors.New("private credential replacement unavailable")
	}
	return nil
}
func removeCredential(path string) error {
	if e := privateDirectory(path, false); e != nil {
		if os.IsNotExist(e) {
			return nil
		}
		return e
	}
	info, e := os.Lstat(path)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil || !info.Mode().IsRegular() {
		return errors.New("credential deletion unavailable")
	}
	if e = os.Remove(path); e != nil {
		return errors.New("credential deletion unavailable")
	}
	return nil
}
