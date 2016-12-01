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
        var call = thrustPendingCalls[seq];
        if (call != undefined) {
            clearTimeout(call['timer']);
            if (call['cbk'] != undefined) {
                var reply = undefined;
                if (err == undefined) {
                    reply = JSON.parse(msg['data']);
                }
                call['cbk'](reply, err);
            }
        }
        return
    }

    if (dir == "call") {
        var ret = {dir: "reply", seq: seq};
        var fn = thrustHandlers[msg['method']]
        if (fn == undefined){
            ret['err'] = "Unknown method"
        } else {
            var dat = msg['data'];
            var reply = fn(dat);

            console.log("dat=" + dat)
            console.log("reply=" + reply)
            ret['data'] = JSON.stringify(reply)
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
    var call = { cbk:cbk }

    THRUST.remote.send(JSON.stringify({
        dir: "call",
        seq: thrustCallSeq,
        data: JSON.stringify(arg),
        method: method}));
    call['timer'] = setTimeout(thrustRpcTimeout, timeout, thrustCallSeq);
    thrustPendingCalls[thrustCallSeq] = call;
    thrustCallSeq++;
}

function thrustRpcInit() {
    THRUST.remote.listen(thisMsgHandler);
}
