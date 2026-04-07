//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !zos

package tui

import "os"

func readSecretFileWithCancel(inFile *os.File) (string, error) {
	return readSecretBytes(inFile)
}
