package main

import (
	"io"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func accessFixture(string, string) error { return nil }

func newWriter(output *os.File) (frameWriter, string, error) {
	fd, err := unix.Dup(int(output.Fd()))
	if err != nil {
		return nil, "", err
	}
	unix.CloseOnExec(fd)
	if err := unix.SetNonblock(fd, true); err != nil {
		_ = unix.Close(fd)
		return nil, "", err
	}
	pipe := os.NewFile(uintptr(fd), "fixture-stdout")
	return func(frame []byte, deadline time.Time) error {
		if !time.Now().Before(deadline) {
			return os.ErrDeadlineExceeded
		}
		if err := pipe.SetWriteDeadline(deadline); err != nil {
			return err
		}
		n, err := pipe.Write(frame)
		if err == nil && n != len(frame) {
			return io.ErrShortWrite
		}
		return err
	}, "pollable-pipe", nil
}
