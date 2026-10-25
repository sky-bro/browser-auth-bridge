// Consumer-owned ttyd 1.7.x protocol. Tokens stay inside this isolated browser
// context; only terminal output/status crosses the bridge.
globalThis.BridgePTYAdapters ??= Object.create(null);
globalThis.BridgePTYAdapters.ttyd = Object.freeze({
  async open({endpoint, cols, rows, emit, signal}) {
    const tokenURL = new URL('token', endpoint.pageUrl);
    const response = await fetch(tokenURL.href, {credentials:'include', redirect:'error', signal});
    if (!response.ok || !response.headers.get('content-type')?.includes('json')) throw new Error('login_required');
    const body = await response.json();
    if (typeof body.token !== 'string' || body.token.length > 16384) throw new Error('invalid_token_response');
    if (signal.aborted) throw new Error('canceled');
    const socket = new WebSocket(endpoint.socketUrl, ['tty']);
    socket.binaryType = 'arraybuffer';
    const encoder = new TextEncoder(), decoder = new TextDecoder();
    let connected=false, stopped=false;
    const close=()=>{if(stopped)return;stopped=true;signal.removeEventListener('abort',close);socket.close();};
    signal.addEventListener('abort',close,{once:true});
    socket.onopen=()=>{if(!stopped)socket.send(encoder.encode(JSON.stringify({AuthToken:body.token,columns:cols,rows})));};
    socket.onmessage=event=>{
      if(stopped)return;
      try {
        if(!(event.data instanceof ArrayBuffer) || event.data.byteLength>1048576 || !event.data.byteLength)throw new Error('invalid_frame');
        const bytes=new Uint8Array(event.data), command=bytes[0];
        if(![48,49,50].includes(command))throw new Error('invalid_frame');
        // ttyd sends preferences after authenticating the initial handshake.
        if(!connected && (command===50 || command===48)){connected=true;emit({type:'status',state:'connected'});}
        if(command===48){const data=decoder.decode(bytes.subarray(1),{stream:true});if(data)emit({type:'stdout',data});}
      } catch (_) {emit({type:'error',error:'invalid_terminal_frame'});close();}
    };
    socket.onerror=()=>{if(!stopped)emit({type:'error',error:'terminal_connection_failed'});close();};
    socket.onclose=()=>{if(!stopped)emit({type:'status',state:'closed'});close();};
    return {
      close,
      send(message) {
        if(stopped || !connected || socket.readyState!==WebSocket.OPEN)return false;
        if(message.type==='stdin' && typeof message.data==='string' && message.data.length<=262144)socket.send(encoder.encode('0'+message.data));
        else if(message.type==='resize' && Number.isInteger(message.cols) && message.cols>0 && message.cols<=10000 && Number.isInteger(message.rows) && message.rows>0 && message.rows<=10000)socket.send(encoder.encode('1'+JSON.stringify({columns:message.cols,rows:message.rows})));
        else return false;
        return true;
      },
    };
  },
});
