//go:build darwin

package session

import (
	"os"

	"golang.org/x/sys/unix"
)

func getTermios() (*unix.Termios, error) {
	return unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TIOCGETA)
}

func setTermios(t *unix.Termios) error {
	return unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETAF, t)
}

func restoreTermios(t *unix.Termios) error {
	return unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETAF, t)
}
