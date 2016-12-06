THRUST.rpc = {
    pendingCalls : {},
    handlers : {},
    sequence : 0,

    msgHandler : function(event) {
    var msg = JSON.parse(event.payload);
        console.log(msg);
        var dir = msg['dir'];
        var seq = msg['seq'];

        if (dir == 'reply') {
            var err = msg['err']
            var call = THRUST.rpc.pendingCalls[seq];
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
            var fn = THRUST.rpc.handlers[msg['method']]
            if (fn == undefined){
                ret['err'] = "Unknown method"
            } else {
                var reply = fn(JSON.parse(msg['data']));
                ret['data'] = JSON.stringify(reply)
            }
            THRUST.remote.send(JSON.stringify(ret));
        }
    },

    registerMethod : function (method, fn) {
        THRUST.rpc.handlers[method] = fn;
    },

    onRpcTimeout : function(seq) {
        var call = THRUST.rpc.pendingCalls[seq];
        if (call != undefined) {
            delete THRUST.rpc.pendingCalls[seq];
            if (call['cbk'] != undefined) {
                call['cbk'].cbk(undefined, "timeout");
            }
        }
    },

    call : function (method, arg, cbk, timeout) {
        var call = { cbk:cbk }
        THRUST.remote.send(JSON.stringify({
            dir: "call",
            seq: THRUST.rpc.sequence,
            data: JSON.stringify(arg),
            method: method}));
        THRUST.rpc.call['timer'] = setTimeout(THRUST.rpc.onRpcTimeout, timeout, THRUST.rpc.sequence);
        THRUST.rpc.pendingCalls[THRUST.rpc.sequence] = call;
        THRUST.rpc.sequence++;
    },

    init : function () {
        THRUST.remote.listen(THRUST.rpc.msgHandler);
    }
};