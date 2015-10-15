/*
实现了GSM模块手法短信的功能.
*/
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

// GSM结构体
type Gsm struct {
	mLogger      *l4g.Logger
	mPort        *serial.SerialPort
	mMutex       sync.Mutex
	mExit        bool
	mChanAtReply chan string
	mChanSMS     chan sms.Message
	mRecvCMT     bool
}

// 构建一个新的GSM结构体.
// name 与GSM模块连接的串口设备.
// baud 串口使用的波特率
// logger 日志
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

// 等待AT命令应答.
// expect 合法字符串应答.
// timeout 等待应答超时
// return 等到的应答, 错误
func (g *Gsm) waitForReply(expect string, timeout time.Duration) (string, error) {
	regExpPatttern := regexp.MustCompile(expect)
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

// 发送AT命令并等待应答.
// expect 合法字符串应答.
// timeout 等待应答超时
// return 等到的应答, 错误
func (g *Gsm) atcmd(cmd, expect string, timeout time.Duration) (string, error) {
	if "" != cmd {
		g.mLogger.Debug(`GSMAT: -> "%s"`, cmd)
		buf := make([]byte, len(cmd)+1)
		copy(buf, cmd)
		buf[len(cmd)] = '\r'
		if _, err := g.mPort.Write(buf); nil != err {
			return "", err
		}
	}

	if "" == expect {
		time.Sleep(timeout)
		return "", nil
	}

	r, err := g.waitForReply(expect, timeout)
	if err != nil {
		g.mLogger.Debug(`GSMAT:<- error "%v"`, err)
	} else {
		g.mLogger.Debug(`GSMAT:<- "%v"`, r)
	}

	return r, err
}

// 判断应答字符串是否为期望的字符串的函数类型
type checkReply func(string) bool

// 多次重试AT指令, 并等待期望的应答.
// cmd AT命令.
// expect 合法应答字符串的正则表达式.
// check 检查应答是否正常的函数.
// times AT命令重试的次数.
// unitDuration 一次AT指令等待的时间.
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

// 等待GSM网络注册(AT命令中的CREG)
func (g *Gsm) waitForCreg() error {
	return g.waitForCommand(
		"AT+CREG?",
		`\+CREG: 0,[0-5]`,
		func(reply string) bool { return "+CREG: 0,1" == reply || "+CREG: 0,5" == reply },
		8,
		time.Second*5)
}

// 等待GPRS网络注册(AT命令中的CGREG)
func (g *Gsm) waitForCgreg() error {
	return g.waitForCommand(
		"AT+CGREG?",
		`\+CGREG: 0,[0-5]`,
		func(reply string) bool { return "+CGREG: 0,1" == reply || "+CGREG: 0,5" == reply },
		8,
		time.Second*5)
}

// 初始化
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

// 处理短信PUD字符串.
// s 串口接收到的PDU字符串.
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

// 串口接收线程
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

// 关闭GSM模块
func (g *Gsm) Teardown() error {
	g.mMutex.Lock()
	defer g.mMutex.Unlock()

	g.mExit = true
	close(g.mChanAtReply)
	close(g.mChanSMS)
	time.Sleep(time.Millisecond * 20)
	return g.mPort.Close()
}

// 检测GSM AT命令的通道是否正常.
// return 错误; ==nil AT命令通道正常.
func (g *Gsm) Ping() error {
	g.mMutex.Lock()
	defer g.mMutex.Unlock()
	_, err := g.atcmd("AT", "OK", time.Millisecond*250)
	return err
}

// 接收短信, 这个函数会阻塞直至接收到短信.
// return 接收到的短信.
func (g *Gsm) RecvSMS() *sms.Message {
	select {
	case msg := <-g.mChanSMS:
		return &msg
	}
}

// 在指定超时时间内接收短信, 这个函数会阻塞直至接收到短信或超时.
// timeout 超时时间.
// return 接收到的短信, 错误.
func (g *Gsm) RecvSMSWithTimeout(timeout *time.Duration) (*sms.Message, error) {
	select {
	case msg := <-g.mChanSMS:
		return &msg, nil
	case <-time.After(*timeout):
		return nil, errors.New("Timeout")
	}
}

// 发送短信.
// num 接收者的号码.
// msg 需要发送的短信内容.
// return 错误; ==nil 发送正常.
func (g *Gsm) SendSMS(num, msg string) error {
	g.mMutex.Lock()
	defer g.mMutex.Unlock()
	if _, err := g.atcmd("AT", "OK", time.Millisecond*250); nil != err {
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

	if len(msg) != len([]rune(msg)) {
		s.Encoding = sms.Encodings.UCS2
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
