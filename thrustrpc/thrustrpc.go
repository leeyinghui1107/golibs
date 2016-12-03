package thrustrpc

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/alexcesaro/log"

	thrwin "github.com/miketheprogrammer/go-thrust/lib/bindings/window"
	thrcmd "github.com/miketheprogrammer/go-thrust/lib/commands"
)

var (
	TimeoutErr = errors.New("Timeout")
)

type call struct {
	seq       uint32
	dc        chan interface{} // Strobes when call is complete.
	replyType reflect.Type
}

func (c *call) done(v interface{}) {
	c.dc <- v
}

func (c *call) handleReply(dat []byte) {
	if dat == nil {
		c.dc <- nil
	}

	var replyv reflect.Value
	replyIsValue := false
	if c.replyType.Kind() == reflect.Ptr {
		replyv = reflect.New(c.replyType.Elem())
	} else {
		replyv = reflect.New(c.replyType)
		replyIsValue = true
	}

	if err := json.Unmarshal([]byte(dat), replyv.Interface()); err != nil {
		c.dc <- err
		return
	}

	if replyIsValue {
		replyv = replyv.Elem()
	}
	c.dc <- replyv
}

type handler struct {
	fn        reflect.Value
	hasArg    bool
	hasReturn bool
	argType   reflect.Type
}

func (h *handler) handleCall(dat []byte) ([]byte, error) {
	var args []reflect.Value
	if dat != nil {
		if !h.hasArg {
			return nil, errors.New("argument error, the method recept no args")
		}

		var argv reflect.Value = reflect.ValueOf(interface{}(nil))
		argIsValue := false
		if h.argType.Kind() == reflect.Ptr {
			argv = reflect.New(h.argType.Elem())
		} else {
			argv = reflect.New(h.argType)
			argIsValue = true
		}
		if err := json.Unmarshal([]byte(dat), argv.Interface()); err != nil {
			return nil, err
		}
		if argIsValue {
			argv = argv.Elem()
		}
		args = []reflect.Value{argv}
	} else {
		if h.hasArg {
			return nil, errors.New("argument error, the method recept one arg")
		}
		args = []reflect.Value{}
	}

	returnValues := h.fn.Call(args)
	var err interface{}
	if h.hasReturn {
		err = returnValues[1].Interface()
	} else {
		err = returnValues[0].Interface()
	}
	if err != nil {
		return nil, err.(error)
	}

	if h.hasReturn {
		reply := returnValues[0].Interface()
		return json.Marshal(reply)
	}
	return nil, nil
}

type Rpc struct {
	seq      uint32
	mutex    sync.Mutex // protects pending, seq, request
	pending  map[uint32]*call
	handlers map[string]*handler
	win      *thrwin.Window
	logger   log.Logger
}

func NewRpc(win *thrwin.Window, logger log.Logger) (*Rpc, error) {
	rpc := &Rpc{
		win:      win,
		pending:  make(map[uint32]*call),
		handlers: make(map[string]*handler),
		logger:   logger,
	}

	_, err := win.HandleRemote(rpc.Handle)
	return rpc, err
}

func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return isExported(t.Name()) || t.PkgPath() == ""
}

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

func (rpc *Rpc) Register(mname string, handlerFunc interface{}) {
	if _, ok := rpc.handlers[mname]; ok {
		panic("rpc2: multiple registrations for " + mname)
	}

	argType := reflect.TypeOf(interface{}(nil))
	method := reflect.ValueOf(handlerFunc)
	mtype := method.Type()

	h := &handler{fn: method}

	if mtype.NumIn() == 1 {
		argType = mtype.In(0)
		if !isExportedOrBuiltinType(argType) {
			rpc.logger.Error(mname, "argument type not exported:", argType)
			return
		}
		h.hasArg = true
		h.argType = argType
	} else if mtype.NumIn() != 0 {
		rpc.logger.Error("method ", mname, " has wrong number of ins:", mtype.NumIn())
		return
	}

	// Method needs one out.
	if mtype.NumOut() == 1 || mtype.NumOut() == 2 {
		// The last return type of the method must be error.
		if returnType := mtype.Out(mtype.NumOut() - 1); returnType != typeOfError {
			rpc.logger.Error("method", mname, "returns", returnType.String(), "not error")
		}
	} else {
		rpc.logger.Error("method", mname, "has wrong number of outs:", mtype.NumOut())
		return
	}

	if mtype.NumOut() == 2 {
		h.hasReturn = true
	}
	rpc.handlers[mname] = h
}

