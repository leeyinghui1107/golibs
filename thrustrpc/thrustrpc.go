package thrustrpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	thrwin "github.com/miketheprogrammer/go-thrust/lib/bindings/window"
	thrcmd "github.com/miketheprogrammer/go-thrust/lib/commands"
)

var (
	TimeoutErr = errors.New("Timeout")
)

type call struct {
	seq uint32
	dc  chan interface{} // Strobes when call is complete.
}

func (c *call) done(reply interface{}) {
	c.dc <- reply
}

type Rpc struct {
	seq      uint32
	mutex    sync.Mutex // protects pending, seq, request
	pending  map[uint32]*call
	handlers map[string]func(arg interface{}) (interface{}, error)
	win      *thrwin.Window
}

func NewRpc(win *thrwin.Window) (*Rpc, error) {
	rpc := &Rpc{
		win:      win,
		pending:  make(map[uint32]*call),
		handlers: make(map[string]func(arg interface{}) (interface{}, error)),
	}

	_, err := win.HandleRemote(rpc.Handle)
	return rpc, err
}

func (rpc *Rpc) Register(method string, fn func(arg interface{}) (interface{}, error)) {
	rpc.handlers[method] = fn
}

func (rpc *Rpc) Call(method string, arg interface{}, timeout time.Duration) (interface{}, error) {
	var c call
	rpc.mutex.Lock()
	c.seq = rpc.seq
	rpc.seq++
	rpc.pending[c.seq] = &c
	rpc.mutex.Unlock()

	msg := map[string]interface{}{
		"dir":    "call",
		"seq":    c.seq,
		"dat":    arg,
		"method": method,
	}

	dat, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	rpc.win.SendRemoteMessage(string(dat))

	select {
	case reply := <-c.dc:
		if err, ok := reply.(error); ok {
			return nil, err
		}
		return reply, nil
	case <-time.After(timeout):
		rpc.mutex.Lock()
		delete(rpc.pending, c.seq)
		rpc.mutex.Unlock()
		return nil, TimeoutErr
	}

}

func (rpc *Rpc) handleCall(seq uint32, method string, arg interface{}) {

	ret := map[string]interface{}{}
	ret["dir"] = "reply"
	ret["seq"] = seq

	defer func() {
		msg, err := json.Marshal(ret)
		if err != nil {
			fmt.Println("Can not marshal json")
			return
		}
		rpc.win.SendRemoteMessage(string(msg))
	}()

	handle, ok := rpc.handlers[method]
	if !ok {
		ret["err"] = "Unsupported method"
		return
	}

	reply, err := handle(arg)
	if err != nil {
		ret["err"] = err.Error()
		return
	}

	ret["data"] = reply

}

func (rpc *Rpc) Handle(er thrcmd.EventResult, this *thrwin.Window) {
	if this != rpc.win {
		return
	}

	fmt.Println("JS->GO:", er.Message.Payload)
	drop := true
	var f map[string]interface{}

	defer func() {
		if drop {
			fmt.Println("Drop, unformated message.")
		}
	}()

	if err := json.Unmarshal([]byte(er.Message.Payload), &f); err != nil {
		return
	}

	_dir, ok := f["dir"]
	if !ok {
		fmt.Println("dir format 1")
		return
	}
	dir, ok := _dir.(string)
	if !ok {
		fmt.Println("dir format 2")
		return
	}

	_seq, ok := f["seq"]
	if !ok {
		fmt.Println("seq format 1")
		return
	}
	fmt.Println(reflect.TypeOf(_seq).String())
	seq, ok := _seq.(uint32)
	if !ok {
		fmt.Println("seq format 2")
		return
	}

	if dir == "call" {
		args, ok := f["data"]
		if !ok {
			fmt.Println("dat format 1")
			return
		}
		_method, ok := f["method"]
		if !ok {
			fmt.Println("method format 1")
			return
		}

		method, ok := _method.(string)
		if !ok {
			fmt.Println("method format 2")
			return
		}
		go rpc.handleCall(seq, method, args)
		drop = false
		return
	}

	if dir == "reply" {
		rpc.mutex.Lock()
		call, ok := rpc.pending[seq]
		delete(rpc.pending, seq)
		rpc.mutex.Unlock()
		if !ok {
			fmt.Println("No this seq", seq)
			return
		}
		_errstr, ok := f["err"]
		if ok {
			errstr, ok := _errstr.(string)
			if !ok {
				return
			}

			call.done(errors.New(errstr))
			drop = false
			return
		}

		__reply, ok := f["data"]
		if !ok {
			return
		}
		_reply, ok := __reply.(string)
		if !ok {
			return
		}

		var reply interface{}

		err := json.Unmarshal([]byte(_reply), &reply)
		if err != nil {
			return
		}

		call.done(reply)
	}
}
