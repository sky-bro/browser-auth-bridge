const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync(__dirname+'/background.js','utf8');
const recipe=JSON.parse(fs.readFileSync(__dirname+'/../../testdata/pty-recipe.json','utf8'));
function event(){const handlers=[];return {addListener:f=>handlers.push(f),emit:(...args)=>handlers.forEach(f=>f(...args))};}
function port(){return {messages:[],onMessage:event(),onDisconnect:event(),postMessage(m){this.messages.push(m);},disconnect(){this.onDisconnect.emit();}};}
const settle=async()=>{for(let i=0;i<8;i++)await new Promise(r=>setImmediate(r));};
async function harness(providerOptions={}){
 const native=port(),pages=[],removed=[],injected=[];
 const scope={URL,setTimeout,clearTimeout,importScripts(){},BridgeProviders:[{id:'demo',origin:recipe.origin,nativeHost:'org.browserauthbridge.demo',...providerOptions}],StreamTransport:{validate:r=>r?.origin===recipe.origin},BrowserExec:{execute:async()=>({version:1}),revoke:async()=>{}},granted:true,sites:{}};
 scope.BrowserAuthBridge={site:()=>scope.registry || null};
 const connections=[];
 scope.chrome={storage:{local:{get:async()=>({bridgeSites:scope.sites})},onChanged:event()},permissions:{contains:async()=>scope.granted,onAdded:event(),onRemoved:event()},runtime:{id:'test-extension',onMessage:event(),connectNative:()=>{const p=connections.length?port():native;connections.push(p);return p;},onStartup:event(),onInstalled:event()},scripting:{executeScript:async args=>{injected.push(args.files)}},tabs:{query:async()=>scope.userTab?[scope.userTab]:[],create:async({url})=>({id:7,url}),get:async()=>({id:7,url:recipe.pageUrl,status:'complete'}),connect(){const p=port();pages.push(p);return p;},remove:async id=>removed.push(id)}};
 vm.runInNewContext(fs.readFileSync(__dirname+'/site-config.js','utf8'),scope);
 vm.runInNewContext(source,scope);await settle();
 const attach=session=>({type:'attach',session,service:'demo',operation:'pty',allowTab:true,payload:{version:1,provider:'demo',target:'test',recipe,cols:80,rows:24}});
 vm.runInNewContext(fs.readFileSync(__dirname+'/pty-transport.js','utf8'),scope);
 return {scope,native,pages,removed,attach,injected,connections};
}
test('PTY sessions isolate traffic and release their owned resources',async()=>{
 const h=await harness(),a='a'.repeat(64),b='b'.repeat(64);
 h.native.onMessage.emit(h.attach(a));await settle();h.native.onMessage.emit(h.attach(b));await settle();assert.equal(h.pages.length,2);
 h.pages[0].onMessage.emit({type:'stdout',data:'first'});assert.equal(h.native.messages.find(m=>m.type==='stdout').session,a);
 h.native.onMessage.emit({type:'stdin',session:b,data:'second'});assert.equal(h.pages[1].messages.at(-1).data,'second');
 h.native.onMessage.emit({type:'close',session:a});await settle();assert.equal(h.pages[0].messages.at(-1).type,'close');assert.equal(h.pages[1].messages.at(-1).type,'stdin');
 h.native.onMessage.emit({type:'close',session:b});await settle();assert.equal(h.removed.length,2);
});
test('manual page policy denies creation and preserves user tabs',async()=>{
 const h=await harness();let m=h.attach('a'.repeat(64));m.allowTab=false;h.native.onMessage.emit(m);await settle();assert.equal(h.pages.length,0);assert.ok(h.native.messages.some(m=>m.type==='error'));
 h.scope.userTab={id:7,url:recipe.pageUrl};m=h.attach('b'.repeat(64));h.native.onMessage.emit(m);await settle();h.native.onMessage.emit({type:'close',session:m.session});await settle();assert.equal(h.removed.length,0);
});
test('revoking origin permission closes active PTY resources',async()=>{
 const h=await harness();h.native.onMessage.emit(h.attach('a'.repeat(64)));await settle();h.scope.granted=false;h.scope.chrome.permissions.onRemoved.emit({origins:[recipe.origin+'/*']});await settle();assert.equal(h.pages[0].messages.at(-1).type,'close');assert.equal(h.removed.length,1);
});
test('provider mismatch never injects the PTY content script',async()=>{
 const h=await harness(),m=h.attach('a'.repeat(64));m.payload.provider='other';h.native.onMessage.emit(m);await settle();assert.equal(h.pages.length,0);assert.ok(h.native.messages.some(m=>m.type==='error'));
});

test('consumer adapter scripts come only from provider config and mismatch is denied',async()=>{
 const h=await harness({ptyAdapter:'ttyd',ptyScripts:['adapters/ttyd.js']});
 const m=h.attach('c'.repeat(64));delete m.payload.recipe;m.payload.endpoint={adapter:'ttyd',origin:recipe.origin,pageUrl:recipe.pageUrl,socketUrl:recipe.socketUrl};
 h.native.onMessage.emit(m);await settle();assert.equal(h.pages.length,1);assert.ok(h.injected[0].includes('adapters/ttyd.js'));
 const bad=structuredClone(m);bad.session='d'.repeat(64);bad.payload.endpoint.adapter='uninstalled';h.native.onMessage.emit(bad);await settle();assert.equal(h.pages.length,1);
});

test('changing a user-configured site closes old sessions and ignores late old-host messages',async()=>{
 const h=await harness({editableOrigin:true});
 h.native.onMessage.emit(h.attach('a'.repeat(64)));await settle();
 assert.equal(h.pages.length,1);
 h.scope.sites={demo:'https://another.example.test'};
 h.scope.chrome.storage.onChanged.emit({bridgeSites:{newValue:h.scope.sites}},'local');
 await settle();
 assert.equal(h.pages[0].messages.at(-1).type,'close');
 assert.equal(h.connections.length,2);
 h.native.onMessage.emit(h.attach('b'.repeat(64)));await settle();
 assert.equal(h.pages.length,1);
});

test('invalid stored origins fail closed without connecting a native host',async()=>{
 const h=await harness({editableOrigin:true});
 h.scope.sites={demo:'http://remote.example.test'};
 h.scope.chrome.permissions.onAdded.emit({});await settle();
 h.native.onMessage.emit(h.attach('a'.repeat(64)));await settle();
 assert.equal(h.pages.length,0);
 assert.equal(h.connections.length,1);
});


test('managed providers never use stale stored origins before native registry confirmation',async()=>{
 const h=await harness({registryManaged:true});
 assert.equal(h.connections.length,0);
 h.scope.sites={demo:recipe.origin};h.scope.chrome.permissions.onAdded.emit();await settle();assert.equal(h.connections.length,0);
 h.scope.registry={id:'demo',origin:recipe.origin,available:true};await h.scope.BridgeSyncProviders();await settle();assert.equal(h.connections.length,1);
 h.native.onMessage.emit(h.attach('a'.repeat(64)));await settle();assert.equal(h.pages.length,1);
 h.scope.registry={...h.scope.registry,disabled:true};await h.scope.BridgeSyncProviders();await settle();
 assert.equal(h.pages[0].messages.at(-1).type,'close');assert.equal(h.scope.granted,true);
 h.scope.registry=null;await h.scope.BridgeSyncProviders();assert.equal(h.connections.length,1);
});
