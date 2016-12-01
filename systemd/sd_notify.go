package systemd

import (
	"errors"
	"net"
	"os"
)

type SdNotifier net.UnixConn

var SdNotifyNoSocket = errors.New("No socket")

func NewSdNotifier() (*SdNotifier, error) {
	socketAddr := &net.UnixAddr{
		Name: os.Getenv("NOTIFY_SOCKET"),
		Net:  "unixgram",
	}

	if socketAddr.Name == "" {
		return nil, SdNotifyNoSocket
	}

	conn, err := net.DialUnix(socketAddr.Net, nil, socketAddr)
	if err != nil {
		return nil, err
	}

	return (*SdNotifier)(conn), nil
}

func (notifier *SdNotifier) Close() {
	(*net.UnixConn)(notifier).Close()
}

func (notifier *SdNotifier) Notify(state string) error {
	_, err := (*net.UnixConn)(notifier).Write([]byte(state))
	return err
}
