//go:build !windows

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type storedCredential struct {
	APIOrigin string `json:"apiOrigin"`
	Token     string `json:"token"`
}

func credentialDir(path string, create bool) (int, error) {
	bad := errors.New("unsafe credential storage")
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return -1, bad
	}
	dir := filepath.Dir(path)
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if e != nil {
		return -1, bad
	}
	parts := strings.Split(strings.TrimPrefix(dir, "/"), "/")
	for i, part := range parts {
		if part == "" {
			unix.Close(fd)
			return -1, bad
		}
		next, er := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if er == unix.ENOENT && create {
			if er = unix.Mkdirat(fd, part, 0700); er == nil || er == unix.EEXIST {
				next, er = unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			}
		}
		unix.Close(fd)
		if er != nil {
			return -1, bad
		}
		fd = next
		var st unix.Stat_t
		if unix.Fstat(fd, &st) != nil || st.Mode&0022 != 0 && st.Mode&unix.S_ISVTX == 0 {
			unix.Close(fd)
			return -1, bad
		}
		if i == len(parts)-1 && (st.Uid != uint32(os.Getuid()) || st.Mode&0777 != 0700) {
			unix.Close(fd)
			return -1, bad
		}
	}
	return fd, nil
}
func readCredential(path string) (storedCredential, error) {
	var value storedCredential
	bad := errors.New("credential unavailable; pair in a terminal")
	d, e := credentialDir(path, false)
	if e != nil {
		return value, bad
	}
	defer unix.Close(d)
	fd, e := unix.Openat(d, filepath.Base(path), unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if e != nil {
		return value, bad
	}
	f := os.NewFile(uintptr(fd), "credential")
	defer f.Close()
	var st unix.Stat_t
	if unix.Fstat(fd, &st) != nil || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Mode&0777 != 0600 || st.Uid != uint32(os.Getuid()) || st.Nlink != 1 {
		return value, bad
	}
	raw, e := io.ReadAll(io.LimitReader(f, 4097))
	defer clear(raw)
	if e != nil || len(raw) > 4096 || strict(raw, &value) != nil || value.Token == "" {
		return storedCredential{}, bad
	}
	return value, nil
}
func writeCredential(path string, value storedCredential) error {
	bad := errors.New("credential storage unavailable")
	d, e := credentialDir(path, true)
	if e != nil {
		return e
	}
	defer unix.Close(d)
	var b [16]byte
	if _, e = rand.Read(b[:]); e != nil {
		return bad
	}
	name := ".credential-" + hex.EncodeToString(b[:])
	fd, e := unix.Openat(d, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if e != nil {
		return bad
	}
	defer unix.Unlinkat(d, name, 0)
	f := os.NewFile(uintptr(fd), "credential")
	raw, e := json.Marshal(value)
	if e == nil {
		_, e = f.Write(raw)
	}
	clear(raw)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil || ce != nil {
		return bad
	}
	if e = unix.Renameat(d, name, d, filepath.Base(path)); e != nil {
		return bad
	}
	return unix.Fsync(d)
}

// An inherited stdout pipe may start blocking. Reopen a nonblocking duplicate
// so Go's poller can enforce release deadlines even under client backpressure.
func deadlineOutput(output *os.File) (*os.File, error) {
	fd, e := unix.Dup(int(output.Fd()))
	if e != nil {
		return nil, errors.New("protocol pipe unavailable")
	}
	unix.CloseOnExec(fd)
	if e = unix.SetNonblock(fd, true); e != nil {
		unix.Close(fd)
		return nil, errors.New("protocol pipe unavailable")
	}
	f := os.NewFile(uintptr(fd), "mcp-stdout")
	if e = f.SetWriteDeadline(time.Now().Add(time.Second)); e != nil {
		f.Close()
		return nil, errors.New("protocol output must support deadlines")
	}
	_ = f.SetWriteDeadline(time.Time{})
	return f, nil
}
