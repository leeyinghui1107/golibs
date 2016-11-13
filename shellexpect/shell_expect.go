package shellexpect

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/xiqingping/golibs/expect"
)

type ShellExpect struct {
	exp    *expect.Expect
	prompt string
}

func NewShellExpect(prompt string, rwc io.ReadWriteCloser) *ShellExpect {
	r := &ShellExpect{
		exp:    expect.NewExpect(rwc),
		prompt: `\[(-?\d*)\]` + prompt,
	}
	r.exp.SetTimeout(time.Second)
	return r
}

func (shell *ShellExpect) SetTimeout(d time.Duration) {
	shell.exp.SetTimeout(d)
}

func (shell *ShellExpect) Close() {
	shell.exp.ReadWriter.(io.Closer).Close()
}

func (shell *ShellExpect) SendCommand(cmd string) {
	shell.exp.FlushInput()
	shell.exp.SendLn(cmd)
}

func (shell *ShellExpect) ExecCommand(cmd string, regexp string) ([]string, error) {
	shell.exp.FlushInput()
	shell.exp.SendLn(cmd)
	if err := shell.exp.Expect(regexp + shell.prompt); err != nil {
		return nil, fmt.Errorf("Wait command reply %s", err)
	}
	r := shell.exp.Groups

	shell.exp.SendLn(`echo $?`)
	if err := shell.exp.Expect(`\(-?{1,3}\)[\r\n]{1,2}` + shell.prompt); err != nil {
		return nil, fmt.Errorf("Check command result %s", err)
	}

	rcode, _ := strconv.Atoi(shell.exp.Groups[1])
	if rcode != 0 {
		return r, fmt.Errorf("Command exit code %v", rcode)
	}

	return r, nil
}
