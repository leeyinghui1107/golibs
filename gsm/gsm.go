package gsm

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/xlab/at/sms"
	"github.com/xiqingping/golibs/serial"
)

type GsmUnsHandler interface {
	OnNewMessage(msg string)
}

type Gsm struct {
	mPort        *serial.SerialPort
	mMutex       sync.Mutex
	mExit        bool
	mChanAtReply chan string
	mChanSMS     chan sms.Message
	mRecvCMT     bool
}

func NewGsm(name string, baud int) (*Gsm, error) {
	s, err := serial.NewSerialPort(name, baud)
	if nil != err {
		return nil, err
	}

	gsm := Gsm{
		mPort:        s,
		mExit:        false,
		mChanAtReply: make(chan string),
		mChanSMS:     make(chan sms.Message),
	}
	go gsm.recvThread()

	return &gsm, nil
}

func (g *Gsm) Init() error {
	g.mMutex.Lock()
	defer g.mMutex.Unlock()

	if _, err := g.atcmd("AT", "OK", time.Second); err != nil {
		return err
	}
	if _, err := g.atcmd("ATE0", "OK", time.Second); err != nil {
		return err
	}

	if _, err := g.atcmd("AT", "OK", time.Second); err != nil {
		return err
	}
	if _, err := g.atcmd("AT+CNMI=2,2,0,0,0", "OK", time.Second*2); err != nil {
		return err
	}
	if _, err := g.atcmd("AT+CMGF=0", "OK", time.Second); err != nil {
		return err
	}

	return nil
}

func (g *Gsm) handleSMS(s string) {
	b, err := hex.DecodeString(s)
	if nil != err {
		fmt.Println("handleSMS error:" + err.Error())
		return
	}

	var msg sms.Message
	_, err = msg.ReadFrom(b)
	if err != nil {
		fmt.Println("handleSMS error:" + err.Error())
		return
	}

	select {
	case g.mChanSMS <- msg:
	default:
		fmt.Println("Drop SMS[" + string(msg.Address) + "]:" + msg.Text)
	}
}

func (g *Gsm) recvThread() error {
	g.mPort.StartRecv()
	lc := g.mPort.GetLineChan()

	for !g.mExit {
		select {
		case b := <-lc:
			reply := strings.Trim(string(b), "\r")
			if len(reply) < 2 {
				continue
			}
			if g.mRecvCMT {
				g.mRecvCMT = false
				g.handleSMS(reply)
				continue
			}
			if strings.HasPrefix(reply, "+CMT:") {
				g.mRecvCMT = true
				continue
			}

			select {
			case g.mChanAtReply <- reply:
			default:
				fmt.Printf("Drop: " + reply)
			}
		}
	}
	return nil
}

func (g *Gsm) Teardown() error {
	g.mMutex.Lock()
	defer g.mMutex.Unlock()

	g.mExit = true
	close(g.mChanAtReply)
	time.Sleep(time.Millisecond * 20)
	return g.mPort.Close()
}

func (g *Gsm) waitForReply(exp string, timeout time.Duration) (string, error) {
	regExpPatttern := regexp.MustCompile(exp)
	t := time.After(timeout)
	for {
		select {
		case data := <-g.mChanAtReply:
			result := regExpPatttern.FindAllString(data, -1)
			if len(result) > 0 {
				return result[0], nil
			}
			fmt.Println("Drop <- " + data)
		case <-t:
			return "", fmt.Errorf("Timeout expired")
		}
	}
}

func (g *Gsm) atcmd(cmd, echo string, timeout time.Duration) (string, error) {
	if "" != cmd {
		fmt.Printf("GSMAT-> \"%s\"\n", cmd)
		buf := make([]byte, len(cmd)+1)
		copy(buf, cmd)
		buf[len(cmd)] = '\r'
		if _, err := g.mPort.Write(buf); nil != err {
			return "", err
		}
	}

	if "" == echo {
		time.Sleep(timeout)
		return "", nil
	}

	r, err := g.waitForReply(echo, timeout)
	if err != nil {
		fmt.Printf("GSMAT<- error:%s\n", err.Error())
	} else {
		fmt.Printf("GSMAT<- \"%s\"\n", r)
	}

	return r, err
}

func (g *Gsm) Ping() error {
	g.mMutex.Lock()
	defer g.mMutex.Unlock()
	_, err := g.atcmd("AT", "OK", time.Second)
	return err
}

func (g *Gsm) RecvSMS() *sms.Message {
	select {
	case msg := <-g.mChanSMS:
		return &msg
	}
}

func (g *Gsm) RecvSMSWithTimeout(timeout *time.Duration) (*sms.Message, error) {
	select {
	case msg := <-g.mChanSMS:
		return &msg, nil
	case <-time.After(*timeout):
		return nil, errors.New("Timeout")
	}
}

func (g *Gsm) SendSMS(num, msg string) error {
	g.mMutex.Lock()
	defer g.mMutex.Unlock()
	if _, err := g.atcmd("AT", "OK", time.Second); nil != err {
		return err
	}
	s := sms.Message{
		Encoding: sms.Encodings.UCS2,
		Text:     msg,
		Address:  sms.PhoneNumber(num),
		VP:       sms.ValidityPeriod(time.Hour * 24 * 4),
		VPFormat: sms.ValidityPeriodFormats.Relative,
		Type:     sms.MessageTypes.Submit,
	}
	n, octets, err := s.PDU()

	if err != nil {
		return err
	}

	cmd := fmt.Sprintf("AT+CMGS=%d", n)
	if _, err := g.atcmd(cmd, "", time.Millisecond*300); nil != err {
		return err
	}

	cmd = hex.EncodeToString(octets)
	buf := make([]byte, len(cmd)+1)
	copy(buf, cmd)
	buf[len(cmd)] = 0x1A
	if _, err := g.mPort.Write(buf); nil != err {
		return err
	}
	_, err = g.atcmd("", "OK", time.Second*10)
	return err
}
