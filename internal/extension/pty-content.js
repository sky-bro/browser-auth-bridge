(() => {
 "use strict";
 // Injection can occur once per session; install one listener per page context.
 if(globalThis.PTYCoreInstalled)return;
 globalThis.PTYCoreInstalled=true;
 chrome.runtime.onConnect.addListener(port=>{
  if(port.name!=="pty-v1")return;
  let socket, recipe, controller, closed=false, opened=false;
  const abort = new AbortController();
  const emit=m=>{try{port.postMessage(m);}catch(_){stop();}};
  function stop(){if(closed)return;closed=true;abort.abort();controller?.close();socket?.close();}
  port.onDisconnect.addListener(stop);
  port.onMessage.addListener(async m=>{
   if(closed)return;
   if(m.type==="close"){stop();return;}
   if(m.type!=="open") {
    if(controller)controller.send(m);
    else if(opened)StreamTransport.send(socket,recipe,m);
    return;
   }
   if(opened)return;opened=true;
   recipe=m.payload?.recipe;
   try {
    if(m.payload?.endpoint) {
     const endpoint=m.payload.endpoint;
     if(!PTYTransport.validateEndpoint(endpoint,location.origin,m.adapter))throw new Error('pty_scope_denied');
     const adapter=globalThis.BridgePTYAdapters?.[endpoint.adapter];
     if(!adapter)throw new Error('adapter_not_installed');
     controller=await adapter.open({endpoint,cols:m.payload.cols,rows:m.payload.rows,emit:m=>{if(!closed)emit(m);},signal:abort.signal});
     if(closed)controller.close();
     return;
    }
    if(!StreamTransport.validate(recipe,location.origin))throw new Error("pty_scope_denied");
    await StreamTransport.authorize(recipe);
    if(closed)return;
    socket=new WebSocket(recipe.socketUrl);
    socket.onopen=()=>{
     StreamTransport.send(socket,recipe,{type:"resize",cols:m.payload.cols,rows:m.payload.rows});
     emit({type:"status",state:"connected"});
    };
    socket.onmessage=e=>{try{const out=StreamTransport.decode(recipe,e.data);if(out)emit(out);}catch(_){emit({type:"error",error:"invalid_terminal_frame"});stop();}};
    socket.onerror=()=>{emit({type:"error",error:"terminal_connection_failed"});stop();};
    socket.onclose=()=>{emit({type:"status",state:"closed"});stop();};
   } catch (_) {emit({type:"error",error:"terminal_authorization_failed"});stop();}
  });
 });
})();
