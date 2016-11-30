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
    var reply = msg['data'];
    var cb = thrustPendingCalls[seq];
    if (cb != undefined) {
        cb(reply, err);
    }
  } else if (dir == "call") {
    var ret = {
        dir: "reply",
        seq: seq,
    };

    var method = msg['method'];
    var fn = thrustPendingCalls[method]
    if (fn == undefined){
        ret['err'] = "Unknown method"
    } else {
        ret['data'] = fn(msg['data'])
    }
    THRUST.remote.send(JSON.stringify(ret));
  }
}

function thrustRpcRegister(method, fn) {
    thrustHandlers[method] = fn;
}

function thrustRpcCall(method, arg, timeout, cbk) {
    thrustPendingCalls[thrustCallSeq] = cbk;
    var msg = {
        dir: "call",
        seq: thrustCallSeq,
        data: arg,
        method: method,
    };
    THRUST.remote.send(JSON.stringify(msg))
    setTimeout(function(seq){
        var cbk = thrustPendingCalls(seq);
        if (cbk != undefined) {
            thrustPendingCalls.delete(seq);
            cbk(undefined, "timeout");
        }
    }(thrustCallSeq))
    thrustCallSeq++;
}

function thrustRpcInit() {
    THRUST.remote.listen(thisMsgHandler);
}