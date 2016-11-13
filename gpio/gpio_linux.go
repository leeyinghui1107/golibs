package gpio

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const (
	gpiobase     = "/sys/class/gpio"
	exportPath   = "/sys/class/gpio/export"
	unexportPath = "/sys/class/gpio/unexport"
)

var (
	bytesSet   = []byte{'1'}
	bytesClear = []byte{'0'}
)

// pin represents a GPIO pin.
type pin struct {
	number        int      // the pin number
	numberAsBytes []byte   // the pin number as a byte array to avoid converting each time
	modePath      string   // the path to the /direction FD to avoid string joining each time
	edgePath      string   // the path to the /edge FD to avoid string joining each time
	valueFile     *os.File // the file handle for the value file
	initial       bool     // is this the initial epoll trigger?
	err           error    //the last error
}

// OpenPin exports the pin, creating the virtual files necessary for interacting with the pin.
// It also sets the mode for the pin, making it ready for use.
func OpenLinuxPin(n int, mode Mode) (Pin, error) {
	// export this pin to create the virtual files on the system
	pinBase, err := expose(n)
	if err != nil {
		return nil, err
	}
	value, err := os.OpenFile(filepath.Join(pinBase, "value"), os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	p := &pin{
		number:    n,
		modePath:  filepath.Join(pinBase, "direction"),
		edgePath:  filepath.Join(pinBase, "edge"),
		valueFile: value,
		initial:   true,
	}
	if err := p.setMode(mode); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

// write opens a file for writing, writes the byte slice to it and closes the
// file.
func write(buf []byte, path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err := file.Write(buf); err != nil {
		return err
	}
	return file.Close()
}

// read opens a file for reading, reads the bytes slice from it and closes the file.
func read(path string) ([]byte, error) {
	return ioutil.ReadFile(path)
}

// Close destroys the virtual files on the filesystem, unexporting the pin.
func (p *pin) Close() {
	writeFile(filepath.Join(gpiobase, "unexport"), "%d", p.number)
}

// Mode retrieves the current mode of the pin.
func (p *pin) Mode() Mode {
	var mode string
	mode, p.err = readFile(p.modePath)
	return Mode(mode)
}

// SetMode sets the mode of the pin.
func (p *pin) SetMode(mode Mode) {
	p.err = p.setMode(mode)
}

func (p *pin) GetMode() Mode {
	currentMode, _ := read(p.modePath)
	currentMode_ := strings.Trim(string(currentMode), "\n ")
	return Mode(currentMode_)
}

func (p *pin) setMode(mode Mode) error {
	if p.GetMode() != mode {
		return write([]byte(mode), p.modePath)
	} else {
		return nil
	}
}

// Set sets the pin level.
func (p *pin) Set(is_high bool) {
	if is_high {
		_, p.err = p.valueFile.Write(bytesSet)
	} else {
		_, p.err = p.valueFile.Write(bytesClear)
	}
}

// Get retrieves the current pin level.
func (p *pin) Get() bool {
	bytes := make([]byte, 1)
	_, p.err = p.valueFile.ReadAt(bytes, 0)
	return bytes[0] == bytesSet[0]
}

// Err returns the last error encountered.
func (p *pin) Err() error {
	return p.err
}

func expose(pin int) (string, error) {
	pinBase := filepath.Join(gpiobase, fmt.Sprintf("gpio%d", pin))
	var err error
	if _, statErr := os.Stat(pinBase); os.IsNotExist(statErr) {
		err = writeFile(filepath.Join(gpiobase, "export"), "%d", pin)
	}
	return pinBase, err
}

func writeFile(path string, format string, args ...interface{}) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0777)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, format, args...)
	return err
}

func readFile(path string) (string, error) {
	buf, err := ioutil.ReadFile(path)
	return strings.TrimSpace(string(buf)), err
}
