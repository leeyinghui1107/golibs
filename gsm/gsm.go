package gsm

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	l4g "github.com/alecthomas/log4go"
	"github.com/xiqingping/golibs/serial"
	"github.com/xlab/at/sms"
)

type GsmUnsHandler interface {
	OnNewMessage(msg string)
}

type Gsm struct {
	mLogger      *l4g.Logger
	mPort        *serial.SerialPort
	mMutex       sync.Mutex
	mExit        bool
	mChanAtReply chan string
	mChanSMS     chan sms.Message
	mRecvCMT     bool
}

func NewGsm(name string, baud int, logger *l4g.Logger) (*Gsm, error) {
	s, err := serial.NewSerialPort(name, baud)
	if nil != err {
		return nil, err
	}

	gsm := Gsm{
		mLogger:      logger,
		mPort:        s,
		mExit:        false,
		mChanAtReply: make(chan string),
		mChanSMS:     make(chan sms.Message),
	}
	go gsm.recvThread()

	return &gsm, nil
}

type checkReply func(string) bool

func (g *Gsm) waitForCreg() error {
	return g.waitForCommand("AT+CREG?",
		`\+CREG: 0,[0-5]`,
		func(reply string) bool {
			return "+CREG: 0,1" == reply || "+CREG: 0,5" == reply

		},
		8,
		time.Second*5)
}

func (g *Gsm) waitForCgreg() error {
	return g.waitForCommand("AT+CGREG?",
		`\+CGREG: 0,[0-5]`,
		func(reply string) bool {
			return "+CGREG: 0,1" == reply || "+CGREG: 0,5" == reply

		},
		8,
		time.Second*5)
}

func (g *Gsm) waitForCommand(cmd, expect string, check checkReply, times int, unitDuration time.Duration) error {
	for i := 0; i < times; i = i + 1 {
		thisEndTime := time.Duration(time.Now().UnixNano()) + unitDuration
		if reply, err := g.atcmd(cmd, expect, unitDuration); err == nil && check(reply) {
			return nil
		}

		leftTime := thisEndTime - time.Duration(time.Now().UnixNano())
		if leftTime > 0 {
			time.Sleep(leftTime)
		}
	}

	return fmt.Errorf(`Wait for command "%s" timeout`, cmd)
}

func (g *Gsm) Init() error {
	g.mMutex.Lock()
	defer g.mMutex.Unlock()

	// auto baudrate
	g.atcmd("AT", "OK", time.Second)
	g.atcmd("AT", "OK", time.Second)
	g.atcmd("ATE0", "OK", time.Second)
	g.atcmd("ATE0", "OK", time.Second)

	if _, err := g.atcmd("ATE0", "OK", time.Second); err != nil {
		return err
	}

	if _, err := g.atcmd("AT+CNMI=2,2,0,0,0", "OK", time.Second*2); err != nil {
		return err
	}

	if _, err := g.atcmd("AT+CMGF=0", "OK", time.Second); err != nil {
		return err
	}

	if reply, err := g.atcmd("AT+CMGD=1,4", "(OK|ERROR)", time.Second*10); err != nil || reply == "ERROR" {
		g.mLogger.Warn("GSMAT: delete sms maybe error.")
	}

	if _, err := g.atcmd("AT+CREG=0", "OK", time.Second); err != nil {
		return err
	}

	if err := g.waitForCreg(); err != nil {
		return err
	}

	if err := g.waitForCgreg(); err != nil {
		return err
	}

	return nil
}

func (g *Gsm) handleSMS(s string) {
	b, err := hex.DecodeString(s)
	if nil != err {
		g.mLogger.Error(`GSMSMS: Decode sms hex string error "%v"`, err)
		return
	}

	var msg sms.Message
	_, err = msg.ReadFrom(b)
	if err != nil {
		g.mLogger.Error(`GSMSMS: Decode sms message error "%v"`, err)
		return
	}

	select {
	case g.mChanSMS <- msg:
	default:
		g.mLogger.Debug(`GSMSMS: Drop [%v]"%v"`, string(msg.Address), msg.Text)
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
				g.mLogger.Debug("GSMAT: Drop <- %v", reply)
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
			g.mLogger.Debug("GSMAT: Drop <- %v", data)
		case <-t:
			return "", fmt.Errorf("Timeout expired")
		}
	}
}

func (g *Gsm) atcmd(cmd, echo string, timeout time.Duration) (string, error) {
	if "" != cmd {
		g.mLogger.Debug(`GSMAT: -> "%s"`, cmd)
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
		g.mLogger.Debug(`GSMAT:<- error "%v"`, err)
	} else {
		g.mLogger.Debug(`GSMAT:<- "%v"`, r)
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
		Encoding: sms.Encodings.Gsm7Bit,
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

	buf := make([]byte, hex.EncodedLen(len(octets))+1)
	n = hex.Encode(buf, octets)
	buf[n] = 0x1A

	if _, err := g.mPort.Write(buf); nil != err {
		return err
	}
	_, err = g.atcmd("", "OK", time.Second*10)
	return err
}
