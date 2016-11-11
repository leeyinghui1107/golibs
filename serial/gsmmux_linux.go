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

type GsmMuxPort struct {
	Port
	num       int
	initiator int
}

func OpenGsmMuxPort(name string, baud int) (p *GsmMuxPort, err error) {
	port, err := openPort(name, baud, time.Second)
	if err != nil {
		return nil, err
	}

	return &GsmMuxPort{
		Port: *port,
	}, nil
}

/*
func (mux *GsmMuxPort) createDeviceNode() {
	for i := mux.initiator; i < mux.initiator+mux.num; i++ {
		cmds := fmt.Sprintf("mknod /dev/ttyGSM%d c 251 %d", i, i)
		fmt.Println("cmd:", cmds)
		cmd := exec.Command(cmds)
		cmd.Run()
	}
}

func (mux *GsmMuxPort) removeDeivceNode() {
	for i := mux.initiator; i < mux.initiator+mux.num; i++ {
		cmd := exec.Command(fmt.Sprintf("rm /dev/ttyGSM%d", i))
		cmd.Run()
	}
}
*/

func (mux *GsmMuxPort) Start(num, initiator int) error {
	fd := mux.Port.f.Fd()
	rc := C.start_mux(C.int(fd), C.int(initiator))
	if rc != 0 {
		return fmt.Errorf("start mux return %d", rc)
	}

	mux.num = num
	mux.initiator = initiator
	//mux.createDeviceNode()
	return nil
}

func (mux *GsmMuxPort) Close() {
	//mux.removeDeivceNode()
	mux.Port.Close()
}
