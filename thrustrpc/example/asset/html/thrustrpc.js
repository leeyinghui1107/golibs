var thrustPendingCalls = {};
var thrustHandlers = {};
var thrustCallSeq = 0;

function thisMsgHandler(event) {
  var msg = JSON.parse(event.payload);
  console.log(msg);
  var dir = msg['dir'];
  var seq = msg['seq'];
  if (dir == 'reply') {
    var err = msg['err']
    var _reply = msg['data'];
    var call = thrustPendingCalls[seq];
    if (call == undefined) {
      return;
    }
    clearTimeout(call['timer']);
    if (call['cbk'] != undefined) {
	  var reply = JSON.parse(_reply);
      call['cbk'](reply, err);
    }
    return
  }

  if (dir == "call") {
    var ret = {
        dir: "reply",
        seq: seq,
    };

    var method = msg['method'];
    var fn = thrustPendingCalls[method]
    if (fn == undefined){
        ret['err'] = "Unknown method"
    } else {
        ret['data'] = JSON.stringify(fn(msg['data']))
    }
    THRUST.remote.send(JSON.stringify(ret));
  }
}

function thrustRpcRegister(method, fn) {
    thrustHandlers[method] = fn;
}

function thrustRpcTimeout(seq) {
  var call = thrustPendingCalls(seq);
  if (call != undefined) {
    thrustPendingCalls.delete(seq);
    if (call['cbk'] != undefined) {
        call['cbk'].cbk(undefined, "timeout");
    }
  }
}

function thrustRpcCall(method, arg, cbk, timeout) {
    var call = {
        cbk:cbk,
    }
    var msg = {
        dir: "call",
        seq: thrustCallSeq,
        data: JSON.stringify(arg),
        method: method,
    };
    THRUST.remote.send(JSON.stringify(msg));
    var timer = setTimeout(thrustRpcTimeout, timeout, thrustCallSeq);
    call['timer'] = timer;
    thrustPendingCalls[thrustCallSeq] = call;
    thrustCallSeq++;
}

function thrustRpcInit() {
    THRUST.remote.listen(thisMsgHandler);
}