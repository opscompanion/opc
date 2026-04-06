//go:build darwin || freebsd || netbsd || openbsd || dragonfly

package tui

import "golang.org/x/sys/unix"

func setSecretInputMode(fd int) error {
	termios, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return err
	}

	newState := *termios
	newState.Lflag &^= unix.ECHO
	newState.Lflag |= unix.ICANON | unix.ISIG
	newState.Iflag |= unix.ICRNL

	return unix.IoctlSetTermios(fd, unix.TIOCSETA, &newState)
}
