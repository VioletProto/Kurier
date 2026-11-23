package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	credentialGeneric      = 1
	credentialLocalMachine = 2
	fileModeInformation    = 16
	pipeNowait             = 1
)

type credential struct {
	flags, kind          uint32
	target, comment      *uint16
	lastWritten          windows.Filetime
	size                 uint32
	blob                 *byte
	persist, attributesN uint32
	attributes           uintptr
	alias, user          *uint16
}

var (
	advapi    = windows.NewLazySystemDLL("advapi32.dll")
	credPut   = advapi.NewProc("CredWriteW")
	credGet   = advapi.NewProc("CredReadW")
	credDrop  = advapi.NewProc("CredDeleteW")
	credFree  = advapi.NewProc("CredFree")
	queryMode = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtQueryInformationFile")
)

func accessFixture(action, target string) error {
	name, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	switch action {
	case "seed":
		sample := []byte(fixtureValue)
		record := credential{kind: credentialGeneric, target: name, size: uint32(len(sample)), blob: &sample[0], persist: credentialLocalMachine}
		ok, _, callErr := credPut.Call(uintptr(unsafe.Pointer(&record)), 0)
		runtime.KeepAlive(sample)
		if ok == 0 {
			return callErr
		}
	case "cleanup":
		ok, _, callErr := credDrop.Call(uintptr(unsafe.Pointer(name)), credentialGeneric, 0)
		if ok == 0 && !errors.Is(callErr, windows.ERROR_NOT_FOUND) {
			return callErr
		}
	default:
		var record *credential
		ok, _, callErr := credGet.Call(uintptr(unsafe.Pointer(name)), credentialGeneric, 0, uintptr(unsafe.Pointer(&record)))
		if ok == 0 {
			if action == "missing" && errors.Is(callErr, windows.ERROR_NOT_FOUND) {
				return nil
			}
			return callErr
		}
		defer credFree.Call(uintptr(unsafe.Pointer(record)))
		if action == "missing" || record == nil || record.blob == nil || record.size != uint32(len(fixtureValue)) ||
			!bytes.Equal(unsafe.Slice(record.blob, record.size), []byte(fixtureValue)) {
			return errors.New("unexpected credential-store result")
		}
	}
	return nil
}

func newWriter(output *os.File) (frameWriter, string, error) {
	handle := windows.Handle(syscallStdout())
	kind, err := windows.GetFileType(handle)
	if err != nil || kind != windows.FILE_TYPE_PIPE {
		return nil, "", errors.New("stdout must be a pipe")
	}
	if err := output.SetWriteDeadline(time.Now().Add(time.Second)); err == nil {
		return func(frame []byte, deadline time.Time) error {
			if !time.Now().Before(deadline) {
				return os.ErrDeadlineExceeded
			}
			if err := output.SetWriteDeadline(deadline); err != nil {
				return err
			}
			n, err := output.Write(frame)
			if err == nil && n != len(frame) {
				return io.ErrShortWrite
			}
			return err
		}, "pollable-pipe", nil
	}

	// A raw synchronous WriteFile is valid only on a synchronous handle.
	var mode uint32
	var status [2]uintptr
	code, _, _ := queryMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&status)), uintptr(unsafe.Pointer(&mode)), unsafe.Sizeof(mode), fileModeInformation)
	if code != 0 || mode&(windows.FILE_SYNCHRONOUS_IO_ALERT|windows.FILE_SYNCHRONOUS_IO_NONALERT) == 0 {
		return nil, "", errors.New("cannot establish synchronous pipe mode")
	}
	wait := uint32(pipeNowait)
	if err := windows.SetNamedPipeHandleState(handle, &wait, nil, nil); err != nil {
		return nil, "", err
	}
	return func(frame []byte, deadline time.Time) error {
		if !time.Now().Before(deadline) {
			return os.ErrDeadlineExceeded
		}
		var written uint32
		if err := windows.WriteFile(handle, frame, &written, nil); err != nil {
			return err
		}
		if int(written) != len(frame) {
			return io.ErrShortWrite
		}
		return nil
	}, "nonblocking-byte-pipe", nil
}

func syscallStdout() windows.Handle {
	handle, _ := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	return handle
}
