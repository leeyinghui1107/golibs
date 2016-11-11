package expect

import (
	"errors"
	"io"
	"os"
	"regexp"
	"sync"
	"syscall"
	"time"
)

type Expect struct {
	io.ReadWriter
	timeout time.Duration
	Before  string
	Groups  []string
	newData chan error
	buffer  []byte
	locker  sync.Locker
	endl    []byte
}

func NewExpect(rw io.ReadWriter) *Expect {
	exp := &Expect{
		ReadWriter: rw,
		timeout:    time.Second,
		newData:    make(chan error),
		buffer:     make([]byte, 100),
		locker:     new(sync.RWMutex),
		endl:       []byte("\r"),
	}

	go exp.readThread()
	return exp
}

func (exp *Expect) SetTimeout(timeout time.Duration) {
	exp.timeout = timeout
}

func (exp *Expect) Send(s string) error {
	_, err := exp.Write([]byte(s))
	return err
}

func (exp *Expect) SetEndLine(b []byte) {
	exp.endl = b
}

func (exp *Expect) SendLn(s string) error {
	//fmt.Println("->:", s)
	if _, err := exp.Write([]byte(s)); err != nil {
		return err
	}

	_, err := exp.Write(exp.endl)
	return err
}

func (exp *Expect) FlushInput() {
	exp.locker.Lock()
	exp.buffer = exp.buffer[:0]
	exp.locker.Unlock()
}

func (exp *Expect) checkForMatch(expr *regexp.Regexp) bool {
	exp.locker.Lock()
	matches := expr.FindSubmatchIndex(exp.buffer)
	defer exp.locker.Unlock()
	if matches != nil {
		groupCount := len(matches) / 2
		exp.Groups = make([]string, groupCount)

		for i := 0; i < groupCount; i++ {
			start := matches[2*i]
			end := matches[2*i+1]
			if start >= 0 && end >= 0 {
				exp.Groups[i] = string(exp.buffer[start:end])
			}
		}
		exp.Before = string(exp.buffer[0:matches[0]])
		exp.buffer = exp.buffer[matches[1]:]
		return true
	}

	return false
}

func (exp *Expect) ExpectRegexp(expr *regexp.Regexp) error {
	t := time.After(exp.timeout)
	for {
		if exp.checkForMatch(expr) {
			return nil
		}
		select {
		case err, ok := <-exp.newData:
			if !ok {
				return errors.New("Read error")
			}
			if err != nil {
				return err
			}
		case <-t:
			exp.Before = ""
			exp.Groups = exp.Groups[:0]
			return errors.New("Expect Timeout")
		}
	}
}

func (exp *Expect) readThread() {
	done := false
	buf := make([]byte, 256)
	for !done {
		n, err := exp.Read(buf)
		pathErr, ok := err.(*os.PathError)
		if ok && pathErr.Err == syscall.EIO {
			err = io.EOF
		}

		if err != nil {
			done = true
		} else {
			exp.locker.Lock()
			//fmt.Println("<-:", string(buf[0:n]))
			exp.buffer = append(exp.buffer, buf[0:n]...)
			exp.locker.Unlock()
		}
		select {
		case exp.newData <- err:
		default:
		}
	}
	close(exp.newData)
}

func (exp *Expect) Expect(expr string) error {
	return exp.ExpectRegexp(regexp.MustCompile(expr))
}
