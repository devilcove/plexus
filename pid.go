package plexus

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

var (
	ErrInvalidPID     = errors.New("invalid pid")          // ErrInvalidPID indicates an invalid pid
	ErrPIDNotRunning  = errors.New("pid not running")      // ErrPIDNotRunning indicatates the pid in question is not running
	ErrProcessRunning = errors.New("proces still running") // ErrProcessRunnig indicates that process is still running
)

func IsAlive(pid int) bool {
	_, err := os.Stat(filepath.Join("/proc/", strconv.Itoa(pid)))
	log.Println(pid, err)
	return err == nil
}

func ReadPID(file string) (int, error) {
	contents, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(string(bytes.TrimSpace(contents)))
	if err != nil {
		return 0, err
	}
	if pid < 0 {
		return pid, ErrInvalidPID
	}
	return pid, nil
}

func WritePID(file string, pid int) error {
	log.Println("write pid", file, pid)
	if pid < 1 {
		return ErrInvalidPID
	}
	old, err := ReadPID(file)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if old != 0 {
		return ErrProcessRunning
	}
	return os.WriteFile(file, []byte(strconv.Itoa(pid)), 0644)
}
