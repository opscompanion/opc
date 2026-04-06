//go:build !darwin && !freebsd && !netbsd && !openbsd && !dragonfly && !linux && !aix && !solaris && !zos

package tui

import "golang.org/x/term"

func setSecretInputMode(fd int) error {
	_, err := term.MakeRaw(fd)
	return err
}
