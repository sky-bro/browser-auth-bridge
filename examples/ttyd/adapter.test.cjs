const test=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs');
const source=fs.readFileSync(__dirname+'/../plugins/ttyd/browser/ttyd.js','utf8');
function harness(){
 const emitted=[],sockets=[],requests=[];
 class Socket {static OPEN=1;readyState=1;sent=[];constructor(url,protocols){this.url=url;this.protocols=protocols;sockets.push(this)}send(data){this.sent.push(new TextDecoder().decode(data))}close(){this.closed=true}}
 const scope={URL,TextEncoder,TextDecoder,ArrayBuffer,Uint8Array,WebSocket:Socket,fetch:async(url,options)=>{requests.push({url,options});return {ok:true,headers:{get:()=> 'application/json'},json:async()=>({token:'browser-only-secret'})}}};
 vm.runInNewContext(source,scope);const abort=new AbortController();
 const args={endpoint:{pageUrl:'https://terminal.example.test/',socketUrl:'wss://terminal.example.test/ws'},cols:80,rows:24,signal:abort.signal,emit:m=>emitted.push(m)};
 return {scope,args,abort,emitted,sockets,requests};
}
function frame(text){return new TextEncoder().encode(text).buffer}
test('ttyd handshake stays in browser and terminal protocol supports resize and Unicode',async()=>{
 const h=harness(),controller=await h.scope.BridgePTYAdapters.ttyd.open(h.args),s=h.sockets[0];
 assert.equal(h.requests[0].options.credentials,'include');assert.equal(h.requests[0].options.redirect,'error');assert.equal(s.protocols[0],'tty');
 s.onopen();assert.deepEqual(JSON.parse(s.sent[0]),{AuthToken:'browser-only-secret',columns:80,rows:24});
 assert.equal(controller.send({type:'stdin',data:'early'}),false);
 s.onmessage({data:frame('2{}')});assert.equal(h.emitted[0].state,'connected');
 controller.send({type:'stdin',data:'你好\n'});controller.send({type:'resize',cols:100,rows:30});assert.equal(s.sent[1],'0你好\n');assert.equal(s.sent[2],'1{"columns":100,"rows":30}');
 const bytes=new TextEncoder().encode('你');s.onmessage({data:new Uint8Array([48,...bytes.slice(0,2)]).buffer});s.onmessage({data:new Uint8Array([48,...bytes.slice(2)]).buffer});assert.equal(h.emitted.at(-1).data,'你');
 assert.ok(!JSON.stringify(h.emitted).includes('browser-only-secret'));
 h.abort.abort();assert.equal(s.closed,true);assert.equal(controller.send({type:'stdin',data:'late'}),false);
});
test('login redirects and malformed frames fail without exposing token',async()=>{
 const h=harness();h.scope.fetch=async()=>({ok:false});await assert.rejects(h.scope.BridgePTYAdapters.ttyd.open(h.args),/login_required/);
 const h2=harness();await h2.scope.BridgePTYAdapters.ttyd.open(h2.args);h2.sockets[0].onmessage({data:'invalid'});assert.equal(h2.emitted[0].error,'invalid_terminal_frame');assert.equal(h2.sockets[0].closed,true);
});
