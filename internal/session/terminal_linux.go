//go:build linux

package session

import (
	"os"

	"golang.org/x/sys/unix"
)

func getTermios() (*unix.Termios, error) {
	return unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
}

func setTermios(t *unix.Termios) error {
	return unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TCSETSF, t)
}

func restoreTermios(t *unix.Termios) error {
	return unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TCSETSF, t)
}
