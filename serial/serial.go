package serial

import (
	"fmt"
	"io"
	"time"
)

const EOL_DROP_DEFULT byte = '\r'
const EOL_DEFAULT byte = '\n'
const LN_DEFAULT string = "\r"

type SerialPort struct {
	mPort     io.ReadWriteCloser
	mName     string
	mBaud     int
	mEol      byte
	mLn       string
	mBuffer   []byte
	mLineChan chan []byte
}

func NewSerialPort(name string, baud int) (*SerialPort, error) {
	s := SerialPort{
		mPort:     nil,
		mEol:      EOL_DEFAULT,
		mLn:       LN_DEFAULT,
		mLineChan: nil,
	}

	if err := s.open(name, baud, time.Second); err != nil {
		return nil, err
	}

	return &s, nil
}

func (s *SerialPort) StartRecv() {
	s.mLineChan = make(chan []byte)
	go s.readThread()

}

func (s *SerialPort) readThread() {
	b := make([]byte, 1)
	s.mBuffer = []byte{}
	for {
		n, err := s.mPort.Read(b)
		if err != nil {
			break
		}

		if n != 1 {
			continue
		}
		if b[0] == s.mEol {
			s.mLineChan <- s.mBuffer
			s.mBuffer = []byte{}
			continue
		}
		s.mBuffer = append(s.mBuffer, b[0])
	}
}

func (s *SerialPort) SetLn(ln string) {
	s.mLn = ln
}

func (s *SerialPort) SetEOL(eol byte) {
	s.mEol = eol
}

func (s *SerialPort) open(name string, baud int, timeout ...time.Duration) error {
	var readTimeout time.Duration

	if len(timeout) > 0 {
		readTimeout = timeout[0]
	}

	port, err := openPort(name, baud, readTimeout)
	if err != nil {
		return fmt.Errorf("Unable to open port \"%s\" - %s", name, err)
	}

	s.mName = name
	s.mBaud = baud
	s.mPort = port
	return nil
}

func (sp *SerialPort) ReadLine() (string, error) {
	r, ok := <-sp.mLineChan
	if ok {
		return string(r), nil
	}
	return "", fmt.Errorf("Serial port closed.")
}

func (sp *SerialPort) Close() error {
	var err error

	if sp.mPort != nil {
		err = sp.mPort.Close()
	}

	if sp.mLineChan != nil {
		close(sp.mLineChan)
	}
	return err
}

func (sp *SerialPort) Write(data []byte) (int, error) {
	if nil == sp.mPort {
		return 0, fmt.Errorf("Serial port is not open")
	}
	return sp.mPort.Write(data)
}

func (sp *SerialPort) Read(b []byte) (int, error) {
	if nil == sp.mPort {
		return 0, fmt.Errorf("Serial port is not open")
	}

	return sp.mPort.Read(b)
}

func posixTimeoutValues(readTimeout time.Duration) (vmin uint8, vtime uint8) {
	const MAXUINT8 = 1<<8 - 1 // 255
	// set blocking / non-blocking read
	var minBytesToRead uint8 = 1
	var readTimeoutInDeci int64
	if readTimeout > 0 {
		// EOF on zero read
		minBytesToRead = 0
		// convert timeout to deciseconds as expected by VTIME
		readTimeoutInDeci = (readTimeout.Nanoseconds() / 1e6 / 100)
		// capping the timeout
		if readTimeoutInDeci < 1 {
			// min possible timeout 1 Deciseconds (0.1s)
			readTimeoutInDeci = 1
		} else if readTimeoutInDeci > MAXUINT8 {
			// max possible timeout is 255 deciseconds (25.5s)
			readTimeoutInDeci = MAXUINT8
		}
	}
	return minBytesToRead, uint8(readTimeoutInDeci)
}
