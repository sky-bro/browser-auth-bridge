const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const event=()=>{const handlers=[];return{addListener:f=>handlers.push(f),emit:(...args)=>handlers.forEach(f=>f(...args))};};
const settle=async()=>{for(let i=0;i<8;i++)await new Promise(r=>setImmediate(r));};
function harness(){
 const ports=[],executed=[],revoked=[],storage={};let authorized=false;
 const chrome={storage:{local:{set:async x=>Object.assign(storage,x)}},permissions:{contains:async()=>authorized,remove:async()=>{authorized=false;return true},onRemoved:event()},runtime:{id:'test',getURL:p=>'chrome-extension://test/'+p,onStartup:event(),onInstalled:event(),onMessage:event(),connectNative(host){assert.equal(host,'org.browserauthbridge.bab');const p={messages:[],onMessage:event(),onDisconnect:event(),postMessage(m){this.messages.push(m)},disconnect(){this.onDisconnect.emit()}};ports.push(p);return p}}};
 const scope={URL,chrome,module:{exports:{}},BrowserExec:{execute:async(p,origin)=>{executed.push({p,origin});return{version:1,value:{ok:true}}},revoke:async o=>revoked.push(o)}};
 vm.runInNewContext(fs.readFileSync(__dirname+'/bab-background.js','utf8'),scope);
 const send=m=>ports.at(-1).onMessage.emit(m);
 return{send,ports,executed,revoked,storage,chrome,scope,authorize:()=>{authorized=true}};
}
const site={id:'example',origin:'https://example.test',label:'Example plugin'};
const attach=(session='a'.repeat(64))=>({type:'attach',session,service:'example',operation:'browser',allowTab:true,payload:{version:1,action:'open',origin:site.origin}});
test('new sites and commands work without packaged provider changes',async()=>{const h=harness();h.send({type:'catalog',payload:[site]});await settle();h.authorize();h.send(attach());await settle();assert.equal(h.executed.length,1);assert.equal(h.executed[0].origin,site.origin);const updated={id:'another',origin:'https://another.test',label:'New plugin'};h.send({type:'attach',session:'b'.repeat(64),service:'bab',operation:'catalog',payload:[site,updated]});await settle();h.send({...attach('c'.repeat(64)),service:'another'});await settle();assert.equal(h.executed.at(-1).origin,updated.origin);});
test('missing grant and unknown services never execute scripts',async()=>{const h=harness();h.send({type:'catalog',payload:[site]});await settle();h.send(attach());await settle();assert.equal(h.executed.length,0);h.authorize();h.send({...attach('b'.repeat(64)),service:'missing'});await settle();assert.equal(h.executed.length,0);assert.ok(h.ports[0].messages.some(m=>m.type==='error'));});
test('removed plugin revokes leases and stops execution',async()=>{const h=harness();h.send({type:'catalog',payload:[site]});await settle();h.authorize();h.send({type:'attach',session:'b'.repeat(64),service:'bab',operation:'catalog',payload:[]});await settle();assert.ok(h.revoked.includes(site.origin));h.send(attach());await settle();assert.equal(h.executed.length,0);});
test('untrusted pages cannot reconnect host or request site catalog',async()=>{const h=harness();let replies=0;h.chrome.runtime.onMessage.emit({type:'bab:reconnect'},{id:'test',url:'https://example.test/'},()=>replies++);assert.equal(h.ports.length,1);assert.equal(replies,0);h.chrome.runtime.onMessage.emit({type:'bab:sites'},{id:'test',url:'chrome-extension://test/bab.html'},()=>replies++);await settle();assert.equal(replies,1);});
test('catalog rejects wildcard, remote HTTP, duplicate IDs and unknown fields',()=>{const h=harness(),valid=h.scope.module.exports.validCatalog;assert.equal(valid([site]),true);for(const value of [[{...site,origin:'http://example.test'}],[{...site,origin:'https://*.test'}],[site,site],[{...site,nativeHost:'attacker.host'}]])assert.equal(valid(value),false);});

test('one registered CLI can expose multiple explicit origins',async()=>{
 const h=harness();const multiple={...site,origins:['https://catalog.example.test'],available:false};
 h.send({type:'catalog',payload:[multiple]});await settle();
 let info;h.chrome.runtime.onMessage.emit({type:'bab:sites'},{id:'test',url:'chrome-extension://test/bab.html'},v=>info=v);await settle();
 assert.equal(info.catalog.length,1);assert.equal(info.sites[0].scopes.length,2);assert.equal(info.sites[0].available,false);
});

test('registered user origin configures only a packaged editable provider',async()=>{
 const h=harness(),saved=[],removed=[];
 h.scope.BridgeProviders=[{id:'example',editableOrigin:true},{id:'fixed',editableOrigin:false}];
 h.scope.BridgeSiteConfig={read:async()=>({}),save:async(id,origin)=>saved.push({id,origin}),remove:async id=>removed.push(id)};
 const configured={...site,origin:'https://user-terminal.example.test:9443'};
 h.send({type:'catalog',payload:[configured]});await settle();
 assert.equal(saved[0].id,'example');assert.equal(saved[0].origin,configured.origin);
 h.send({type:'catalog',payload:[]});await settle();assert.equal(removed[0],'example');
});


test('disabled plugins keep authorization but cannot execute and revoke only their own leases',async()=>{
 const h=harness(),revocations=[];
 h.scope.BrowserExec.revoke=async(origin,owner)=>revocations.push({origin,owner});
 h.send({type:'catalog',payload:[site]});await settle();h.authorize();
 h.send({type:'catalog',payload:[{...site,disabled:true}]});await settle();h.send(attach());await settle();
 assert.equal(h.executed.length,0);assert.ok(revocations.some(r=>r.owner==='bab:example'));
 let info;h.chrome.runtime.onMessage.emit({type:'bab:sites'},{id:'test',url:'chrome-extension://test/bab.html'},v=>info=v);await settle();
 assert.equal(info.sites[0].enabled,false);assert.equal(info.sites[0].authorized,true);
 h.send({type:'catalog',payload:[site]});await settle();h.send(attach('c'.repeat(64)));await settle();assert.equal(h.executed.length,1);
});


test('invalid replacement catalog fails closed and clears managed provider activation',async()=>{
 const h=harness(),states=[];h.scope.BrowserAuthBridge={setCatalog:async sites=>states.push(sites)};
 h.send({type:'catalog',payload:[site]});await settle();h.authorize();
 h.send({type:'catalog',payload:[{...site,disabled:'invalid'}]});await settle();
 assert.equal(states.at(-1).length,0);h.send(attach());await settle();assert.equal(h.executed.length,0);
});
