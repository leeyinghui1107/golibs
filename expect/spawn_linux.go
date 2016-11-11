package expect

// #ifndef CGO_TERMIOS_H
// #define CGO_TERMIOS_H
// #include <termios.h>
// #include <unistd.h>
// typedef struct termios termios;
// #endif
import "C"

import (
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"
)

func open() (pty, tty *os.File, err error) {
	p, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}

	sname, err := ptsname(p)
	if err != nil {
		return nil, nil, err
	}

	err = unlockpt(p)
	if err != nil {
		return nil, nil, err
	}

	t, err := os.OpenFile(sname, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}

	return p, t, nil
}

func ptsname(f *os.File) (string, error) {
	var n C.uint
	err := ioctl(f.Fd(), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n)))
	if err != nil {
		return "", err
	}
	return "/dev/pts/" + strconv.Itoa(int(n)), nil
}

func unlockpt(f *os.File) error {
	var u C.int
	// use TIOCSPTLCK with a zero valued arg to clear the slave pty lock
	return ioctl(f.Fd(), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&u)))
}

func ioctl(fd, cmd, ptr uintptr) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, ptr)
	if e != 0 {
		return e
	}
	return nil
}

func setEcho(f *os.File, on bool) error {
	var termios C.termios
	if err := ioctl(f.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&termios))); err != nil {
		return err
	}
	if on {
		termios.c_lflag |= C.ECHO
	} else {
		termios.c_lflag &^= C.ECHO
	}
	return ioctl(f.Fd(), syscall.TCSETS, uintptr(unsafe.Pointer(&termios)))
}

func start(c *exec.Cmd) (pty *os.File, err error) {
	pty, tty, err := open()
	if err != nil {
		return nil, err
	}
	defer tty.Close()

	err = setEcho(pty, false)
	if err != nil {
		return nil, err
	}

	err = setEcho(tty, false)
	if err != nil {
		return nil, err
	}

	c.Stdout = tty
	c.Stdin = tty
	c.Stderr = tty
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.Setctty = true
	c.SysProcAttr.Setsid = true
	err = c.Start()
	if err != nil {
		pty.Close()
		return nil, err
	}

	return pty, err
}

type SubProcess struct {
	cmd *exec.Cmd
	*os.File
}

func SpawnCommand(name string, arg ...string) (io.ReadWriteCloser, error) {
	cmd := exec.Command(name, arg...)
	f, err := start(cmd)
	if err != nil {
		return nil, err
	}

	return &SubProcess{
		cmd:  cmd,
		File: f,
	}, nil
}

func (p *SubProcess) Close() error {
	if err := p.cmd.Process.Kill(); err != nil {
		return err
	}
	if err := p.cmd.Wait(); err != nil {
		return nil
	}
	return p.File.Close()
}
