//go:build !windows && !js

package session

import (
	"os"
	"os/signal"
	"time"

	"github.com/mmmorris1975/ssm-session-client/datachannel"
	"golang.org/x/sys/unix"
)

var origTermios *unix.Termios

func initializeTerminal(c datachannel.DataChannel) error {
	installSignalHandlers(c)
	handleTerminalResize(c)
	return configureStdin()
}

func cleanupTerminal() {
	if origTermios != nil {
		_ = restoreTermios(origTermios)
	}
}

func installSignalHandlers(c datachannel.DataChannel) {
	sigCh := make(chan os.Signal, 10)
	signal.Notify(sigCh, unix.SIGWINCH)

	go func() {
		for sig := range sigCh {
			if sig == unix.SIGWINCH {
				_ = updateTermSize(c)
			}
		}
	}()

	// trigger initial size update
	sigCh <- unix.SIGWINCH
}

func handleTerminalResize(c datachannel.DataChannel) {
	go func() {
		for {
			_ = updateTermSize(c)
			time.Sleep(500 * time.Millisecond)
		}
	}()
}

func updateTermSize(c datachannel.DataChannel) error {
	rows, cols, err := getWinSize()
	if err != nil {
		cols = 132
		rows = 45
	}
	return c.SetTerminalSize(rows, cols)
}

func getWinSize() (rows, cols uint32, err error) {
	sz, err := unix.IoctlGetWinsize(int(os.Stdin.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err
	}
	return uint32(sz.Row), uint32(sz.Col), nil
}

func configureStdin() (err error) {
	origTermios, err = getTermios()
	if err != nil {
		return err
	}

	newTermios := *origTermios
	newTermios.Lflag = origTermios.Lflag ^ unix.ICANON ^ unix.ECHO ^ unix.ISIG

	return setTermios(&newTermios)
}