func (rpc *Rpc) Call(method string, arg interface{}, timeout time.Duration) (interface{}, error) {
	data, err := json.Marshal(arg)
	if err != nil {
		return nil, err
	}

	var c call
	rpc.mutex.Lock()
	c.seq = rpc.seq
	rpc.seq++
	rpc.pending[c.seq] = &c
	rpc.mutex.Unlock()

	msgObj := map[string]interface{}{
		"dir":    "call",
		"seq":    c.seq,
		"data":   string(data),
		"method": method,
	}

	msg, err := json.Marshal(msgObj)
	if err != nil {
		rpc.mutex.Lock()
		delete(rpc.pending, c.seq)
		rpc.mutex.Unlock()
		return nil, err
	}

	rpc.logger.Debug("GO->JS:", string(msg))
	rpc.win.SendRemoteMessage(string(msg))

	select {
	case reply := <-c.dc:
		if reply == nil {
			return nil, nil
		}

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

func (rpc *Rpc) handleCall(seq uint32, method string, arg []byte) {
	ret := map[string]interface{}{}
	ret["dir"] = "reply"
	ret["seq"] = seq

	defer func() {
		msg, err := json.Marshal(ret)
		if err != nil {
			rpc.logger.Error("Can not marshal json")
			return
		}
		rpc.logger.Debug("GO->JS:", string(msg))
		rpc.win.SendRemoteMessage(string(msg))
	}()

	h, ok := rpc.handlers[method]
	if !ok {
		ret["err"] = "Unsupported method"
		return
	}

	if reply, err := h.handleCall(arg); err != nil {
		ret["err"] = err.Error()
	} else if reply != nil {
		ret["data"] = string(reply)
	}
}

func (rpc *Rpc) Handle(er thrcmd.EventResult, this *thrwin.Window) {
	if this != rpc.win {
		return
	}

	rpc.logger.Debug("JS->GO:", er.Message.Payload)
	drop := true
	var f map[string]interface{}
	var what string

	defer func() {
		if drop {
			rpc.logger.Warning("Drop:", what)
		}
	}()

	if err := json.Unmarshal([]byte(er.Message.Payload), &f); err != nil {
		what = `Unmarshal json string error` + err.Error()
		return
	}

	_dir, ok := f["dir"]
	if !ok {
		what = `no "dir" section`
		return
	}
	dir, ok := _dir.(string)
	if !ok {
		what = `"dir" section is not a string`
		return
	}

	_seq, ok := f["seq"]
	if !ok {
		what = `no "seq" section`
		return
	}

	fseq, ok := _seq.(float64)
	if !ok {
		what = `"seq" section is not a number`
		return
	}

	seq := uint32(fseq)

	if dir == "call" {

		_method, ok := f["method"]
		if !ok {
			what = `no "method" section`
			return
		}

		method, ok := _method.(string)
		if !ok {
			what = `"method" section is not a string`
			return
		}
		_data, ok := f["data"]
		if !ok {
			go rpc.handleCall(seq, method, nil)
			drop = false
			return
		}

		data, ok := _data.(string)
		if !ok {
			what = `"data" section is not a string`
			return
		}

		go rpc.handleCall(seq, method, []byte(data))
		drop = false
		return
	}

	if dir == "reply" {
		rpc.mutex.Lock()
		call, ok := rpc.pending[seq]
		delete(rpc.pending, seq)
		rpc.mutex.Unlock()
		if !ok {
			what = `no pending call for seq ` + strconv.FormatUint(uint64(seq), 10)
			return
		}
		_errstr, ok := f["err"]
		if ok {
			errstr, ok := _errstr.(string)
			if !ok {
				what = `"err" section is not a string`
				return
			}

			call.done(errors.New(errstr))
			drop = false
			return
		}

		_data, ok := f["data"]
		if !ok {
			go call.handleReply(nil)
			drop = false
			return
		}
		data, ok := _data.(string)
		if !ok {
			what = `"data" section is not a string`
			return
		}
		go call.handleReply([]byte(data))
		drop = false
	}
}
