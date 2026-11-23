package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	credFree  = advapi.NewProc("CredFree")
	queryMode = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtQueryInformationFile")
)

const consoleInput = "CONIN$"

func credentialName(path string) (*uint16, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("absolute credential identifier required")
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(path))))
	return windows.UTF16PtrFromString("Kurier/MCP/" + hex.EncodeToString(sum[:]))
}

func writeCredential(path string, value storedCredential) error {
	name, err := credentialName(path)
	if err != nil {
		return err
	}
	sample, err := json.Marshal(value)
	if err != nil {
		return err
	}
	defer clear(sample)
	if len(sample) > 2560 {
		return errors.New("credential too large")
	}
	record := credential{kind: credentialGeneric, target: name, size: uint32(len(sample)), blob: &sample[0], persist: credentialLocalMachine}
	ok, _, err := credPut.Call(uintptr(unsafe.Pointer(&record)), 0)
	runtime.KeepAlive(record)
	if ok == 0 {
		return err
	}
	return nil
}
func readCredential(path string) (storedCredential, error) {
	var value storedCredential
	name, err := credentialName(path)
	if err != nil {
		return value, err
	}
	var record *credential
	ok, _, err := credGet.Call(uintptr(unsafe.Pointer(name)), credentialGeneric, 0, uintptr(unsafe.Pointer(&record)))
	if ok == 0 {
		return value, err
	}
	defer credFree.Call(uintptr(unsafe.Pointer(record)))
	if record == nil || record.blob == nil || record.size == 0 || record.size > 2560 {
		return value, errors.New("invalid credential")
	}
	raw := unsafe.Slice(record.blob, record.size)
	defer clear(raw)
	if strict(raw, &value) != nil || value.Token == "" {
		return storedCredential{}, errors.New("invalid credential")
	}
	return value, nil
}

type windowsPipe struct {
	*os.File
	deadline time.Time
}

func (p *windowsPipe) SetWriteDeadline(d time.Time) error { p.deadline = d; return nil }
func (p *windowsPipe) Write(frame []byte) (int, error) {
	// Anonymous pipes can hold less than the discovery response. PIPE_NOWAIT
	// lets us recheck the release deadline as the reader drains each chunk.
	handle := windows.Handle(p.Fd())
	written := 0
	for written < len(frame) {
		if !time.Now().Before(p.deadline) {
			return written, os.ErrDeadlineExceeded
		}
		end := written + min(4096, len(frame)-written)
		var n uint32
		if err := windows.WriteFile(handle, frame[written:end], &n, nil); err != nil {
			return written, err
		}
		written += int(n)
		if n == 0 {
			time.Sleep(time.Millisecond)
		}
	}
	return written, nil
}

func deadlineOutput(output *os.File) (deadlinePipe, error) {
	// Duplicate stdout so stream cleanup has its own handle.
	var duplicate windows.Handle
	process := windows.CurrentProcess()
	if err := windows.DuplicateHandle(process, windows.Handle(output.Fd()), process, &duplicate, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(duplicate), "mcp-stdout")
	pipe, err := configurePipe(file)
	if err != nil {
		file.Close()
	}
	return pipe, err
}
func configurePipe(file *os.File) (deadlinePipe, error) {
	handle := windows.Handle(file.Fd())
	kind, err := windows.GetFileType(handle)
	if err != nil || kind != windows.FILE_TYPE_PIPE {
		return nil, errors.New("stdout must be a pipe")
	}
	if file.SetWriteDeadline(time.Now().Add(time.Second)) == nil {
		return file, nil
	}
	// A raw synchronous WriteFile is valid only on a synchronous handle.
	var mode uint32
	var status [2]uintptr
	code, _, _ := queryMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&status)), uintptr(unsafe.Pointer(&mode)), unsafe.Sizeof(mode), fileModeInformation)
	if code != 0 || mode&(windows.FILE_SYNCHRONOUS_IO_ALERT|windows.FILE_SYNCHRONOUS_IO_NONALERT) == 0 {
		return nil, errors.New("cannot establish synchronous pipe mode")
	}
	wait := uint32(pipeNowait)
	if err := windows.SetNamedPipeHandleState(handle, &wait, nil, nil); err != nil {
		return nil, err
	}
	return &windowsPipe{File: file}, nil
}
