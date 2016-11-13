package eshellexpect

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/xiqingping/golibs/expect"
)

type EShellExpect struct {
	exp    *expect.Expect
	prompt string
}

func NewEShellExpect(prompt string, rwc io.ReadWriteCloser) *EShellExpect {
	r := &EShellExpect{
		exp:    expect.NewExpect(rwc),
		prompt: `\[(-?\d*)\]` + prompt,
	}
	r.exp.SetTimeout(time.Second)
	return r
}

func (shell *EShellExpect) SetTimeout(d time.Duration) {
	shell.exp.SetTimeout(d)
}

func (shell *EShellExpect) Close() {
	shell.exp.ReadWriter.(io.Closer).Close()
}

func (shell *EShellExpect) ExecCommand(cmd string, exp string) ([]string, error) {
	shell.exp.FlushInput()
	shell.exp.SendLn(cmd)
	err := shell.exp.Expect(exp + shell.prompt)
	if err != nil {
		return nil, fmt.Errorf("Wait command reply %s", err)
	}

	rcode, _ := strconv.Atoi(shell.exp.Groups[len(shell.exp.Groups)-1])
	r := shell.exp.Groups[:len(shell.exp.Groups)-1]

	if rcode != 0 {
		return r, fmt.Errorf("Command exit code %v", rcode)
	}

	return r, nil
}
