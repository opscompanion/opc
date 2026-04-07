//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package tui

import (
	"errors"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var waitForSecretInput = waitForSecretInputReady

func readSecretFileWithCancel(inFile *os.File) (string, error) {
	fd := int(inFile.Fd())
	interruptCh := make(chan os.Signal, 1)
	notifyInterrupt(interruptCh)
	defer stopInterruptNotify(interruptCh)

	secret := make([]byte, 0, 32)
	var buf [1]byte

	for {
		select {
		case <-interruptCh:
			return "", ErrSecretInputCancelled
		default:
		}

		ready, err := waitForSecretInput(fd, 100*time.Millisecond)
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				return "", ErrSecretInputCancelled
			}
			return "", err
		}
		if !ready {
			continue
		}

		n, err := unix.Read(fd, buf[:])
		if n > 0 {
			switch buf[0] {
			case '\r', '\n':
				return string(secret), nil
			case '\b', 127:
				if len(secret) > 0 {
					secret = secret[:len(secret)-1]
				}
			case 3, 4:
				return "", ErrSecretInputCancelled
			default:
				secret = append(secret, buf[0])
			}
			continue
		}
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				return "", ErrSecretInputCancelled
			}
			if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
				continue
			}
			if errors.Is(err, os.ErrClosed) {
				return "", ErrSecretInputCancelled
			}
			return "", err
		}
		if len(secret) > 0 {
			return string(secret), nil
		}
		return "", ErrSecretInputCancelled
	}
}

func waitForSecretInputReady(fd int, timeout time.Duration) (bool, error) {
	pollTimeout := int(timeout / time.Millisecond)
	if pollTimeout < 0 {
		pollTimeout = -1
	}

	n, err := unix.Poll([]unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}, pollTimeout)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
