package serial

/*
#include <sys/ioctl.h>
#include <sys/socket.h>
#include <linux/gsmmux.h>
#include <termios.h>
#include <unistd.h>

#define N_GSM0710	21
#define DEFAULT_SPEED	B115200
#define SERIAL_PORT	/dev/ttyS0

int start_mux(int fd, int initiator) {
	int rc;
	int ldisc = N_GSM0710;
	struct gsm_config c;
	struct termios configuration;

	write(fd, "AT+CMUX=0\r", 10);
	sleep(2);

	rc = ioctl(fd, TIOCSETD, &ldisc);
	if (rc != 0) {
		return rc;
	}

	rc = ioctl(fd, GSMIOC_GETCONF, &c);
	if (rc != 0) {
		return rc;
	}

	c.initiator = initiator;
	c.encapsulation = 0;
	c.mru = 127;
	c.mtu = 127;

	rc = ioctl(fd, GSMIOC_SETCONF, &c);
	if (rc != 0) {
		return rc;
	}

	return 0;
}
*/
import "C"

import (
	"fmt"
	"time"
)

type GsmMuxPort Port

func NewGsmMuxPort(name string, baud int) (*GsmMuxPort, error) {
	port, err := openPort(name, baud, time.Second)
	return (*GsmMuxPort)(port), err

}

func (mux *GsmMuxPort) Start(num, initiator int) error {
	fd := (*Port)(mux).f.Fd()
	rc := C.start_mux(C.int(fd), C.int(initiator))
	if rc != 0 {
		return fmt.Errorf("start mux return %d", rc)
	}

	return nil
}

func (mux *GsmMuxPort) Close() {
	(*Port)(mux).Close()
}
