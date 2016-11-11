package expect

import (
	"io"
	"os/exec"
)

type SubProcess struct {
	cmd *exec.Cmd
	io.WriteCloser
	io.ReadCloser
}

func SpawnCommand(name string, arg ...string) (io.ReadWriteCloser, error) {
	cmd := exec.Command(name, arg...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}

	return &SubProcess{
		cmd:         cmd,
		WriteCloser: stdin,
		ReadCloser:  stdout,
	}, nil
}

func (p *SubProcess) Close() error {
	if err := p.cmd.Process.Kill(); err != nil {
		return err
	}
	if err := p.cmd.Wait(); err != nil {
		return err
	}

	return nil
}
